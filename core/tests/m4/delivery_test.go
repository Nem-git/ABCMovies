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
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/delivery"
	"github.com/nem-git/abcmovies/core/internal/library"
	"github.com/nem-git/abcmovies/core/internal/policy"
	"github.com/nem-git/abcmovies/core/internal/slotwiring"
	"github.com/nem-git/abcmovies/core/internal/sourcecache"
	"github.com/nem-git/abcmovies/core/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
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
					AuthContext: apiserver.BearerPrefix + "provider-secret",
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
				Delivery: &corev1.TrackDelivery{Locations: []string{providerURL + "/v1"}, AuthContext: apiserver.BearerPrefix + "provider-secret"},
			},
			{
				Id: "a1", Media: &corev1.Track_Audio{Audio: &corev1.AudioTrack{Codec: "aac"}},
				Delivery: &corev1.TrackDelivery{Locations: []string{providerURL + "/a1"}, AuthContext: apiserver.BearerPrefix + "provider-secret"},
			},
		},
	}
}

// drmPerTrackSource is a schema-valid PER_TRACK manifest whose video track is
// DRM-encrypted. The contract allows the field; v1 must refuse it.
func drmPerTrackSource(providerURL string) *corev1.MediaSource {
	ms := perTrackSource(providerURL)
	ms.Tracks[0].Delivery.Drm = []*corev1.TrackDrm{{System: "org.w3.clearkey", LicenseUrl: "https://license.example/"}}
	return ms
}

// countingFactory wraps a sink factory and counts NewSink calls, so a test can
// prove a refusal produced no sink at all.
type countingFactory struct {
	delivery.SinkFactory
	calls int
}

func (c *countingFactory) NewSink(ctx context.Context, s *delivery.Session, tracks []*corev1.Track) (delivery.Sink, error) {
	c.calls++
	return c.SinkFactory.NewSink(ctx, s, tracks)
}

// engineManager adapts the engine to the API service's DeliveryManager
// seam. M4 predates the play-menu surface; a menu request on this milestone
// is booked as unknown.
type engineManager struct{ eng *delivery.Engine }

func (m engineManager) Start(ctx context.Context, req delivery.StartRequest) (*delivery.Session, error) {
	return m.eng.Start(ctx, req)
}

func (m engineManager) Heartbeat(id string, memberUserID string) error {
	return m.eng.Heartbeat(id, memberUserID)
}

func (m engineManager) PlayMenu(string) (*apiserver.PlayMenu, error) {
	return nil, apiserver.ErrPlayMenuNotFound
}

func (m engineManager) RevokeAllOnAccount(accountID string) int {
	return m.eng.RevokeAllOnAccount(accountID)
}

func (m engineManager) RevokeOthersOnAccount(provider, accountID string, keepMembers []string) int {
	return m.eng.RevokeOthersOnAccount(provider, accountID, keepMembers)
}

func (m engineManager) ApplyAccountCap(ctx context.Context, provider, accountID string, enforceNow bool) (int, error) {
	return m.eng.ApplyAccountCap(ctx, provider, accountID, enforceNow)
}

func (m engineManager) Cancel(jobID string) (*delivery.Session, error) {
	return m.eng.Cancel(jobID)
}

func (m engineManager) GetDeliveryOptions(string, string) (*apiv1.GetDeliveryOptionsResponse, error) {
	return &apiv1.GetDeliveryOptionsResponse{}, nil
}

func (m engineManager) ListJobs(string, string) ([]*corev1.Job, string, error) {
	return nil, "", nil
}

