package m8

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nem-git/abcmovies/core/drm"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/delivery"
	"github.com/nem-git/abcmovies/core/internal/policy"
	"github.com/nem-git/abcmovies/core/internal/slotstore"
	"github.com/nem-git/abcmovies/core/internal/slotwiring"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// TestEngineFlowDRMPull tests the full engine flow for a DRM-encrypted track
// via the pull (relay) path. It always runs: uses a mock DRM client and a
// local HTTP server for the encrypted source.
func TestEngineFlowDRMPull(t *testing.T) {
	encSrv := encServer()
	defer encSrv.Close()

	src := drmWholeMuxSource(encSrv.URL)
	e, drmClient := newEngineWithDRM(t, src, &recordingSinkFactory{})

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

	// Verify the relay pull returns clear bytes.
	track := src.Tracks[0]
	token, relay, err := relayTokenForTrack(t, drmClient, s.ID, track)
	if err != nil {
		t.Fatalf("relay token: %v", err)
	}

	handler := &delivery.RelayHandler{Relay: relay}
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + delivery.RelayPrefix + token)
	if err != nil {
		t.Fatalf("relay GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)

	// The mock DRM client prefixes clear bytes with "clear-".
	if string(body) != "clear-encrypted-bytes" {
		t.Fatalf("relay body = %q, want clear-encrypted-bytes", body)
	}
}

// TestEngineFlowDRMPush tests the full engine flow for a DRM-encrypted track
// via the push (disk) path. It always runs: uses a mock DRM client and a
// local HTTP server for the encrypted source.
func TestEngineFlowDRMPush(t *testing.T) {
	encSrv := encServer()
	defer encSrv.Close()

	src := drmWholeMuxSource(encSrv.URL)
	sink := &recordingSink{pushes: true}
	e, _ := newEngineWithDRM(t, src, &singletonSinkFactory{sink: sink})

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

	// The download push is triggered automatically by Start.
	// Poll for completion.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job := s.Job()
		if job.GetStatus() == corev1.JobStatus_JOB_STATUS_DONE {
			break
		}
		if job.GetStatus() == corev1.JobStatus_JOB_STATUS_FAILED {
			t.Fatalf("download failed: %s", job.GetError())
		}
		time.Sleep(5 * time.Millisecond)
	}

	if len(sink.delivered) != 1 || sink.delivered[0] != "c1" {
		t.Fatalf("delivered = %v, want [c1]", sink.delivered)
	}
}

// TestEngineFlowDirectDownloadDisk tests the full engine flow for a direct
// download start with the real DiskSink. It always runs: uses a mock DRM
// client, a local HTTP server for the encrypted source, and a real DiskSink
// writing to a temp directory. It verifies that DeliverDownload is triggered
// automatically (no manual call) and the file lands on disk.
func TestEngineFlowDirectDownloadDisk(t *testing.T) {
	encSrv := encServer()
	defer encSrv.Close()

	src := drmWholeMuxSource(encSrv.URL)
	tmpDir := t.TempDir()
	diskFac, err := delivery.NewDiskFactory(tmpDir)
	if err != nil {
		t.Fatalf("NewDiskFactory: %v", err)
	}

	e, _ := newEngineWithDRM(t, src, diskFac)

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

	// Poll the job status — DeliverDownload should be triggered automatically
	// by Start, no manual call needed. The .part file exists before delivery
	// completes, so file count is not a completion signal.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job := s.Job()
		if job.GetStatus() == corev1.JobStatus_JOB_STATUS_DONE {
			break
		}
		if job.GetStatus() == corev1.JobStatus_JOB_STATUS_FAILED {
			t.Fatalf("download failed: %s", job.GetError())
		}
		time.Sleep(5 * time.Millisecond)
	}

	if s.Job().GetStatus() != corev1.JobStatus_JOB_STATUS_DONE {
		t.Fatalf("download did not complete: status=%s", s.Job().GetStatus())
	}

	// After finalize, exactly one file with no .part suffix should remain.
	entries, _ := os.ReadDir(tmpDir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 file on disk, got %d", len(entries))
	}
	name := entries[0].Name()
	if strings.HasSuffix(name, ".part") {
		t.Fatalf("file was not finalized: %s", name)
	}

	// The mock DRM client prefixes clear bytes with "clear-".
	data, err := os.ReadFile(tmpDir + "/" + name)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "clear-encrypted-bytes" {
		t.Errorf("content = %q, want clear-encrypted-bytes", data)
	}
}

