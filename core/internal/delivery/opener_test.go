package delivery

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
)

// fakeCache is a mock ContentKeyCache for testing the opener.
type fakeCache struct {
	keys      map[string][]*slotsv1.ContentKey
	getErr    error
	putErr    error
	dropErr   error
	getCalls  int
	putCalls  int
	dropCalls int
}

func newFakeCache() *fakeCache {
	return &fakeCache{keys: map[string][]*slotsv1.ContentKey{}}
}

func (f *fakeCache) key(provider, contentID string, system slotsv1.DrmSystem) string {
	return provider + "/" + contentID + "/" + system.String()
}

func (f *fakeCache) GetKeys(_ context.Context, provider, contentID string, system slotsv1.DrmSystem) ([]*slotsv1.ContentKey, error) {
	f.getCalls++
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.keys[f.key(provider, contentID, system)], nil
}

func (f *fakeCache) PutKeys(_ context.Context, provider, contentID string, system slotsv1.DrmSystem, keys []*slotsv1.ContentKey) error {
	f.putCalls++
	if f.putErr != nil {
		return f.putErr
	}
	f.keys[f.key(provider, contentID, system)] = keys
	return nil
}

func (f *fakeCache) DropKeys(_ context.Context, provider, contentID string, system slotsv1.DrmSystem) error {
	f.dropCalls++
	if f.dropErr != nil {
		return f.dropErr
	}
	delete(f.keys, f.key(provider, contentID, system))
	return nil
}

// fakeOpenerDRM is a mock DRMClient for testing the opener.
type fakeOpenerDRM struct {
	supported    []slotsv1.DrmSystem
	keys         []*slotsv1.ContentKey
	acquireErr   error
	decryptErr   error
	decryptCalls int
}

func (f *fakeOpenerDRM) Supports(system slotsv1.DrmSystem) bool {
	for _, s := range f.supported {
		if s == system {
			return true
		}
	}
	return false
}

func (f *fakeOpenerDRM) AcquireKeys(_ context.Context, _, _ string, _ slotsv1.DrmSystem, _ []byte, _ []string, _, _ string) ([]*slotsv1.ContentKey, error) {
	if f.acquireErr != nil {
		return nil, f.acquireErr
	}
	return f.keys, nil
}

func (f *fakeOpenerDRM) Decrypt(_ context.Context, source drm.Source, _ []*slotsv1.ContentKey) ([]byte, error) {
	f.decryptCalls++
	if f.decryptErr != nil {
		return nil, f.decryptErr
	}
	return []byte("clear-bytes"), nil
}

func drmTrack(url string) *corev1.Track {
	return &corev1.Track{
		Id: "v1",
		Media: &corev1.Track_Video{
			Video: &corev1.VideoTrack{Codec: "h264"},
		},
		Delivery: &corev1.TrackDelivery{
			Locations: []string{url},
			Drm: []*corev1.TrackDrm{{
				System:   "DRM_SYSTEM_CLEARKEY",
				InitData: []byte("pssh-data"),
				KeyIds:   []string{"kid-1"},
			}},
		},
	}
}

func plainTrack() *corev1.Track {
	return &corev1.Track{
		Id: "v1",
		Media: &corev1.Track_Video{
			Video: &corev1.VideoTrack{Codec: "h264"},
		},
		Delivery: &corev1.TrackDelivery{
			Locations: []string{"https://cdn.example/clear.bin"},
		},
	}
}

func testSession() *Session {
	return &Session{
		ID: "del-1",
		Context: corev1.DeliveryContext{
			Provider: "jellyfin",
			NativeId: "item-1",
		},
	}
}

func TestOpenerCacheHitSkipsAcquire(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("encrypted-bytes"))
	}))
	defer srv.Close()

	drm := &fakeOpenerDRM{
		supported: []slotsv1.DrmSystem{slotsv1.DrmSystem_DRM_SYSTEM_CLEARKEY},
		keys: []*slotsv1.ContentKey{
			{KeyId: "kid-1", KeyMaterial: []byte("m"), Validity: durationpb.New(time.Hour)},
		},
	}
	cache := newFakeCache()
	cache.keys["jellyfin/item-1/DRM_SYSTEM_CLEARKEY"] = drm.keys

	o := NewOpener(drm, cache, nil)
	sess := testSession()
	track := drmTrack(srv.URL)

	res, err := o.Fetch(sess, track)(context.Background(), "")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	body, _ := io.ReadAll(res.Body)
	if string(body) != "clear-bytes" {
		t.Fatalf("body = %q, want clear-bytes", body)
	}
	if cache.getCalls != 1 {
		t.Errorf("cache getCalls = %d, want 1", cache.getCalls)
	}
	if cache.putCalls != 0 {
		t.Errorf("cache putCalls = %d, want 0 (cache hit)", cache.putCalls)
	}
}

func TestOpenerCacheMissAcquiresAndCaches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("encrypted-bytes"))
	}))
	defer srv.Close()

	drm := &fakeOpenerDRM{
		supported: []slotsv1.DrmSystem{slotsv1.DrmSystem_DRM_SYSTEM_CLEARKEY},
		keys: []*slotsv1.ContentKey{
			{KeyId: "kid-1", KeyMaterial: []byte("m"), Validity: durationpb.New(time.Hour)},
		},
	}
	cache := newFakeCache()

	o := NewOpener(drm, cache, nil)
	sess := testSession()
	track := drmTrack(srv.URL)

	_, err := o.Fetch(sess, track)(context.Background(), "")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if cache.getCalls != 1 {
		t.Errorf("cache getCalls = %d, want 1", cache.getCalls)
	}
	if cache.putCalls != 1 {
		t.Errorf("cache putCalls = %d, want 1 (acquired and cached)", cache.putCalls)
	}
}

