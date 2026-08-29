// Package m4 holds the milestone acceptance tests for M4 (delivery): one
// passthrough play session and one remux download session end-to-end. Both
// run through the seams that ship in v1 — a sink wired from instance config
// via slotwiring, the delivery engine, and the API service arming it — so the
// test is a spec of what the composed stack does, not a unit probe.
package m4_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/delivery"
	"github.com/nem-git/abcmovies/core/internal/slotwiring"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// fakeResolver is a canned produce-sources surface: it hands back a fixed
// manifest for any item, standing in for a provider slot that an instance
// would need a live credential to reach.
type fakeResolver struct{ source *corev1.MediaSource }

func (f *fakeResolver) ProduceSources(_ context.Context, _, _, _ string) (*corev1.MediaSource, error) {
	return f.source, nil
}

// muxSource is a WHOLE_MUX manifest (a single containerised feature), the
// shape a remux download consumes.
func muxSource(providerURL string) *corev1.MediaSource {
	return &corev1.MediaSource{
		Type:        corev1.MediaSourceType_MEDIA_SOURCE_TYPE_STATIC,
		Seekable:    corev1.Seekable_SEEKABLE_FULL,
		Addressable: corev1.Addressable_ADDRESSABLE_WHOLE_MUX,
		Tracks: []*corev1.Track{
			{
				Id:    "feature",
				Media: &corev1.Track_Video{Video: &corev1.VideoTrack{Codec: "hevc"}},
				Delivery: &corev1.TrackDelivery{
					Locations:   []string{providerURL + "/mux"},
					AuthContext: "Bearer provider-secret",
				},
			},
		},
	}
}

// perTrackSource is a PER_TRACK manifest whose tracks live behind a provider
// that insists on an engine-held auth header.
func perTrackSource(providerURL string) *corev1.MediaSource {
	return &corev1.MediaSource{
		Type:        corev1.MediaSourceType_MEDIA_SOURCE_TYPE_STATIC,
		Seekable:    corev1.Seekable_SEEKABLE_FULL,
		Addressable: corev1.Addressable_ADDRESSABLE_PER_TRACK,
		Tracks: []*corev1.Track{
			{
				Id: "v1", Media: &corev1.Track_Video{Video: &corev1.VideoTrack{Codec: "h264"}},
				Delivery: &corev1.TrackDelivery{Locations: []string{providerURL + "/v1"}, AuthContext: "Bearer provider-secret"},
			},
			{
				Id: "a1", Media: &corev1.Track_Audio{Audio: &corev1.AudioTrack{Codec: "aac"}},
				Delivery: &corev1.TrackDelivery{Locations: []string{providerURL + "/a1"}, AuthContext: "Bearer provider-secret"},
			},
		},
	}
}

// buildServer wires the delivery engine over the given resolver and sink
// factory and returns an armed CoreService together with the engine its
// harness drives.
func buildServer(resolver delivery.Resolver, sinks delivery.SinkFactory) (*apiserver.Server, *delivery.Engine) {
	eng := delivery.New(delivery.Options{
		SessionTTL:        time.Hour,
		ConcurrentStreams: 3,
		SourceResolver:    resolver,
		SinkFactory:       sinks,
		RecordJob:         func(*corev1.Job) {},
	})
	bus := apiserver.NewInMemoryBus()
	srv := apiserver.NewServer(bus, config.Stores{Jobs: store.NewInMemory()}, nil, nil)
	srv.SetDelivery(eng)
	return srv, eng
}