// TestEngineFlowRealDRM is a real end-to-end test. It skips unless
// ABCMOVIES_WVD_PATH is set and -short is not passed.
func TestEngineFlowRealDRM(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	wvdPath := os.Getenv("ABCMOVIES_WVD_PATH")
	if wvdPath == "" {
		t.Skip("skipping: ABCMOVIES_WVD_PATH not set")
	}

	// Use the real Shaka Widevine test stream as the encrypted source.
	src := drmWholeMuxSource("https://storage.googleapis.com/shaka-demo-assets/angel-one-widevine/dash.mpd")
	e, drmClient := newEngineWithRealDRM(t, wvdPath, src)

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

	// Verify the relay pull returns clear bytes.
	track := src.Tracks[0]
	token, relay, err := relayTokenForTrack(t, drmClient, s.ID, track)
	if err != nil {
		t.Fatalf("relay token: %v", err)
	}

	handler := &delivery.RelayHandler{Relay: relay}
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + delivery.RelayPrefix + token)
	if err != nil {
		t.Fatalf("relay GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)

	if len(body) == 0 {
		t.Fatal("relay body is empty")
	}
	t.Logf("relay returned %d clear bytes", len(body))
}

// newEngineWithDRM builds a delivery engine with a mock DRM client and the
// given sink factory.
func newEngineWithDRM(t *testing.T, src *corev1.MediaSource, sinkFactory delivery.SinkFactory) (*delivery.Engine, drm.Client) {
	t.Helper()

	drmClient := &fakeM8DRMClient{
		supported: []slotsv1.DrmSystem{slotsv1.DrmSystem_DRM_SYSTEM_CLEARKEY},
		keys: []*slotsv1.ContentKey{
			{KeyId: "kid-1", KeyMaterial: []byte("mock-key-material")},
		},
	}

	eng := delivery.New(delivery.Options{
		SessionTTL:      time.Hour,
		Now:             time.Now,
		RecordJob:       func(*corev1.Job) {},
		SourceResolver:  &fakeResolver{source: src},
		DRMClient:       drmClient,
		ContentKeyCache: newFakeCache(),
		Opener:          delivery.NewOpener(drmClient, newFakeCache(), nil),
		SinkFactory:     sinkFactory,
		InstancePolicy:  policy.Defaults(),
	})
	t.Cleanup(eng.Close)
	return eng, drmClient
}

// newEngineWithRealDRM builds a delivery engine with the real local CDM
// adapter wired through SetupDRM.
func newEngineWithRealDRM(t *testing.T, wvdPath string, src *corev1.MediaSource) (*delivery.Engine, drm.Client) {
	t.Helper()

	st := store.NewInMemory()
	ss, err := slotstore.New(st)
	if err != nil {
		t.Fatalf("slotstore.New: %v", err)
	}
	slot := &corev1.Slot{
		Id:   "drm-test",
		Kind: corev1.SlotKind_SLOT_KIND_DRM,
		Config: &corev1.Slot_Drm{Drm: &corev1.DrmConfig{
			Adapter: "local-cdm",
			Options: map[string]string{"wvd_path": wvdPath},
		}},
	}
	if err := ss.Put(context.Background(), slot); err != nil {
		t.Fatalf("slotStore.Put: %v", err)
	}

	drmClient := slotwiring.SetupDRM(context.Background(), []*corev1.Slot{slot}, nil)
	if drmClient == nil {
		t.Fatal("SetupDRM returned nil")
	}

	eng := delivery.New(delivery.Options{
		SessionTTL:      time.Hour,
		Now:             time.Now,
		RecordJob:       func(*corev1.Job) {},
		SourceResolver:  &fakeResolver{source: src},
		DRMClient:       drmClient,
		ContentKeyCache: newFakeCache(),
		Opener:          delivery.NewOpener(drmClient, newFakeCache(), nil),
		SinkFactory:     &recordingSinkFactory{},
		InstancePolicy:  policy.Defaults(),
	})
	t.Cleanup(eng.Close)
	return eng, drmClient
}

// relayTokenForTrack stages a relay token for a track and returns the token
// and the relay it was granted on.
func relayTokenForTrack(t *testing.T, drmClient drm.Client, sessionID string, track *corev1.Track) (string, *delivery.Relay, error) {
	t.Helper()
	sess := &delivery.Session{ID: sessionID}
	opener := delivery.NewOpener(drmClient, newFakeCache(), nil)
	fetch := opener.Fetch(sess, track)
	relay := delivery.NewRelay()
	token, err := relay.Grant(sessionID, track.GetId(), fetch, time.Minute)
	if err != nil {
		return "", nil, err
	}
	return token, relay, nil
}

// recordingSinkFactory creates recording sinks.
type recordingSinkFactory struct{}

func (f *recordingSinkFactory) NewSink(_ context.Context, _ *delivery.Session, _ []*corev1.Track) (delivery.Sink, error) {
	return &recordingSink{}, nil
}

// singletonSinkFactory always returns the same sink.
type singletonSinkFactory struct {
	sink *recordingSink
}

func (f *singletonSinkFactory) NewSink(_ context.Context, _ *delivery.Session, _ []*corev1.Track) (delivery.Sink, error) {
	return f.sink, nil
}