func TestOpenerDecryptFailureDropsAndRelicenses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("encrypted-bytes"))
	}))
	defer srv.Close()

	drm := &fakeOpenerDRM{
		supported: []slotsv1.DrmSystem{slotsv1.DrmSystem_DRM_SYSTEM_CLEARKEY},
		keys: []*slotsv1.ContentKey{
			{KeyId: "kid-1", KeyMaterial: []byte("m"), Validity: durationpb.New(time.Hour)},
		},
		decryptErr: errors.New("decrypt failed"),
	}
	cache := newFakeCache()
	// Pre-populate cache so the first decrypt uses cached keys.
	cache.keys["jellyfin/item-1/DRM_SYSTEM_CLEARKEY"] = drm.keys

	o := NewOpener(drm, cache, nil)
	sess := testSession()
	track := drmTrack(srv.URL)

	_, err := o.Fetch(sess, track)(context.Background(), "")
	if err == nil {
		t.Fatal("expected error after failed re-license")
	}
	if !strings.Contains(err.Error(), "DRM could not be decrypted") {
		t.Fatalf("error = %q, want neutral DRM error", err.Error())
	}
	if cache.dropCalls != 2 {
		t.Errorf("cache dropCalls = %d, want 2 (initial drop + post-relicense drop)", cache.dropCalls)
	}
	if drm.decryptCalls != 2 {
		t.Errorf("decryptCalls = %d, want 2 (initial + re-license)", drm.decryptCalls)
	}
}

func TestOpenerAcquireFailureReturnsNeutralError(t *testing.T) {
	drm := &fakeOpenerDRM{
		supported:  []slotsv1.DrmSystem{slotsv1.DrmSystem_DRM_SYSTEM_CLEARKEY},
		acquireErr: errors.New("license server unavailable"),
	}
	cache := newFakeCache()

	o := NewOpener(drm, cache, nil)
	sess := testSession()
	track := drmTrack("DRM_SYSTEM_CLEARKEY")

	_, err := o.Fetch(sess, track)(context.Background(), "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "DRM could not be decrypted") {
		t.Fatalf("error = %q, want neutral DRM error", err.Error())
	}
}

func TestOpenerNoDRMPassthrough(t *testing.T) {
	drm := &fakeOpenerDRM{}
	cache := newFakeCache()

	o := NewOpener(drm, cache, nil)
	sess := testSession()
	track := plainTrack()

	// Non-DRM track: the opener fetches directly (no cache, no decrypt).
	// We can't actually fetch from cdn.example, so we test that the
	// non-DRM path doesn't touch the cache or DRM client.
	res, err := o.Fetch(sess, track)(context.Background(), "")
	// The fetch will fail (no real server), but the error should be a
	// fetch error, not a DRM error.
	if err != nil && strings.Contains(err.Error(), "DRM") {
		t.Fatalf("non-DRM track should not produce DRM error, got: %v", err)
	}
	_ = res
	if cache.getCalls != 0 {
		t.Errorf("cache getCalls = %d, want 0 for non-DRM", cache.getCalls)
	}
}

func TestOpenerDRMNoSupportedSystem(t *testing.T) {
	drm := &fakeOpenerDRM{
		supported: []slotsv1.DrmSystem{slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE},
	}
	cache := newFakeCache()

	o := NewOpener(drm, cache, nil)
	sess := testSession()
	track := drmTrack("DRM_SYSTEM_CLEARKEY") // not supported by fakeDRM

	_, err := o.Fetch(sess, track)(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for unsupported DRM system")
	}
	if !strings.Contains(err.Error(), "no supported system") {
		t.Fatalf("error = %q, want 'no supported system'", err.Error())
	}
}

func TestOpenerOpenReturnsReader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("encrypted-bytes"))
	}))
	defer srv.Close()

	drm := &fakeOpenerDRM{
		supported: []slotsv1.DrmSystem{slotsv1.DrmSystem_DRM_SYSTEM_CLEARKEY},
		keys: []*slotsv1.ContentKey{
			{KeyId: "kid-1", KeyMaterial: []byte("m"), Validity: durationpb.New(time.Hour)},
		},
	}
	cache := newFakeCache()

	o := NewOpener(drm, cache, nil)
	sess := testSession()
	track := drmTrack(srv.URL)

	reader, err := o.Open(context.Background(), sess, track)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = reader.Close() }()
	body, _ := io.ReadAll(reader)
	if string(body) != "clear-bytes" {
		t.Fatalf("body = %q, want clear-bytes", body)
	}
}

func TestParseRange(t *testing.T) {
	tests := []struct {
		header string
		n      int
		start  int
		end    int
		ok     bool
	}{
		{"bytes=0-499", 1000, 0, 500, true},
		{"bytes=500-999", 1000, 500, 1000, true},
		{"bytes=500-", 1000, 500, 1000, true},
		{"bytes=-500", 1000, 500, 1000, true},
		{"bytes=-2000", 1000, 0, 1000, true},
		{"bytes=0-0", 1000, 0, 1, true},
		{"bytes=999-999", 1000, 999, 1000, true},
		{"bytes=1000-", 1000, 0, 0, false},
		{"bytes=abc", 1000, 0, 0, false},
		{"", 1000, 0, 0, false},
		{"bytes=0-100,200-300", 1000, 0, 0, false}, // multi-range
	}
	for _, tt := range tests {
		start, end, ok := parseRange(tt.header, tt.n)
		if ok != tt.ok || start != tt.start || end != tt.end {
			t.Errorf("parseRange(%q, %d) = (%d, %d, %v), want (%d, %d, %v)",
				tt.header, tt.n, start, end, ok, tt.start, tt.end, tt.ok)
		}
	}
}
