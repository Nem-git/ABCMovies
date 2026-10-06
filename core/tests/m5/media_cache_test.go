package m5_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/delivery"
)

// TestM5MediaBytesNeverReachAStore pins the payload rule of PLAN.md §2.4:
// media and keys are never written into any persisted store — only metadata
// (catalogues, entries, job records, encrypted credentials) is. It runs one
// play end to end, pulls the provider's bytes through the relay, and then:
//   - proves payload bytes did move (the relay's response matches the fake
//     stream exactly), so the absence assertion below is not vacuous;
//   - probes every captured Store.Put for any of those payload bytes.
func TestM5MediaBytesNeverReachAStore(t *testing.T) {
	jf := fakeJellyfinServer(t)
	stack := newM5Stack(t, jf)
	client := apiv1.NewCoreServiceClient(startWireServer(t, stack))
	aliceCtx := authedCtx(t.Context(), stack.aliceToken)

	play, err := client.StartDelivery(aliceCtx, &apiv1.StartDeliveryRequest{
		Goal:         apiv1.DeliveryGoal_DELIVERY_GOAL_PLAY,
		Provider:     stack.ns,
		AccountId:    "lnk_alice_home",
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

	info, err := client.GetPlayInfo(aliceCtx, &apiv1.GetPlayInfoRequest{SessionId: job.GetId()})
	if err != nil {
		t.Fatalf("GetPlayInfo: %v", err)
	}
	if len(info.GetTracks()) != 1 || info.GetTracks()[0].GetRelayUrl() == "" {
		t.Fatalf("expected one relay track, got %v", info.GetTracks())
	}

	// The player pulls the payload through the relay: a plain HTTP GET
	// authenticated by the relay token only (PLAN.md §3.6).
	relaySrv := httptest.NewServer(&delivery.RelayHandler{Relay: stack.relay})
	defer relaySrv.Close()
	res, err := http.Get(relaySrv.URL + info.GetTracks()[0].GetRelayUrl())
	if err != nil {
		t.Fatalf("relay pull: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read relay body: %v", err)
	}
	want := jf.streamBytes["movie-gondwana"]
	if string(data) != string(want) {
		t.Fatalf("relay payload = %q, want %q", data, want)
	}

	// The fake's own stream counter must show the movie was really served —
	// otherwise the absence assertions would be vacuous.
	if hits := jf.streamHits["movie-gondwana"]; hits == 0 {
		t.Fatal("fake provider stream never hit; test proves nothing")
	}

	// The probe must have seen writes (job persistence, account credentials,
	// library derivation all hit stores), otherwise absence proves nothing.
	if puts := stack.puts.count(); puts == 0 {
		t.Fatal("probe recorded no store puts; invariant unverified")
	}

	markers := [][]byte{
		want,
		jf.streamBytes["movie-coral"],
		jf.streamBytes["series-tidal"],
	}
	if stack.puts.contains(markers...) {
		t.Fatalf("a store Put contained provider payload bytes; caches hold metadata and keys only (PLAN.md §2.4)")
	}
}