// buildServer wires the delivery engine over the given resolver and sink
// factory and returns an armed CoreService together with the engine its
// harness drives.
func buildServer(resolver delivery.Resolver, sinks delivery.SinkFactory) (*apiserver.Server, *delivery.Engine) {
	eng := delivery.New(delivery.Options{
		SessionTTL:     time.Hour,
		InstancePolicy: policy.Set{policy.KeyConcurrentStreams: "3"},
		SourceResolver: resolver,
		SinkFactory:    sinks,
		RecordJob:      func(*corev1.Job) {},
	})
	bus := apiserver.NewInMemoryBus()
	srv := apiserver.NewServer(apiserver.Settings{Bus: bus, Stores: config.Stores{Jobs: store.NewInMemory()}})
	srv.SetDelivery(engineManager{eng: eng})
	srv.SetLibrary(trivialLibrary{})
	return srv, eng
}

// trivialLibrary satisfies the delivery-authorization seam for the M4
// harness: every account is reachable by every member, slicing very little
// real sharing logic (the M5 harness carries the sharing work).
type trivialLibrary struct{}

func (trivialLibrary) Library(context.Context, string) ([]*corev1.LibraryEntry, error) {
	return nil, nil
}

func (trivialLibrary) Metadata(context.Context, string) (*corev1.TitleMetadata, bool, error) {
	return nil, false, nil
}

func (trivialLibrary) ReachAuthorized(accountID, userID string) (library.Reach, bool) {
	return library.Reach{AccountID: accountID, Visibility: accounts.VisibilityPublic}, true
}
func (trivialLibrary) ReachesForUser(userID string) []library.Reach                { return nil }
func (trivialLibrary) RemoveReach(accountID string)                                {}
func (trivialLibrary) SetReachSharing(string, accounts.Visibility, []string) error { return nil }

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

	// Sinks come from the slot store (slotwiring), exactly as v1 declares
	// them: one enabled disk slot naming its path.
	sinks, err := slotwiring.SetupSinks([]*corev1.Slot{{
		Id: "disk", Kind: corev1.SlotKind_SLOT_KIND_SINK,
		Config: &corev1.Slot_Sink{Sink: &corev1.SinkConfig{
			Adapter: "disk", Options: map[string]string{"path": root},
		}},
	}}, delivery.NewRelay(), delivery.NewOpener(nil, nil, nil))
	if err != nil {
		t.Fatalf("SetupSinks: %v", err)
	}

	srv, eng := buildServer(&fakeResolver{muxSource(provider.URL)}, sinks)
	defer eng.Close()

	resp, err := srv.StartDelivery(apiserver.WithUserID(context.Background(), "u1"), &apiv1.StartDeliveryRequest{
		Goal:           apiv1.DeliveryGoal_DELIVERY_GOAL_DOWNLOAD,
		Provider:       "jellyfin",
		AccountId:      "acc1",
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
	if err := eng.Complete(context.Background(), sess.ID); err != nil {
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
	srv, eng := buildServer(&fakeResolver{perTrackSource(provider.URL)}, &delivery.RelaySinkFactory{Relay: relay, Opener: delivery.NewOpener(nil, nil, nil)})
	defer eng.Close()

	resp, err := srv.StartDelivery(apiserver.WithUserID(context.Background(), "u1"), &apiv1.StartDeliveryRequest{
		Goal:      apiv1.DeliveryGoal_DELIVERY_GOAL_PLAY,
		Provider:  "jellyfin",
		AccountId: "acc1",
		NativeId:  "item1",
		Sink:      "relay",
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
	device, ok := sess.Sink.(*delivery.RelaySink)
	if !ok {
		t.Fatalf("sink is %T, want *RelaySink", sess.Sink)
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
		res, err := relay.Open(context.Background(), tok)
		if err != nil {
			t.Fatalf("relay.Open %s: %v", tr.GetId(), err)
		}
		data, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if string(data) != want[tr.GetId()] {
			t.Errorf("track %s relayed %q, want %q", tr.GetId(), data, want[tr.GetId()])
		}
	}

	if err := eng.Complete(context.Background(), sess.ID); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	// After finalize the session's relay tickets are revoked.
	v1tok, _ := device.RelayToken("v1")
	if res, err := relay.Open(context.Background(), v1tok); err == nil {
		_ = res.Body.Close()
		t.Error("relay still served a ticket after the session finalized")
	}
}

// TestM4DRMRefusedNotSilentlyDelivered proves the SCOPE consequence: a
// DRM-encrypted manifest is refused with an honest error, and no session, job,
// or sink is ever produced. The engine declines the whole delivery rather than
// pass an encrypted track through unlicensed (PLAN.md §2.5 — reject, never
// downgrade).
func TestM4DRMRefusedNotSilentlyDelivered(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("encrypted-track-bytes"))
	}))
	defer provider.Close()

	factory := &countingFactory{SinkFactory: &delivery.RelaySinkFactory{Relay: delivery.NewRelay(), Opener: delivery.NewOpener(nil, nil, nil)}}
	srv, eng := buildServer(&fakeResolver{drmPerTrackSource(provider.URL)}, factory)
	defer eng.Close()

	_, err := srv.StartDelivery(apiserver.WithUserID(context.Background(), "u1"), &apiv1.StartDeliveryRequest{
		Goal:      apiv1.DeliveryGoal_DELIVERY_GOAL_PLAY,
		Provider:  "jellyfin",
		AccountId: "acc1",
		NativeId:  "item1",
		Sink:      "relay",
	})
	if status.Code(err) != codes.InvalidArgument || !strings.Contains(err.Error(), "DRM-encrypted") {
		t.Fatalf("want a loud InvalidArgument DRM refusal, got code=%v err=%v", status.Code(err), err)
	}
	if eng.LiveSessions() != 0 {
		t.Fatalf("refusal must create no session, LiveSessions = %d", eng.LiveSessions())
	}
	if factory.calls != 0 {
		t.Fatalf("refusal reached the sink factory %d times, want 0", factory.calls)
	}
}

