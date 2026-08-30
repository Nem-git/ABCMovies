package m5_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/delivery"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestM5PlayEndToEndProvesRelayNotes proves one play session across the whole
// M5 stack (PLAN.md §6.2): alice starts a play against her linked account's
// server namespace, the session stages its menu and is announced as a job,
// GetPlayInfo recovers the staged menu with the relay URLs, and the player's
// pull through the relay serves the provider's bytes — as an unauthenticated
// consumer who never sees the provider's credentials. The provider is touched
// exactly once more, through the relay's own fetch.
func TestM5PlayEndToEndProvesRelayNotes(t *testing.T) {
	jf := fakeJellyfinServer(t)
	stack := newM5Stack(t, jf)
	client := apiv1.NewCoreServiceClient(startWireServer(t, stack))
	aliceCtx := authedCtx(t.Context(), stack.aliceToken)

	// Subscribe to alice's user events before the start: the bus keeps no
	// history (PLAN.md §9.2, at-most-once).
	evCh := stack.bus.Subscribe("m5-play-alice", stack.alice.UserID)
	defer stack.bus.Unsubscribe("m5-play-alice")

	play, err := client.StartDelivery(aliceCtx, &apiv1.StartDeliveryRequest{
		Goal:         apiv1.DeliveryGoal_DELIVERY_GOAL_PLAY,
		Provider:     stack.ns,
		AccountId:    "lnk_alice_home",
		MemberUserId: stack.alice.UserID,
		NativeId:     "movie-gondwana",
		Sink:         "device",
	})
	if err != nil {
		t.Fatalf("StartDelivery: %v", err)
	}
	job := play.GetJob()
	if job.GetStatus() != corev1.JobStatus_JOB_STATUS_RUNNING {
		t.Fatalf("job status = %v, want running", job.GetStatus())
	}
	if job.GetOwnerUserId() != stack.alice.UserID {
		t.Errorf("job owner = %q, want alice", job.GetOwnerUserId())
	}

	// The start announces two notifications to alice's stream, in engine
	// order: the staged play menu, then the job-status event (menu-ready is
	// announced inside Engine.Start before the handler persists the job).
	select {
	case env := <-evCh:
		if env.GetType() != corev1.EventType_EVENT_TYPE_DELIVERY_PLAY_MENU_READY {
			t.Fatalf("first event type = %v, want play-menu-ready", env.GetType())
		}
		if env.GetPlayMenuReady().GetJobId() != job.GetId() {
			t.Fatalf("menu-ready job = %q, want %q", env.GetPlayMenuReady().GetJobId(), job.GetId())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the play-menu-ready event")
	}
	select {
	case env := <-evCh:
		if env.GetType() != corev1.EventType_EVENT_TYPE_JOB_STATUS {
			t.Fatalf("second event type = %v, want job-status", env.GetType())
		}
		if env.GetJobStatus().GetJobId() != job.GetId() {
			t.Fatalf("event job = %q, want %q", env.GetJobStatus().GetJobId(), job.GetId())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the delivery job-status event")
	}

	// Recover the staged play menu: one container video track with a relay URL.
	info, err := client.GetPlayInfo(aliceCtx, &apiv1.GetPlayInfoRequest{SessionId: job.GetId()})
	if err != nil {
		t.Fatalf("GetPlayInfo: %v", err)
	}
	if len(info.GetTracks()) != 1 {
		t.Fatalf("play menu has %d tracks, want the single video container", len(info.GetTracks()))
	}
	tr := info.GetTracks()[0]
	if tr.GetVideo() == nil || tr.GetVideo().GetCodec() != "hevc" {
		t.Fatalf("play track is %v, want a hevc video track", tr)
	}
	if tr.GetRelayUrl() == "" || tr.GetTrackId() == "" {
		t.Fatalf("play track missing relay url or id: %+v", tr)
	}
	// A passthrough play resolves no container (the plan never names one); the
	// menu's container stays empty until a remux step asks for one.
	if got := info.GetContainer(); got != "" {
		t.Fatalf("container = %q, want empty for a passthrough play", got)
	}

	// The player pulls through the relay as an unauthenticated consumer; the
	// token — not the provider's credentials — authorizes the stream.
	relaySrv := httptest.NewServer(&delivery.RelayHandler{Relay: stack.relay})
	defer relaySrv.Close()
	res, err := http.Get(relaySrv.URL + tr.GetRelayUrl())
	if err != nil {
		t.Fatalf("relay pull: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read relay body: %v", err)
	}
	want := []byte("P5-gondwana-mkv-bytes")
	if string(data) != string(want) {
		t.Fatalf("relay served %q, want the provider's stream bytes %q", data, want)
	}

	jf.mu.Lock()
	hits := jf.streamHits["movie-gondwana"]
	jf.mu.Unlock()
	if hits != 1 {
		t.Fatalf("provider stream fetched %d times, want exactly once through the relay", hits)
	}

	// The session's job is queryable through the lifecycle surface too.
	got, err := client.GetJob(aliceCtx, &apiv1.GetJobRequest{JobId: job.GetId()})
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got.GetJob().GetId() != job.GetId() {
		t.Errorf("GetJob id = %q, want %q", got.GetJob().GetId(), job.GetId())
	}
}

// TestM5PlayInfoCrossUserDenied proves the ownership boundary of GetPlayInfo
// (PLAN.md §6.2): bob — even a legitimate, authenticated member — cannot
// recover alice's staged play menu.
func TestM5PlayInfoCrossUserDenied(t *testing.T) {
	stack := newM5Stack(t, fakeJellyfinServer(t))
	client := apiv1.NewCoreServiceClient(startWireServer(t, stack))
	aliceCtx := authedCtx(t.Context(), stack.aliceToken)

	play, err := client.StartDelivery(aliceCtx, &apiv1.StartDeliveryRequest{
		Goal:         apiv1.DeliveryGoal_DELIVERY_GOAL_PLAY,
		Provider:     stack.ns,
		AccountId:    "lnk_alice_home",
		MemberUserId: stack.alice.UserID,
		NativeId:     "movie-gondwana",
		Sink:         "device",
	})
	if err != nil {
		t.Fatalf("StartDelivery: %v", err)
	}

	_, err = client.GetPlayInfo(authedCtx(t.Context(), stack.bobToken), &apiv1.GetPlayInfoRequest{SessionId: play.GetJob().GetId()})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("bob GetPlayInfo on alice's session: got %v, want PermissionDenied", status.Code(err))
	}
}

// TestM5PlayInfoUnknownSessionCleanMiss proves a GetPlayInfo on an unknown
// session id is a clean NotFound (PLAN.md §6.2 recovery: an at-most-once
// subscriber that missed the ready event can query safely).
func TestM5PlayInfoUnknownSessionCleanMiss(t *testing.T) {
	stack := newM5Stack(t, fakeJellyfinServer(t))
	client := apiv1.NewCoreServiceClient(startWireServer(t, stack))
	aliceCtx := authedCtx(t.Context(), stack.aliceToken)

	_, err := client.GetPlayInfo(aliceCtx, &apiv1.GetPlayInfoRequest{SessionId: "del-no-such-session"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("GetPlayInfo on unknown session: got %v, want NotFound", status.Code(err))
	}
}
