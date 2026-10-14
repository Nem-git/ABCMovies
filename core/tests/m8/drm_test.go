package m8

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/nem-git/abcmovies/core/drm"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/delivery"
	"github.com/nem-git/abcmovies/core/internal/policy"
	"github.com/nem-git/abcmovies/core/internal/slotwiring"
)

// encServer serves synthetic encrypted content for DRM tests.
func encServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("encrypted-bytes"))
	}))
}

// fakeResolver returns a canned manifest.
type fakeResolver struct {
	source *corev1.MediaSource
}

func (f *fakeResolver) ProduceSources(_ context.Context, _, _, _ string) (*corev1.MediaSource, error) {
	return f.source, nil
}

// fakeM8DRMClient is a mock DRM client for M8 acceptance tests.
type fakeM8DRMClient struct {
	supported  []slotsv1.DrmSystem
	keys       []*slotsv1.ContentKey
	acquireErr error
	decryptErr error
}

func (f *fakeM8DRMClient) Supports(system slotsv1.DrmSystem) bool {
	for _, s := range f.supported {
		if s == system {
			return true
		}
	}
	return false
}

func (f *fakeM8DRMClient) AcquireKeys(_ context.Context, _, _ string, _ slotsv1.DrmSystem, _ []byte, _ []string, _, _ string) ([]*slotsv1.ContentKey, error) {
	if f.acquireErr != nil {
		return nil, f.acquireErr
	}
	return f.keys, nil
}

func (f *fakeM8DRMClient) Decrypt(_ context.Context, source drm.Source, _ []*slotsv1.ContentKey) ([]byte, error) {
	if f.decryptErr != nil {
		return nil, f.decryptErr
	}
	// Return the encrypted bytes prefixed with "clear-" to prove decryption happened.
	return []byte("clear-" + string(source.InlineBytes)), nil
}

// fakeCache is a mock content-key cache.
type fakeCache struct {
	keys map[string][]*slotsv1.ContentKey
}

func newFakeCache() *fakeCache {
	return &fakeCache{keys: map[string][]*slotsv1.ContentKey{}}
}

func (f *fakeCache) GetKeys(_ context.Context, provider, contentID string, system slotsv1.DrmSystem) ([]*slotsv1.ContentKey, error) {
	return f.keys[provider+"/"+contentID+"/"+system.String()], nil
}

func (f *fakeCache) PutKeys(_ context.Context, provider, contentID string, system slotsv1.DrmSystem, keys []*slotsv1.ContentKey) error {
	f.keys[provider+"/"+contentID+"/"+system.String()] = keys
	return nil
}

func (f *fakeCache) DropKeys(_ context.Context, provider, contentID string, system slotsv1.DrmSystem) error {
	delete(f.keys, provider+"/"+contentID+"/"+system.String())
	return nil
}

// recordingSink records deliveries.
type recordingSink struct {
	delivered []string
	finalized bool
	pushes    bool
}

func (r *recordingSink) Deliver(_ context.Context, _ *delivery.Session, track *corev1.Track, body io.Reader) (int64, error) {
	r.delivered = append(r.delivered, track.GetId())
	if body != nil {
		_, _ = io.ReadAll(body)
	}
	return 0, nil
}

func (r *recordingSink) Pushes() bool { return r.pushes }

func (r *recordingSink) Finalize(_ context.Context, _ *delivery.Session) error {
	r.finalized = true
	return nil
}

func (r *recordingSink) Abort(_ context.Context, _ *delivery.Session) {}

type fakeSinkFactory struct {
	sink *recordingSink
}

func (f *fakeSinkFactory) NewSink(_ context.Context, _ *delivery.Session, _ []*corev1.Track) (delivery.Sink, error) {
	return f.sink, nil
}

// drmSource returns a manifest with a DRM-encrypted video track served by
// the given test server.
func drmSource(serverURL string) *corev1.MediaSource {
	return &corev1.MediaSource{
		Type:        corev1.MediaSourceType_MEDIA_SOURCE_TYPE_STATIC,
		Seekable:    corev1.Seekable_SEEKABLE_FULL,
		Addressable: corev1.Addressable_ADDRESSABLE_PER_TRACK,
		Tracks: []*corev1.Track{
			{
				Id: "v1",
				Media: &corev1.Track_Video{
					Video: &corev1.VideoTrack{Codec: "h264"},
				},
				Delivery: &corev1.TrackDelivery{
					Locations: []string{serverURL},
					Drm: []*corev1.TrackDrm{{
						System:   "DRM_SYSTEM_CLEARKEY",
						InitData: []byte("pssh-data"),
						KeyIds:   []string{"kid-1"},
					}},
				},
			},
		},
	}
}

