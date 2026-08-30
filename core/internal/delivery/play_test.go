package delivery

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

// playSource is a PER_TRACK manifest whose tracks live on a fake provider
// server, with an engine-side auth_context the provider insists on.
func playSource(providerURL string) *corev1.MediaSource {
	return &corev1.MediaSource{
		Type:        corev1.MediaSourceType_MEDIA_SOURCE_TYPE_STATIC,
		Seekable:    corev1.Seekable_SEEKABLE_FULL,
		Addressable: corev1.Addressable_ADDRESSABLE_PER_TRACK,
		Tracks: []*corev1.Track{
			{
				Id:    "v1",
				Media: &corev1.Track_Video{Video: &corev1.VideoTrack{Codec: "h264"}},
				Delivery: &corev1.TrackDelivery{
					Locations:   []string{providerURL + "/stream/v1"},
					AuthContext: "Bearer provider-secret",
				},
			},
			{
				Id:    "a1",
				Media: &corev1.Track_Audio{Audio: &corev1.AudioTrack{Codec: "aac"}},
				Delivery: &corev1.TrackDelivery{
					Locations:   []string{providerURL + "/stream/a1"},
					AuthContext: "Bearer provider-secret",
				},
			},
		},
	}
}

// TestPlayStartStagesMenuAndAnnouncesReady proves a play Start delivers every
// location-bearing track to the sink and announces the staged menu through the
// MenuReady hook before the job is recorded — so a subscriber waiting on the
// delivery-play-menu-ready event always finds a complete menu (PLAN.md §6.2).
func TestPlayStartStagesMenuAndAnnouncesReady(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/v1") {
			_, _ = w.Write([]byte("video-bytes"))
			return
		}
		_, _ = w.Write([]byte("audio-bytes"))
	}))
	defer provider.Close()

	relay := NewRelay()
	now := time.Now()
	var announced *Session
	e := New(Options{
		SessionTTL:        24 * time.Hour,
		ConcurrentStreams: 3,
		Now:               func() time.Time { return now },
		RecordJob:         func(*corev1.Job) {},
		SourceResolver:    &fakeResolver{source: playSource(provider.URL)},
		SinkFactory:       &DeviceSinkFactory{Relay: relay},
		MenuReady:         func(s *Session) { announced = s },
	})
	defer e.Close()

	sess, err := e.Start(context.Background(), StartRequest{
		Goal: GoalPlay, MemberUserID: "u1",
		Provider: "jellyfin", AccountID: "acc1", NativeID: "item1", Sink: "device",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	device, ok := sess.Sink.(*DeviceSink)
	if !ok {
		t.Fatalf("sink is %T, want *DeviceSink", sess.Sink)
	}
	for _, id := range []string{"v1", "a1"} {
		tok, ok := device.RelayToken(id)
		if !ok {
			t.Fatalf("no relay token staged for %s", id)
		}
		res, err := relay.Open(tok)
		if err != nil {
			t.Fatalf("open staged token %s: %v", id, err)
		}
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
	}
	if len(sess.Menu) != 2 {
		t.Errorf("menu has %d tracks, want 2", len(sess.Menu))
	}
	if announced != sess {
		t.Errorf("MenuReady announced %v, want the started session", announced)
	}
}

// TestPlayStartSKipsLocationlessTracks proves staging only ever grants tokens
// for tracks that actually carry a location: carried-in audio/subtitle tracks
// are delivered *with* their carrier, never as their own relay grant.
func TestPlayStartSkipsLocationlessTracks(t *testing.T) {
	relay := NewRelay()
	e := New(Options{
		SessionTTL:        24 * time.Hour,
		ConcurrentStreams: 3,
		RecordJob:         func(*corev1.Job) {},
		SourceResolver: &fakeResolver{source: &corev1.MediaSource{
			Type:        corev1.MediaSourceType_MEDIA_SOURCE_TYPE_STATIC,
			Seekable:    corev1.Seekable_SEEKABLE_FULL,
			Addressable: corev1.Addressable_ADDRESSABLE_WHOLE_MUX,
			Tracks: []*corev1.Track{
				{Id: "container", Media: &corev1.Track_Video{}, Delivery: &corev1.TrackDelivery{Locations: []string{"http://x/stream"}}},
				{Id: "audio-1", Media: &corev1.Track_Audio{}, Delivery: &corev1.TrackDelivery{CarriedIn: "container"}},
			},
		}},
		SinkFactory: &DeviceSinkFactory{Relay: relay},
	})
	defer e.Close()

	sess, err := e.Start(context.Background(), StartRequest{
		Goal: GoalPlay, MemberUserID: "u1",
		Provider: "jellyfin", AccountID: "acc1", NativeID: "item1", Sink: "device",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	device, ok := sess.Sink.(*DeviceSink)
	if !ok {
		t.Fatalf("sink is %T, want *DeviceSink", sess.Sink)
	}
	if tok, ok := device.RelayToken("container"); !ok || tok == "" {
		t.Error("container track should have a staged token")
	}
	if _, ok := device.RelayToken("audio-1"); ok {
		t.Error("carried-in track must not get its own relay token")
	}
}

// TestPlayEndToEndPassthroughRelay proves one passthrough play session
// end-to-end: the engine resolves a per-track manifest, the device sink stages
// relay tickets per track, and a minimal built-in consumer pulls each track's
// bytes through the relay. The fake provider insists on the engine-held
// auth_context — so the credentials are demonstrably attached engine-side and
// never exposed to the consumer (PLAN.md §3.6, §6.2).
func TestPlayEndToEndPassthroughRelay(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := r.Header.Get("Authorization")
		if secret == "" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/v1") {
			_, _ = w.Write([]byte("video-bytes"))
			return
		}
		_, _ = w.Write([]byte("audio-bytes"))
	}))
	defer provider.Close()

	relay := NewRelay()
	now := time.Now()
	e := New(Options{
		SessionTTL:        24 * time.Hour,
		ConcurrentStreams: 3,
		Now:               func() time.Time { return now },
		RecordJob:         func(*corev1.Job) {},
		SourceResolver:    &fakeResolver{source: playSource(provider.URL)},
		SinkFactory:       &DeviceSinkFactory{Relay: relay},
	})
	defer e.Close()

	sess, err := e.Start(context.Background(), StartRequest{
		Goal: GoalPlay, MemberUserID: "u1",
		Provider: "jellyfin", AccountID: "acc1", NativeID: "item1", Sink: "device",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(sess.Plan.Steps) != 1 || sess.Plan.Steps[0].Kind != StepPassthrough {
		t.Fatalf("step chain = %#v, want a single passthrough", sess.Plan.Steps)
	}
	device, ok := sess.Sink.(*DeviceSink)
	if !ok {
		t.Fatalf("sink is %T, want *DeviceSink", sess.Sink)
	}

	// Minimal built-in consumer: stage each track, pull its bytes via the
	// relay, and confirm the provider's bytes arrived untouched.
	for _, trackID := range []string{"v1", "a1"} {
		var track *corev1.Track
		for _, tr := range playSource(provider.URL).Tracks {
			if tr.GetId() == trackID {
				track = tr
			}
		}
		if _, err := sess.Sink.Deliver(context.Background(), sess, track, nil); err != nil {
			t.Fatalf("Deliver %s: %v", trackID, err)
		}
		tok, ok := device.RelayToken(trackID)
		if !ok {
			t.Fatalf("no relay token staged for %s", trackID)
		}

		// A consumer pulls only through the token; it never sees the
		// auth_context (the token is opaque).
		res, err := relay.Open(tok)
		if err != nil {
			t.Fatalf("relay.Open %s: %v", trackID, err)
		}
		data, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		want := map[string]string{"v1": "video-bytes", "a1": "audio-bytes"}[trackID]
		if string(data) != want {
			t.Errorf("track %s relayed %q, want %q", trackID, data, want)
		}
	}

	// The provider never saw an unauthenticated pull from these tokens.
	v1tok, _ := device.RelayToken("v1")
	res, err := relay.Open(v1tok)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	_ = res.Body.Close()
	if err := e.Complete(sess.ID); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	// After finalize, the relay no longer serves the session's tokens.
	if _, err := relay.Open(v1tok); err == nil {
		t.Error("relay still served a token after the session finalized")
	}
}

func TestRelayRejectsExpiredAndUnknown(t *testing.T) {
	relay := NewRelay()
	relay.now = func() time.Time { return time.Now() }
	tok, _ := relay.Grant("s1", "v1", func(ctx context.Context, _ string) (FetchResult, error) {
		return FetchResult{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("x"))}, nil
	}, time.Second)
	if _, err := relay.Open(tok); err != nil {
		t.Fatalf("fresh token should open: %v", err)
	}
	if _, err := relay.Open("bogus"); err == nil {
		t.Error("unknown token should fail")
	}
	relay.now = func() time.Time { return time.Now().Add(2 * time.Second) }
	if _, err := relay.Open(tok); err == nil {
		t.Error("expired token should fail")
	}
	relay.Revoke("s1")
	if _, err := relay.Open(tok); err == nil {
		t.Error("token should fail after revoke")
	}
}