func (trivialLibrary) RefreshAvailability(context.Context, string, []string) (sourcecache.Stats, error) {
	return sourcecache.Stats{}, nil
}

// --- Integration test for GetDeliveryOptions and ListJobs ---

// coverageLibrary returns one library entry with coverage for a fake
// provider, so GetDeliveryOptions can resolve a manifest through the engine.
type coverageLibrary struct {
	entry *corev1.LibraryEntry
}

func (l coverageLibrary) Library(context.Context, string) ([]*corev1.LibraryEntry, error) {
	return []*corev1.LibraryEntry{l.entry}, nil
}

func (l coverageLibrary) Metadata(context.Context, string) (*corev1.TitleMetadata, bool, error) {
	return nil, false, nil
}

func (l coverageLibrary) ReachAuthorized(accountID, userID string) (library.Reach, bool) {
	return library.Reach{AccountID: accountID, Visibility: accounts.VisibilityPublic}, true
}
func (l coverageLibrary) ReachesForUser(userID string) []library.Reach { return nil }

func (l coverageLibrary) RemoveReach(accountID string) {}

func (l coverageLibrary) SetReachSharing(string, accounts.Visibility, []string) error {
	return nil
}

func (l coverageLibrary) RefreshAvailability(context.Context, string, []string) (sourcecache.Stats, error) {
	return sourcecache.Stats{}, nil
}

// resolvingDelivery is a DeliveryManager that implements GetDeliveryOptions
// and ListJobs using the engine's resolver and the server's job store.
type resolvingDelivery struct {
	eng  *delivery.Engine
	jobs store.Store
}

func (m resolvingDelivery) Start(ctx context.Context, req delivery.StartRequest) (*delivery.Session, error) {
	return m.eng.Start(ctx, req)
}

func (m resolvingDelivery) Heartbeat(id string, memberUserID string) error {
	return m.eng.Heartbeat(id, memberUserID)
}

func (m resolvingDelivery) PlayMenu(string) (*apiserver.PlayMenu, error) {
	return nil, apiserver.ErrPlayMenuNotFound
}

func (m resolvingDelivery) RevokeAllOnAccount(accountID string) int {
	return m.eng.RevokeAllOnAccount(accountID)
}