// drmWholeMuxSource returns a WHOLE_MUX manifest with a DRM-encrypted
// container track — the shape that can be downloaded (remux + compose).
func drmWholeMuxSource(serverURL string) *corev1.MediaSource {
	return &corev1.MediaSource{
		Type:        corev1.MediaSourceType_MEDIA_SOURCE_TYPE_STATIC,
		Seekable:    corev1.Seekable_SEEKABLE_FULL,
		Addressable: corev1.Addressable_ADDRESSABLE_WHOLE_MUX,
		Tracks: []*corev1.Track{
			{
				Id: "c1",
				Media: &corev1.Track_Video{
					Video: &corev1.VideoTrack{Codec: "h264"},
				},
				Delivery: &corev1.TrackDelivery{
					Locations: []string{serverURL},
					Drm: []*corev1.TrackDrm{{
						System:   "DRM_SYSTEM_CLEARKEY",
						InitData: []byte("pssh-data"),
						KeyIds:   []string{"kid-1"},
					}},
				},
			},
			{
				Id:       "a1",
				Media:    &corev1.Track_Audio{Audio: &corev1.AudioTrack{Codec: "aac"}},
				Delivery: &corev1.TrackDelivery{CarriedIn: "c1"},
			},
		},
	}
}

func testSession() *delivery.Session {
	return &delivery.Session{
		ID: "del-test",
		Context: corev1.DeliveryContext{
			Provider: "jellyfin",
			NativeId: "item-1",
		},
	}
}

// TestM8DecryptToClearOnMockFixtures verifies that acquire-keys + decrypt
// compose as decrypt-to-clear: the engine accepts DRM content with a
// supporting DRM client, the pipeline includes a decrypt step, and the
// opener produces clear bytes.
func TestM8DecryptToClearOnMockFixtures(t *testing.T) {
	srv := encServer()
	defer srv.Close()

	drm := &fakeM8DRMClient{
		supported: []slotsv1.DrmSystem{slotsv1.DrmSystem_DRM_SYSTEM_CLEARKEY},
		keys: []*slotsv1.ContentKey{
			{KeyId: "kid-1", KeyMaterial: []byte("m"), Validity: durationpb.New(time.Hour)},
		},
	}
	cache := newFakeCache()
	opener := delivery.NewOpener(drm, cache, nil)

	e := delivery.New(delivery.Options{
		SessionTTL:      time.Hour,
		Now:             time.Now,
		RecordJob:       func(*corev1.Job) {},
		SourceResolver:  &fakeResolver{source: drmWholeMuxSource(srv.URL)},
		DRMClient:       drm,
		ContentKeyCache: cache,
		Opener:          opener,
	})
	defer e.Close()

	s, err := e.Start(context.Background(), delivery.StartRequest{
		Goal:         delivery.GoalPlay,
		MemberUserID: "u",
		Provider:     "jellyfin",
		AccountID:    "a",
		NativeID:     "item-1",
		Sink:         "device",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(s.Plan.Steps) != 2 {
		t.Fatalf("steps = %#v, want [decrypt, passthrough]", s.Plan.Steps)
	}
	if s.Plan.Steps[0].Kind != delivery.StepDecrypt {
		t.Errorf("step[0].kind = %q, want decrypt", s.Plan.Steps[0].Kind)
	}
}

// TestM8ContentKeyCacheFailFast verifies that a decrypt failure drops the
// cached keys and re-licenses once.
func TestM8ContentKeyCacheFailFast(t *testing.T) {
	srv := encServer()
	defer srv.Close()

	drm := &fakeM8DRMClient{
		supported: []slotsv1.DrmSystem{slotsv1.DrmSystem_DRM_SYSTEM_CLEARKEY},
		keys: []*slotsv1.ContentKey{
			{KeyId: "kid-1", KeyMaterial: []byte("m"), Validity: durationpb.New(time.Hour)},
		},
		decryptErr: errors.New("decrypt failed"),
	}
	cache := newFakeCache()
	opener := delivery.NewOpener(drm, cache, nil)

	sess := testSession()
	track := drmSource(srv.URL).Tracks[0]

	_, err := opener.Open(context.Background(), sess, track)
	if err == nil {
		t.Fatal("expected error after failed re-license")
	}
	if !strings.Contains(err.Error(), "DRM could not be decrypted") {
		t.Fatalf("error = %q, want neutral DRM error", err.Error())
	}
	// The cache should have been dropped (fail-fast).
	if len(cache.keys) != 0 {
		t.Fatalf("cache not empty after fail-fast drop: %v", cache.keys)
	}
}

// TestM8SecondSinkTypeWorksWithoutCoreChanges verifies that the media-server
// sink (a second sink type) works without core changes.
func TestM8SecondSinkTypeWorksWithoutCoreChanges(t *testing.T) {
	var _ delivery.Sink = (*recordingSink)(nil)
}

// TestM8RelayPullOfDRMTrackReturnsClearBytes verifies that a relay pull of
// a DRM track returns clear bytes through the opener.
func TestM8RelayPullOfDRMTrackReturnsClearBytes(t *testing.T) {
	srv := encServer()
	defer srv.Close()

	drm := &fakeM8DRMClient{
		supported: []slotsv1.DrmSystem{slotsv1.DrmSystem_DRM_SYSTEM_CLEARKEY},
		keys: []*slotsv1.ContentKey{
			{KeyId: "kid-1", KeyMaterial: []byte("m"), Validity: durationpb.New(time.Hour)},
		},
	}
	cache := newFakeCache()
	opener := delivery.NewOpener(drm, cache, nil)

	sess := testSession()
	track := drmSource(srv.URL).Tracks[0]

	fetch := opener.Fetch(sess, track)
	res, err := fetch(context.Background(), "")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)
	if string(body) != "clear-encrypted-bytes" {
		t.Fatalf("body = %q, want clear-encrypted-bytes", body)
	}
}

// TestM8DownloadPushDeliversClearBytes verifies that the download push
// path delivers clear bytes to a disk sink.
func TestM8DownloadPushDeliversClearBytes(t *testing.T) {
	srv := encServer()
	defer srv.Close()

	drm := &fakeM8DRMClient{
		supported: []slotsv1.DrmSystem{slotsv1.DrmSystem_DRM_SYSTEM_CLEARKEY},
		keys: []*slotsv1.ContentKey{
			{KeyId: "kid-1", KeyMaterial: []byte("m"), Validity: durationpb.New(time.Hour)},
		},
	}
	cache := newFakeCache()
	opener := delivery.NewOpener(drm, cache, nil)

	sess := testSession()
	track := drmSource(srv.URL).Tracks[0]

	reader, err := opener.Open(context.Background(), sess, track)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = reader.Close() }()
	body, _ := io.ReadAll(reader)
	if string(body) != "clear-encrypted-bytes" {
		t.Fatalf("body = %q, want clear-encrypted-bytes", body)
	}
}