// TestM4RemuxDownloadToDiskEndToEnd proves one remux download session
// end-to-end: a whole-mux feature is resolved, the disk sink names it by the
// output contract (§1.15), the consumer pushes the provider's bytes, and the
// finalised deliverable lands on disk. It runs through SlotEntry config → the
// composed sink factory → the engine → the API service.
func TestM4RemuxDownloadToDiskEndToEnd(t *testing.T) {
	featureBytes := []byte("feature-mux-bytes")
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write(featureBytes)
	}))
	defer provider.Close()

	root := t.TempDir()

	// Sinks come from instance config (slotwiring), exactly as v1 declares
	// them: one enabled disk entry naming its path.
	sinks, err := slotwiring.SetupSinks([]config.SlotEntry{{
		ID: "disk", Adapter: "disk", Enabled: true, Options: map[string]string{"path": root},
	}}, delivery.NewRelay())
	if err != nil {
		t.Fatalf("SetupSinks: %v", err)
	}

	srv, eng := buildServer(&fakeResolver{muxSource(provider.URL)}, sinks)
	defer eng.Close()

	resp, err := srv.StartDelivery(context.Background(), &apiv1.StartDeliveryRequest{
		Goal:           apiv1.DeliveryGoal_DELIVERY_GOAL_DOWNLOAD,
		Provider:       "jellyfin",
		AccountId:      "acc1",
		MemberUserId:   "u1",
		NativeId:       "item1",
		Sink:           "disk",
		SelectedTarget: "Inception (2010)",
		Container:      "mkv",
	})
	if err != nil {
		t.Fatalf("StartDelivery: %v", err)
	}
	job := resp.GetJob()
	if job.GetStatus() != corev1.JobStatus_JOB_STATUS_RUNNING {
		t.Fatalf("job status = %s, want running", job.GetStatus())
	}

	sess, ok := eng.Get(job.GetId())
	if !ok {
		t.Fatalf("session %q not found in engine", job.GetId())
	}
	if len(sess.Plan.Steps) != 2 || sess.Plan.Steps[0].Kind != delivery.StepRemux || sess.Plan.Steps[1].Kind != delivery.StepCompose {
		t.Fatalf("step chain = remux→compose, got %#v", sess.Plan.Steps)
	}

	// The download consumer pulls the provider's mux and pushes it through the
	// session's sink. The sink is the §1.15-named deliverable.
	for _, tr := range muxSource(provider.URL).GetTracks() {
		if _, err := sess.Sink.Deliver(context.Background(), sess, tr, strings.NewReader(string(featureBytes))); err != nil {
			t.Fatalf("Deliver %s: %v", tr.GetId(), err)
		}
	}
	if err := eng.Complete(sess.ID); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	final := filepath.Join(root, "Inception (2010).mkv")
	data, err := os.ReadFile(final)
	if err != nil {
		t.Fatalf("deliverable not on disk: %v", err)
	}
	if string(data) != string(featureBytes) {
		t.Errorf("deliverable content = %q, want %q", data, featureBytes)
	}
}

// TestM4PassthroughPlayToDeviceEndToEnd proves one passthrough play session
// end-to-end: a per-track source is resolved, the device sink stages a relay
// ticket per track, and a minimal built-in consumer pulls each track's bytes
// through the relay — never seeing the provider's auth context. The provider
// insists on an engine-held Authorization header, so credentials demonstrably
// never leave the engine (PLAN.md §3.6).
func TestM4PassthroughPlayToDeviceEndToEnd(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
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

	relay := delivery.NewRelay()
	srv, eng := buildServer(&fakeResolver{perTrackSource(provider.URL)}, &delivery.DeviceSinkFactory{Relay: relay})
	defer eng.Close()

	resp, err := srv.StartDelivery(context.Background(), &apiv1.StartDeliveryRequest{
		Goal:         apiv1.DeliveryGoal_DELIVERY_GOAL_PLAY,
		Provider:     "jellyfin",
		AccountId:    "acc1",
		MemberUserId: "u1",
		NativeId:     "item1",
		Sink:         "device",
	})
	if err != nil {
		t.Fatalf("StartDelivery: %v", err)
	}
	sess, ok := eng.Get(resp.GetJob().GetId())
	if !ok {
		t.Fatalf("session not found")
	}
	if len(sess.Plan.Steps) != 1 || sess.Plan.Steps[0].Kind != delivery.StepPassthrough {
		t.Fatalf("step chain = passthrough, got %#v", sess.Plan.Steps)
	}
	device, ok := sess.Sink.(*delivery.DeviceSink)
	if !ok {
		t.Fatalf("sink is %T, want *DeviceSink", sess.Sink)
	}

	want := map[string]string{"v1": "video-bytes", "a1": "audio-bytes"}
	src := perTrackSource(provider.URL)
	for _, tr := range src.GetTracks() {
		if _, err := sess.Sink.Deliver(context.Background(), sess, tr, nil); err != nil {
			t.Fatalf("Deliver %s: %v", tr.GetId(), err)
		}
		tok, ok := device.RelayToken(tr.GetId())
		if !ok {
			t.Fatalf("no relay ticket staged for %s", tr.GetId())
		}
		body, _, err := relay.Open(tok)
		if err != nil {
			t.Fatalf("relay.Open %s: %v", tr.GetId(), err)
		}
		data, _ := io.ReadAll(body)
		_ = body.Close()
		if string(data) != want[tr.GetId()] {
			t.Errorf("track %s relayed %q, want %q", tr.GetId(), data, want[tr.GetId()])
		}
	}

	if err := eng.Complete(sess.ID); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	// After finalize the session's relay tickets are revoked.
	v1tok, _ := device.RelayToken("v1")
	if _, _, err := relay.Open(v1tok); err == nil {
		t.Error("relay still served a ticket after the session finalized")
	}
}