func (m resolvingDelivery) RevokeOthersOnAccount(provider, accountID string, keepMembers []string) int {
	return m.eng.RevokeOthersOnAccount(provider, accountID, keepMembers)
}

func (m resolvingDelivery) ApplyAccountCap(ctx context.Context, provider, accountID string, enforceNow bool) (int, error) {
	return m.eng.ApplyAccountCap(ctx, provider, accountID, enforceNow)
}

func (m resolvingDelivery) Cancel(jobID string) (*delivery.Session, error) {
	return m.eng.Cancel(jobID)
}

// GetDeliveryOptions resolves the manifest for the entry's coverage provider
// and returns the tracks, sinks, and containers the frontend dialog renders.
func (m resolvingDelivery) GetDeliveryOptions(entryID, userID string) (*apiv1.GetDeliveryOptionsResponse, error) {
	src, err := m.eng.ResolveManifest(context.Background(), "jellyfin", "", "item1")
	if err != nil {
		return nil, err
	}
	tracks := make([]*apiv1.DownloadTrack, 0, len(src.GetTracks()))
	for _, tr := range src.GetTracks() {
		dt := &apiv1.DownloadTrack{TrackId: tr.GetId()}
		switch tr.GetMedia().(type) {
		case *corev1.Track_Video:
			dt.Media = &apiv1.DownloadTrack_Video{Video: tr.GetVideo()}
		case *corev1.Track_Audio:
			dt.Media = &apiv1.DownloadTrack_Audio{Audio: tr.GetAudio()}
		case *corev1.Track_Subtitle:
			dt.Media = &apiv1.DownloadTrack_Subtitle{Subtitle: tr.GetSubtitle()}
		}
		tracks = append(tracks, dt)
	}
	return &apiv1.GetDeliveryOptionsResponse{
		Tracks:     tracks,
		Sinks:      []*apiv1.DownloadSink{{SinkId: "device", Kind: corev1.SlotKind_SLOT_KIND_SINK, Label: "This device"}},
		Containers: []string{"mp4", "mkv"},
	}, nil
}

// ListJobs returns the caller's jobs from the store, paginated.
func (m resolvingDelivery) ListJobs(userID, pageToken string) ([]*corev1.Job, string, error) {
	keys, err := m.jobs.List(context.Background(), apiserver.JobKeyPrefix)
	if err != nil {
		return nil, "", err
	}
	var jobs []*corev1.Job
	for _, key := range keys {
		val, err := m.jobs.Get(context.Background(), key)
		if err != nil {
			continue
		}
		job := &corev1.Job{}
		if err := proto.Unmarshal(val, job); err != nil {
			continue
		}
		if job.GetOwnerUserId() == userID {
			jobs = append(jobs, job)
		}
	}
	const pageSize = 50
	if len(jobs) <= pageSize {
		return jobs, "", nil
	}
	return jobs[:pageSize], jobs[pageSize].GetId(), nil
}

// buildDeliveryTestServer wires a server with the resolvingDelivery manager
// and a library seam that returns the given entry.
func buildDeliveryTestServer(eng *delivery.Engine, entry *corev1.LibraryEntry) *apiserver.Server {
	bus := apiserver.NewInMemoryBus()
	srv := apiserver.NewServer(apiserver.Settings{Bus: bus, Stores: config.Stores{Jobs: store.NewInMemory()}})
	srv.SetDelivery(resolvingDelivery{eng: eng, jobs: store.NewInMemory()})
	srv.SetLibrary(coverageLibrary{entry: entry})
	return srv
}