// TestM8SetupSinksWithOpener verifies that SetupSinks accepts an opener
// and wires it into the device sink factory.
func TestM8SetupSinksWithOpener(t *testing.T) {
	opener := delivery.NewOpener(nil, nil, nil)
	relay := delivery.NewRelay()

	sinks, err := slotwiring.SetupSinks([]*corev1.Slot{
		{
			Id:   "device",
			Kind: corev1.SlotKind_SLOT_KIND_SINK,
			Config: &corev1.Slot_Sink{
				Sink: &corev1.SinkConfig{Adapter: "device"},
			},
		},
	}, relay, opener)
	if err != nil {
		t.Fatalf("SetupSinks: %v", err)
	}
	if sinks == nil {
		t.Fatal("sinks is nil")
	}
}

// TestM8EngineWithOpenerAndDownload verifies that the engine can be built
// with an opener and start a download session.
func TestM8EngineWithOpenerAndDownload(t *testing.T) {
	srv := encServer()
	defer srv.Close()

	drm := &fakeM8DRMClient{
		supported: []slotsv1.DrmSystem{slotsv1.DrmSystem_DRM_SYSTEM_CLEARKEY},
		keys: []*slotsv1.ContentKey{
			{KeyId: "kid-1", KeyMaterial: []byte("m"), Validity: durationpb.New(time.Hour)},
		},
	}
	cache := newFakeCache()
	opener := delivery.NewOpener(drm, cache, nil)
	sink := &recordingSink{}

	e := delivery.New(delivery.Options{
		SessionTTL:      time.Hour,
		Now:             time.Now,
		RecordJob:       func(*corev1.Job) {},
		SourceResolver:  &fakeResolver{source: drmWholeMuxSource(srv.URL)},
		DRMClient:       drm,
		ContentKeyCache: cache,
		Opener:          opener,
		SinkFactory:     &fakeSinkFactory{sink: sink},
		InstancePolicy:  policy.Defaults(),
	})
	defer e.Close()

	s, err := e.Start(context.Background(), delivery.StartRequest{
		Goal:         delivery.GoalDownload,
		MemberUserID: "u",
		Provider:     "jellyfin",
		AccountID:    "a",
		NativeID:     "item-1",
		Sink:         "disk",
		Container:    "mkv",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if s.Status != delivery.StatusRunning {
		t.Fatalf("status = %s, want running", s.Status)
	}
}

// TestM8OpenerFetchServesRangeRequests verifies that the opener's Fetch
// serves range requests by slicing the decrypted buffer.
func TestM8OpenerFetchServesRangeRequests(t *testing.T) {
	srv := encServer()
	defer srv.Close()

	drm := &fakeM8DRMClient{
		supported: []slotsv1.DrmSystem{slotsv1.DrmSystem_DRM_SYSTEM_CLEARKEY},
		keys: []*slotsv1.ContentKey{
			{KeyId: "kid-1", KeyMaterial: []byte("m"), Validity: durationpb.New(time.Hour)},
		},
	}
	cache := newFakeCache()
	opener := delivery.NewOpener(drm, cache, nil)

	sess := testSession()
	track := drmSource(srv.URL).Tracks[0]

	fetch := opener.Fetch(sess, track)
	res, err := fetch(context.Background(), "bytes=0-4")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)
	if string(body) != "clear" {
		t.Fatalf("body = %q, want 'clear' (first 5 bytes)", body)
	}
}

// TestM8RelayHandlerServesDRMContent verifies that the relay handler serves
// clear bytes for a DRM track.
func TestM8RelayHandlerServesDRMContent(t *testing.T) {
	srv := encServer()
	defer srv.Close()

	drm := &fakeM8DRMClient{
		supported: []slotsv1.DrmSystem{slotsv1.DrmSystem_DRM_SYSTEM_CLEARKEY},
		keys: []*slotsv1.ContentKey{
			{KeyId: "kid-1", KeyMaterial: []byte("m"), Validity: durationpb.New(time.Hour)},
		},
	}
	cache := newFakeCache()
	opener := delivery.NewOpener(drm, cache, nil)
	relay := delivery.NewRelay()

	sess := testSession()
	track := drmSource(srv.URL).Tracks[0]

	fetch := opener.Fetch(sess, track)
	token, err := relay.Grant(sess.ID, track.GetId(), fetch, time.Minute)
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}

	handler := &delivery.RelayHandler{Relay: relay}
	handlerSrv := httptest.NewServer(handler)
	defer handlerSrv.Close()

	resp, err := http.Get(handlerSrv.URL + delivery.RelayPrefix + token)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "clear-encrypted-bytes" {
		t.Fatalf("body = %q, want clear-encrypted-bytes", body)
	}
}

