package m8

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/delivery"
	"github.com/nem-git/abcmovies/core/internal/slotwiring"
)

// TestM8SecondSinkTypeWorksWithoutCoreChanges is the M8 pluggability proof:
// the media-server sink (a second sink type) is declared as a slot, built by
// the generic sink registry, and driven by the engine exactly like a device
// sink — with no core dispatch change naming it (PLAN.md §6.4: sinks are added
// without touching the core). It stages a manifest and per-track relay tickets,
// and a pull serves clear bytes through the opener.
func TestM8SecondSinkTypeWorksWithoutCoreChanges(t *testing.T) {
	srv := encServer()
	defer srv.Close()

	drmClient := &fakeM8DRMClient{
		supported: []slotsv1.DrmSystem{slotsv1.DrmSystem_DRM_SYSTEM_CLEARKEY},
		keys: []*slotsv1.ContentKey{
			{KeyId: "kid-1", KeyMaterial: []byte("m"), Validity: durationpb.New(time.Hour)},
		},
	}
	cache := newFakeCache()
	opener := delivery.NewOpener(drmClient, cache, nil)
	relay := delivery.NewRelay()

	// The media-server sink is registered by its own wiring shim; the engine's
	// sink setup builds it through the generic registry.
	sinkFactory, err := slotwiring.SetupSinks([]*corev1.Slot{{
		Id:   "mediaserver",
		Kind: corev1.SlotKind_SLOT_KIND_SINK,
		Config: &corev1.Slot_Sink{
			Sink: &corev1.SinkConfig{Adapter: "mediaserver"},
		},
	}}, relay, opener)
	if err != nil {
		t.Fatalf("SetupSinks: %v", err)
	}

	e := delivery.New(delivery.Options{
		SessionTTL:      time.Hour,
		Now:             time.Now,
		RecordJob:       func(*corev1.Job) {},
		SourceResolver:  &fakeResolver{source: drmSource(srv.URL)},
		DRMClient:       drmClient,
		ContentKeyCache: cache,
		Opener:          opener,
		SinkFactory:     sinkFactory,
	})
	defer e.Close()

	s, err := e.Start(context.Background(), delivery.StartRequest{
		Goal:         delivery.GoalPlay,
		MemberUserID: "u",
		Provider:     "jellyfin",
		AccountID:    "a",
		NativeID:     "item-1",
		Sink:         "mediaserver",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	ms, ok := s.Sink.(*delivery.MediaServerSink)
	if !ok {
		t.Fatalf("sink = %T, want *delivery.MediaServerSink", s.Sink)
	}
	if ms.Pushes() {
		t.Fatal("media-server sink must be a pull sink")
	}

	manifestTok, err := ms.ManifestToken()
	if err != nil {
		t.Fatalf("ManifestToken: %v", err)
	}
	res, err := relay.Open(context.Background(), manifestTok)
	if err != nil {
		t.Fatalf("open manifest: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	manifest, _ := io.ReadAll(res.Body)
	if !strings.HasPrefix(string(manifest), "#EXTM3U") {
		t.Fatalf("manifest = %q, want an HLS playlist", manifest)
	}

	trackTok, ok := ms.TrackToken("v1")
	if !ok {
		t.Fatal("v1 not staged")
	}
	tres, err := relay.Open(context.Background(), trackTok)
	if err != nil {
		t.Fatalf("open track: %v", err)
	}
	defer func() { _ = tres.Body.Close() }()
	body, _ := io.ReadAll(tres.Body)
	if string(body) != "clear-encrypted-bytes" {
		t.Fatalf("track body = %q, want clear-encrypted-bytes", body)
	}
}