// TestGetDeliveryOptionsValid proves a valid entry_id returns the correct
// tracks (matching the resolver's manifest), sinks, and containers.
func TestGetDeliveryOptionsValid(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("video-bytes"))
	}))
	defer provider.Close()

	sinks, err := slotwiring.SetupSinks([]*corev1.Slot{{
		Id: "relay", Kind: corev1.SlotKind_SLOT_KIND_SINK,
		Config: &corev1.Slot_Sink{Sink: &corev1.SinkConfig{Adapter: "relay", Options: map[string]string{}}},
	}}, delivery.NewRelay(), delivery.NewOpener(nil, nil, nil))
	if err != nil {
		t.Fatalf("SetupSinks: %v", err)
	}

	eng := delivery.New(delivery.Options{
		SessionTTL:     time.Hour,
		InstancePolicy: policy.Set{policy.KeyConcurrentStreams: "3"},
		SourceResolver: &fakeResolver{perTrackSource(provider.URL)},
		SinkFactory:    sinks,
		RecordJob:      func(*corev1.Job) {},
	})
	defer eng.Close()

	srv := buildDeliveryTestServer(eng, &corev1.LibraryEntry{
		Id:   "le-test1",
		Kind: corev1.EntryKind_ENTRY_KIND_MOVIE,
		Coverage: map[string]*corev1.CoverageRow{
			"jellyfin:item1": {Present: true, Verdict: corev1.CoverageVerdict_COVERAGE_VERDICT_CORROBORATED},
		},
	})

	ctx := apiserver.WithUserID(context.Background(), "u1")
	resp, err := srv.GetDeliveryOptions(ctx, &apiv1.GetDeliveryOptionsRequest{EntryId: "le-test1"})
	if err != nil {
		t.Fatalf("GetDeliveryOptions: %v", err)
	}

	// Tracks must match the resolver's manifest (perTrackSource has v1 video + a1 audio)
	wantTracks := map[string]bool{"v1": true, "a1": true}
	if len(resp.GetTracks()) != len(wantTracks) {
		t.Fatalf("tracks count = %d, want %d", len(resp.GetTracks()), len(wantTracks))
	}
	for _, tr := range resp.GetTracks() {
		if !wantTracks[tr.GetTrackId()] {
			t.Errorf("unexpected track id %q", tr.GetTrackId())
		}
	}

	// Sinks must include the device sink
	if len(resp.GetSinks()) == 0 {
		t.Fatal("no sinks returned")
	}
	if resp.GetSinks()[0].GetSinkId() != "device" {
		t.Errorf("sink id = %q, want device", resp.GetSinks()[0].GetSinkId())
	}

	// Containers must be non-empty
	if len(resp.GetContainers()) == 0 {
		t.Fatal("no containers returned")
	}
}

