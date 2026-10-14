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
// a play session that requests packaging (Format: HLS) ends in a Package step,
// stages per-track relay tokens, and the Package step assembles them into a
// manifest the player pulls. The sink is a RelaySink (the collapsed pull sink);
// the pipeline owns the manifest, not the sink.
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

	sinkFactory, err := slotwiring.SetupSinks([]*corev1.Slot{{
		Id:   "relay",
		Kind: corev1.SlotKind_SLOT_KIND_SINK,
		Config: &corev1.Slot_Sink{
			Sink: &corev1.SinkConfig{Adapter: "relay"},
		},
	}}, relay, opener)
	if err != nil {
		t.Fatalf("SetupSinks: %v", err)
	}

	e := delivery.New(delivery.Options{
		SessionTTL:      time.Hour,
		Relay:           relay,
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
		Sink:         "relay",
		Format:       delivery.PackagingHLS,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	rs, ok := s.Sink.(*delivery.RelaySink)
	if !ok {
		t.Fatalf("sink = %T, want *delivery.RelaySink", s.Sink)
	}
	if rs.Pushes() {
		t.Fatal("relay sink must be a pull sink")
	}
	if s.Plan.Terminal() != delivery.StepPackage {
		t.Fatalf("terminal step = %q, want package", s.Plan.Terminal())
	}

	manifestTok, ok := e.ManifestToken(s.ID)
	if !ok {
		t.Fatal("no manifest token granted")
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

	trackTok, ok := rs.RelayToken("v1")
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