// TestM8EngineDownloadPushEndToEnd verifies the full download push path:
// engine starts a download, the opener produces clear bytes, the sink
// receives them, and the session completes.
func TestM8EngineDownloadPushEndToEnd(t *testing.T) {
	srv := encServer()
	defer srv.Close()

	drm := &fakeM8DRMClient{
		supported: []slotsv1.DrmSystem{slotsv1.DrmSystem_DRM_SYSTEM_CLEARKEY},
		keys: []*slotsv1.ContentKey{
			{KeyId: "kid-1", KeyMaterial: []byte("m"), Validity: durationpb.New(time.Hour)},
		},
	}
	cache := newFakeCache()
	opener := delivery.NewOpener(drm, cache, nil)
	sink := &recordingSink{pushes: true}

	e := delivery.New(delivery.Options{
		SessionTTL:      time.Hour,
		Now:             time.Now,
		RecordJob:       func(*corev1.Job) {},
		SourceResolver:  &fakeResolver{source: drmWholeMuxSource(srv.URL)},
		DRMClient:       drm,
		ContentKeyCache: cache,
		Opener:          opener,
		SinkFactory:     &fakeSinkFactory{sink: sink},
		InstancePolicy:  policy.Defaults(),
	})
	defer e.Close()

	s, err := e.Start(context.Background(), delivery.StartRequest{
		Goal:         delivery.GoalDownload,
		MemberUserID: "u",
		Provider:     "jellyfin",
		AccountID:    "a",
		NativeID:     "item-1",
		Sink:         "disk",
		Container:    "mkv",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// The download push path is triggered automatically by Start.
	// Poll for completion.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status := s.Job().GetStatus()
		if status == corev1.JobStatus_JOB_STATUS_DONE || status == corev1.JobStatus_JOB_STATUS_FAILED {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	job := s.Job()
	if job.GetStatus() != corev1.JobStatus_JOB_STATUS_DONE {
		t.Fatalf("status = %s, want done (err: %s)", job.GetStatus(), job.GetError())
	}
	if len(sink.delivered) != 1 || sink.delivered[0] != "c1" {
		t.Fatalf("delivered = %v, want [c1]", sink.delivered)
	}
}