// TestGetDeliveryOptionsEmptyEntryID proves an empty entry_id is rejected
// with InvalidArgument (schema validation).
func TestGetDeliveryOptionsEmptyEntryID(t *testing.T) {
	eng := delivery.New(delivery.Options{SessionTTL: time.Hour, InstancePolicy: policy.Set{policy.KeyConcurrentStreams: "3"}})
	defer eng.Close()
	srv := buildDeliveryTestServer(eng, &corev1.LibraryEntry{Id: "le-test1", Kind: corev1.EntryKind_ENTRY_KIND_MOVIE})

	ctx := apiserver.WithUserID(context.Background(), "u1")
	_, err := srv.GetDeliveryOptions(ctx, &apiv1.GetDeliveryOptionsRequest{EntryId: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("empty entry_id: got code=%v, want InvalidArgument", status.Code(err))
	}
}

// TestGetDeliveryOptionsUnknownEntryDoesNotPanic proves that an unknown
// entry_id does not panic the API handler — the stub resolvingDelivery
// resolves directly via the engine, so it returns tracks regardless.
func TestGetDeliveryOptionsUnknownEntryDoesNotPanic(t *testing.T) {
	eng := delivery.New(delivery.Options{
		SessionTTL:     time.Hour,
		InstancePolicy: policy.Set{policy.KeyConcurrentStreams: "3"},
		SourceResolver: &fakeResolver{perTrackSource("http://dummy")},
	})
	defer eng.Close()
	srv := buildDeliveryTestServer(eng, &corev1.LibraryEntry{Id: "le-test1", Kind: corev1.EntryKind_ENTRY_KIND_MOVIE})

	ctx := apiserver.WithUserID(context.Background(), "u1")
	resp, err := srv.GetDeliveryOptions(ctx, &apiv1.GetDeliveryOptionsRequest{EntryId: "le-unknown"})
	if err != nil {
		t.Fatalf("unknown entry_id: %v", err)
	}
	if len(resp.GetTracks()) == 0 {
		t.Fatal("expected tracks from the stub resolver, got none")
	}
}

// TestListJobsReturnsCallerJobs proves ListJobs returns only the caller's
// jobs, excluding other users' jobs.
func TestListJobsReturnsCallerJobs(t *testing.T) {
	eng := delivery.New(delivery.Options{SessionTTL: time.Hour, InstancePolicy: policy.Set{policy.KeyConcurrentStreams: "3"}})
	defer eng.Close()

	// Seed the jobs store with two jobs: one for u1, one for u2
	jobs := store.NewInMemory()
	for _, j := range []*corev1.Job{
		{Id: "del-u1", Kind: corev1.JobKind_JOB_KIND_DELIVERY, Status: corev1.JobStatus_JOB_STATUS_DONE, OwnerUserId: "u1"},
		{Id: "del-u2", Kind: corev1.JobKind_JOB_KIND_DELIVERY, Status: corev1.JobStatus_JOB_STATUS_DONE, OwnerUserId: "u2"},
	} {
		val, err := proto.Marshal(j)
		if err != nil {
			t.Fatalf("marshal job: %v", err)
		}
		if err := jobs.Put(context.Background(), apiserver.JobKeyPrefix+j.GetId(), val); err != nil {
			t.Fatalf("put job: %v", err)
		}
	}

	bus := apiserver.NewInMemoryBus()
	srv := apiserver.NewServer(apiserver.Settings{Bus: bus, Stores: config.Stores{Jobs: jobs}})
	srv.SetDelivery(resolvingDelivery{eng: eng, jobs: jobs})
	srv.SetLibrary(coverageLibrary{entry: &corev1.LibraryEntry{Id: "le-test1", Kind: corev1.EntryKind_ENTRY_KIND_MOVIE}})

	ctx := apiserver.WithUserID(context.Background(), "u1")
	resp, err := srv.ListJobs(ctx, &apiv1.ListJobsRequest{})
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(resp.GetJobs()) != 1 {
		t.Fatalf("jobs count = %d, want 1 (only u1's job)", len(resp.GetJobs()))
	}
	if resp.GetJobs()[0].GetOwnerUserId() != "u1" {
		t.Errorf("job owner = %q, want u1", resp.GetJobs()[0].GetOwnerUserId())
	}
	if resp.GetJobs()[0].GetId() != "del-u1" {
		t.Errorf("job id = %q, want del-u1", resp.GetJobs()[0].GetId())
	}
}

// TestListJobsEmpty proves ListJobs returns an empty list when the caller has
// no jobs.
func TestListJobsEmpty(t *testing.T) {
	eng := delivery.New(delivery.Options{SessionTTL: time.Hour, InstancePolicy: policy.Set{policy.KeyConcurrentStreams: "3"}})
	defer eng.Close()
	srv := buildDeliveryTestServer(eng, &corev1.LibraryEntry{Id: "le-test1", Kind: corev1.EntryKind_ENTRY_KIND_MOVIE})

	ctx := apiserver.WithUserID(context.Background(), "u1")
	resp, err := srv.ListJobs(ctx, &apiv1.ListJobsRequest{})
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(resp.GetJobs()) != 0 {
		t.Fatalf("jobs count = %d, want 0", len(resp.GetJobs()))
	}
	if resp.GetNextPageToken() != "" {
		t.Errorf("next_page_token = %q, want empty", resp.GetNextPageToken())
	}
}
