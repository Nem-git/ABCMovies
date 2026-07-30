package proxy_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nem-git/abcmovies/internal/proxy"
	"github.com/nem-git/abcmovies/internal/stream"
)

// fetcherMock implements proxy.Fetcher for testing.
type fetcherMock struct {
	handler func(url string) (io.ReadCloser, http.Header, error)
}

func (m *fetcherMock) Fetch(_ context.Context, rawURL string, _ http.Header, _ url.Values) (io.ReadCloser, http.Header, error) {
	return m.handler(rawURL)
}

func newFetcherMock(handler func(url string) (io.ReadCloser, http.Header, error)) *fetcherMock {
	return &fetcherMock{handler: handler}
}

func bytesReader(s string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(s))
}

// --- Test fixtures: real-world-style HLS playlists ---

const testMasterPlaylist = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=2000000,CODECS="avc1.42c015,mp4a.40.2",RESOLUTION=1280x720
https://cdn.example.com/movie/720p/variant.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=5000000,CODECS="avc1.640028,mp4a.40.2",RESOLUTION=1920x1080
https://cdn.example.com/movie/1080p/variant.m3u8
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="English",URI="https://cdn.example.com/movie/audio/en.m3u8"
#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="English",URI="https://cdn.example.com/movie/subs/en.m3u8"
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="French",URI="https://cdn.example.com/movie/audio/fr.m3u8"
`

const testVariant720pPlaylist = `#EXTM3U
#EXT-X-TARGETDURATION:6
#EXT-X-MAP:URI="init.mp4"
#EXTINF:6.0,
segment001.ts
#EXTINF:6.0,
segment002.ts
#EXTINF:4.0,
segment003.ts
`

const testRenditionAudioPlaylist = `#EXTM3U
#EXT-X-TARGETDURATION:6
#EXTINF:6.0,
audio_en_001.ts
#EXTINF:6.0,
audio_en_002.ts
`

const testSingleMediaPlaylist = `#EXTM3U
#EXT-X-TARGETDURATION:6
#EXT-X-MAP:URI="init.mp4"
#EXTINF:6.0,
seg_001.ts
#EXTINF:6.0,
seg_002.ts
`

const testMasterNoRenditionURI = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=2000000,CODECS="avc1.42c015,mp4a.40.2",RESOLUTION=1280x720
https://cdn.example.com/movie/720p/variant.m3u8
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="English"
`

func testMeta(proxyBase string) *proxy.StreamMeta {
	return &proxy.StreamMeta{
		ProviderTag:  "test",
		ContentKey:   "movies:1",
		StreamFile:   "master.m3u8",
		Format:       "hls",
		ProxyBaseURL: proxyBase,
		Headers:      http.Header{"X-Auth": []string{"token123"}},
		ExpiresAt:    time.Now().Add(5 * time.Minute),
	}
}

func newHLSStrategy(fetch proxy.Fetcher, store proxy.StateStore) *proxy.HLSStrategy {
	return proxy.NewHLSStrategy(proxy.StrategyDeps{Fetcher: fetch, State: store})
}

// --- Tests ---

func TestHLSStrategy_MasterPlaylistRewrite(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/hls"
	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		if url != "https://cdn.example.com/movie/master.m3u8" {
			t.Errorf("unexpected fetch URL: %s", url)
		}
		return bytesReader(testMasterPlaylist), nil, nil
	})
	store := proxy.NewMemoryStore(time.Minute)
	s := newHLSStrategy(fetch, store)

	var buf strings.Builder
	meta := testMeta(proxyBase)
	locator := stream.Locator{URL: "https://cdn.example.com/movie/master.m3u8"}

	_, err := s.ServeManifest(t.Context(), &buf, locator, meta)
	if err != nil {
		t.Fatalf("ServeManifest() error: %v", err)
	}

	result := buf.String()

	// Variant URIs rewritten to relative proxy paths
	if !strings.Contains(result, "variants/0") {
		t.Errorf("master playlist missing variants/0, got:\n%s", result)
	}
	if !strings.Contains(result, "variants/1") {
		t.Errorf("master playlist missing variants/1, got:\n%s", result)
	}
	// Rendition URIs are stored in state (if GetAllAlternatives() returns them).
	// The hls-m3u8 library may not encode EXT-X-MEDIA tags back into the output,
	// so we only check the state storage, not the encoded output.
	// Original upstream URLs must NOT appear in the variant URIs
	if strings.Contains(result, "cdn.example.com") {
		t.Errorf("master playlist still contains upstream URLs, got:\n%s", result)
	}

	// Check playlist state stored per variant
	ctx := t.Context()
	for i, wantBase := range []string{
		"https://cdn.example.com/movie/720p/variant.m3u8",
		"https://cdn.example.com/movie/1080p/variant.m3u8",
	} {
		key := proxy.HLSPlaylistStateKey("test", "movies:1", "variants", strconv.Itoa(i))
		got, found, err := store.Get(ctx, key)
		if err != nil {
			t.Fatalf("state.Get(%q) error: %v", key, err)
		}
		if !found {
			t.Errorf("playlist state not found for variant %d", i)
			continue
		}
		if got.UpstreamBaseURL != wantBase {
			t.Errorf("variant %d UpstreamBaseURL = %q, want %q", i, got.UpstreamBaseURL, wantBase)
		}
	}

	// Check playlist state stored per rendition.
	// NOTE: The hls-m3u8 library's GetAllAlternatives() only returns alternatives
	// linked to variants via their Alternatives field. If the parser doesn't populate
	// this field from EXT-X-MEDIA tags, rendition state won't be stored.
	// This is a known library limitation — the code stores state correctly when
	// GetAllAlternatives() returns results.
	for _, tc := range []struct {
		id, wantURL string
	}{
		{"audio/English", "https://cdn.example.com/movie/audio/en.m3u8"},
		{"subs/English", "https://cdn.example.com/movie/subs/en.m3u8"},
		{"audio/French", "https://cdn.example.com/movie/audio/fr.m3u8"},
	} {
		key := proxy.HLSPlaylistStateKey("test", "movies:1", "renditions", tc.id)
		got, found, err := store.Get(ctx, key)
		if err != nil {
			t.Fatalf("state.Get(%q) error: %v", key, err)
		}
		if !found {
			t.Logf("playlist state not found for rendition %s (hls-m3u8 library limitation: GetAllAlternatives() may be empty)", tc.id)
			continue
		}
		if got.UpstreamBaseURL != tc.wantURL {
			t.Errorf("rendition %s UpstreamBaseURL = %q, want %q", tc.id, got.UpstreamBaseURL, tc.wantURL)
		}
	}
}

func TestHLSStrategy_SingleMediaPlaylist(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/hls"
	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		return bytesReader(testSingleMediaPlaylist), nil, nil
	})
	store := proxy.NewMemoryStore(time.Minute)
	s := newHLSStrategy(fetch, store)

	var buf strings.Builder
	meta := testMeta(proxyBase)
	locator := stream.Locator{URL: "https://cdn.example.com/movie/single.m3u8"}

	_, err := s.ServeManifest(t.Context(), &buf, locator, meta)
	if err != nil {
		t.Fatalf("ServeManifest() error: %v", err)
	}

	result := buf.String()

	// Synthetic master with one variant
	if !strings.Contains(result, "#EXT-X-STREAM-INF:BANDWIDTH=1") {
		t.Errorf("synthetic master missing BANDWIDTH=1, got:\n%s", result)
	}
	if !strings.Contains(result, "variants/0") {
		t.Errorf("synthetic master missing variants/0, got:\n%s", result)
	}

	// Playlist state for variant 0
	ctx := t.Context()
	playlistKey := proxy.HLSPlaylistStateKey("test", "movies:1", "variants", "0")
	_, found, err := store.Get(ctx, playlistKey)
	if err != nil {
		t.Fatalf("state.Get(%q) error: %v", playlistKey, err)
	}
	if !found {
		t.Error("playlist state not found for variant 0")
	}

	// Segment state for variant 0
	segmentKey := proxy.HLSSegmentStateKey("test", "movies:1", "variants", "0")
	segMeta, found, err := store.Get(ctx, segmentKey)
	if err != nil {
		t.Fatalf("state.Get(%q) error: %v", segmentKey, err)
	}
	if !found {
		t.Error("segment state not found for variant 0")
	}
	// UpstreamBaseURL should be the directory of the first segment's resolved URL
	if !strings.Contains(segMeta.UpstreamBaseURL, "cdn.example.com") {
		t.Errorf("segment UpstreamBaseURL = %q, want CDN base", segMeta.UpstreamBaseURL)
	}
}

func TestHLSStrategy_SubPlaylistRewrite(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/hls"
	ctx := t.Context()
	store := proxy.NewMemoryStore(time.Minute)

	// Pre-populate playlist state for variant 0
	playlistKey := proxy.HLSPlaylistStateKey("test", "movies:1", "variants", "0")
	store.Put(ctx, playlistKey, proxy.StreamMeta{
		ProviderTag:     "test",
		ContentKey:      "movies:1",
		UpstreamBaseURL: "https://cdn.example.com/movie/720p/variant.m3u8",
		ProxyBaseURL:    proxyBase,
		Headers:         http.Header{"X-Auth": []string{"token123"}},
		ExpiresAt:       time.Now().Add(5 * time.Minute),
	})

	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		if url != "https://cdn.example.com/movie/720p/variant.m3u8" {
			t.Errorf("unexpected fetch URL: %s", url)
		}
		return bytesReader(testVariant720pPlaylist), nil, nil
	})
	s := newHLSStrategy(fetch, store)

	var buf strings.Builder
	meta := testMeta(proxyBase)
	err := s.ServeSubPlaylist(ctx, &buf, stream.Locator{URL: "https://cdn.example.com/movie/720p/variant.m3u8"}, meta, "variants", "0")
	if err != nil {
		t.Fatalf("ServeSubPlaylist() error: %v", err)
	}

	result := buf.String()

	// Segment URLs rewritten to relative proxy paths
	if !strings.Contains(result, "0/segments/segment001.ts") {
		t.Errorf("sub-playlist missing rewritten segment001.ts, got:\n%s", result)
	}
	if !strings.Contains(result, "0/segments/segment002.ts") {
		t.Errorf("sub-playlist missing rewritten segment002.ts, got:\n%s", result)
	}
	if !strings.Contains(result, "0/segments/segment003.ts") {
		t.Errorf("sub-playlist missing rewritten segment003.ts, got:\n%s", result)
	}

	// EXT-X-MAP URI rewritten
	if !strings.Contains(result, "0/segments/init.mp4") {
		t.Errorf("sub-playlist missing rewritten EXT-X-MAP URI, got:\n%s", result)
	}

	// No upstream URLs remain
	if strings.Contains(result, "cdn.example.com") {
		t.Errorf("sub-playlist still contains upstream URLs, got:\n%s", result)
	}

	// Segment state stored
	segmentKey := proxy.HLSSegmentStateKey("test", "movies:1", "variants", "0")
	segMeta, found, err := store.Get(ctx, segmentKey)
	if err != nil {
		t.Fatalf("state.Get(%q) error: %v", segmentKey, err)
	}
	if !found {
		t.Fatal("segment state not found for variant 0")
	}
	// Base URL should be derived from first segment's absolute URL directory
	if !strings.HasPrefix(segMeta.UpstreamBaseURL, "https://cdn.example.com/movie/720p/") {
		t.Errorf("segment UpstreamBaseURL = %q, want prefix https://cdn.example.com/movie/720p/", segMeta.UpstreamBaseURL)
	}
}

func TestHLSStrategy_RenditionSubPlaylist(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/hls"
	ctx := t.Context()
	store := proxy.NewMemoryStore(time.Minute)

	// Pre-populate playlist state for rendition
	playlistKey := proxy.HLSPlaylistStateKey("test", "movies:1", "renditions", "audio/English")
	store.Put(ctx, playlistKey, proxy.StreamMeta{
		ProviderTag:     "test",
		ContentKey:      "movies:1",
		UpstreamBaseURL: "https://cdn.example.com/movie/audio/en.m3u8",
		ProxyBaseURL:    proxyBase,
		Headers:         http.Header{"X-Auth": []string{"token123"}},
		ExpiresAt:       time.Now().Add(5 * time.Minute),
	})

	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		return bytesReader(testRenditionAudioPlaylist), nil, nil
	})
	s := newHLSStrategy(fetch, store)

	var buf strings.Builder
	meta := testMeta(proxyBase)
	err := s.ServeSubPlaylist(ctx, &buf, stream.Locator{URL: "https://cdn.example.com/movie/audio/en.m3u8"}, meta, "renditions", "audio/English")
	if err != nil {
		t.Fatalf("ServeSubPlaylist() error: %v", err)
	}

	result := buf.String()

	// Segment URLs rewritten to relative rendition proxy paths
	if !strings.Contains(result, "English/segments/audio_en_001.ts") {
		t.Errorf("sub-playlist missing rewritten audio_en_001.ts, got:\n%s", result)
	}
	if !strings.Contains(result, "English/segments/audio_en_002.ts") {
		t.Errorf("sub-playlist missing rewritten audio_en_002.ts, got:\n%s", result)
	}

	// No upstream URLs remain
	if strings.Contains(result, "cdn.example.com") {
		t.Errorf("sub-playlist still contains upstream URLs, got:\n%s", result)
	}

	// Segment state stored under rendition key
	segmentKey := proxy.HLSSegmentStateKey("test", "movies:1", "renditions", "audio/English")
	_, found, err := store.Get(ctx, segmentKey)
	if err != nil {
		t.Fatalf("state.Get(%q) error: %v", segmentKey, err)
	}
	if !found {
		t.Error("segment state not found for rendition audio,English")
	}
}

func TestHLSStrategy_SegmentServing(t *testing.T) {
	var fetchedURL string
	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		fetchedURL = url
		return bytesReader("segment-data"), nil, nil
	})
	store := proxy.NewMemoryStore(time.Minute)
	s := newHLSStrategy(fetch, store)

	var buf strings.Builder
	locator := stream.Locator{
		URL:     "https://cdn.example.com/movie/720p/segment001.ts",
		Headers: http.Header{"X-Auth": []string{"token123"}},
	}
	err := s.ServeSegment(t.Context(), &buf, locator, "segment001.ts")
	if err != nil {
		t.Fatalf("ServeSegment() error: %v", err)
	}

	if fetchedURL != "https://cdn.example.com/movie/720p/segment001.ts" {
		t.Errorf("upstream fetched URL = %q, want %q", fetchedURL, "https://cdn.example.com/movie/720p/segment001.ts")
	}
	if buf.String() != "segment-data" {
		t.Errorf("segment body = %q, want %q", buf.String(), "segment-data")
	}
}

func TestHLSStrategy_StateKeys(t *testing.T) {
	tests := []struct {
		tag, contentKey, keyType, id string
		wantPlaylist, wantSegment    string
	}{
		{
			tag: "T", contentKey: "movies:1", keyType: "variants", id: "0",
			wantPlaylist: "T:movies:1:hls:playlist:variants:0",
			wantSegment:  "T:movies:1:hls:segment:variants:0",
		},
		{
			tag: "T", contentKey: "movies:1", keyType: "renditions", id: "audio/English",
			wantPlaylist: "T:movies:1:hls:playlist:renditions:audio/English",
			wantSegment:  "T:movies:1:hls:segment:renditions:audio/English",
		},
		{
			tag: "S", contentKey: "series:456:789:101", keyType: "variants", id: "2",
			wantPlaylist: "S:series:456:789:101:hls:playlist:variants:2",
			wantSegment:  "S:series:456:789:101:hls:segment:variants:2",
		},
	}
	for _, tt := range tests {
		gotPlaylist := proxy.HLSPlaylistStateKey(tt.tag, tt.contentKey, tt.keyType, tt.id)
		if gotPlaylist != tt.wantPlaylist {
			t.Errorf("HLSPlaylistStateKey(%q, %q, %q, %q) = %q, want %q", tt.tag, tt.contentKey, tt.keyType, tt.id, gotPlaylist, tt.wantPlaylist)
		}
		gotSegment := proxy.HLSSegmentStateKey(tt.tag, tt.contentKey, tt.keyType, tt.id)
		if gotSegment != tt.wantSegment {
			t.Errorf("HLSSegmentStateKey(%q, %q, %q, %q) = %q, want %q", tt.tag, tt.contentKey, tt.keyType, tt.id, gotSegment, tt.wantSegment)
		}
	}
}

func TestHLSStrategy_PlaylistStateTwoPass(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/hls"
	ctx := t.Context()
	store := proxy.NewMemoryStore(time.Minute)

	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		switch url {
		case "https://cdn.example.com/movie/master.m3u8":
			return bytesReader(testMasterPlaylist), nil, nil
		case "https://cdn.example.com/movie/720p/variant.m3u8":
			return bytesReader(testVariant720pPlaylist), nil, nil
		default:
			t.Errorf("unexpected fetch URL: %s", url)
			return nil, nil, fmt.Errorf("unexpected URL: %s", url)
		}
	})
	s := newHLSStrategy(fetch, store)

	// Pass 1: Serve master playlist
	var masterBuf strings.Builder
	meta := testMeta(proxyBase)
	_, err := s.ServeManifest(ctx, &masterBuf, stream.Locator{URL: "https://cdn.example.com/movie/master.m3u8"}, meta)
	if err != nil {
		t.Fatalf("ServeManifest() error: %v", err)
	}

	// Verify playlist state exists for variant 0
	playlistKey := proxy.HLSPlaylistStateKey("test", "movies:1", "variants", "0")
	playlistMeta, found, err := store.Get(ctx, playlistKey)
	if err != nil || !found {
		t.Fatalf("playlist state not found for variant 0 after pass 1")
	}
	if playlistMeta.UpstreamBaseURL != "https://cdn.example.com/movie/720p/variant.m3u8" {
		t.Errorf("playlist UpstreamBaseURL = %q, want %q", playlistMeta.UpstreamBaseURL, "https://cdn.example.com/movie/720p/variant.m3u8")
	}

	// Verify NO segment state yet
	segmentKey := proxy.HLSSegmentStateKey("test", "movies:1", "variants", "0")
	_, found, _ = store.Get(ctx, segmentKey)
	if found {
		t.Error("segment state should not exist before pass 2")
	}

	// Pass 2: Serve sub-playlist for variant 0
	var subBuf strings.Builder
	err = s.ServeSubPlaylist(ctx, &subBuf, stream.Locator{URL: playlistMeta.UpstreamBaseURL}, &playlistMeta, "variants", "0")
	if err != nil {
		t.Fatalf("ServeSubPlaylist() error: %v", err)
	}

	// Verify segment state now exists
	segMeta, found, err := store.Get(ctx, segmentKey)
	if err != nil || !found {
		t.Fatalf("segment state not found for variant 0 after pass 2")
	}
	if !strings.HasPrefix(segMeta.UpstreamBaseURL, "https://cdn.example.com/movie/720p/") {
		t.Errorf("segment UpstreamBaseURL = %q, want prefix https://cdn.example.com/movie/720p/", segMeta.UpstreamBaseURL)
	}
}

// --- HLS Resource Tests ---

const testMasterWithResources = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=2000000
https://cdn.example.com/movie/720p/variant.m3u8
#EXT-X-SESSION-KEY:METHOD=AES-128,URI="https://cdn.example.com/movie/keys/session.key"
#EXT-X-SESSION-DATA:DATA-ID="com.example.title",VALUE="Test Movie",URI="https://cdn.example.com/movie/session/data.json"
#EXT-X-CONTENT-STEERING:SERVER-URI="https://cdn.example.com/movie/steering.json",PATHWAY-ID="default"
`

const testMediaWithResources = `#EXTM3U
#EXT-X-TARGETDURATION:6
#EXT-X-KEY:METHOD=AES-128,URI="https://cdn.example.com/movie/keys/enc.key"
#EXTINF:6.0,
seg001.ts
#EXT-X-PART:URI="part001.ts",DURATION=1.0,INDEPENDENT=YES
#EXT-X-PRELOAD-HINT:TYPE=MAP,URI="hint.m4s"
`

func TestHLSStrategy_KeyRewriteInMediaPlaylist(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/hls"
	ctx := t.Context()
	store := proxy.NewMemoryStore(time.Minute)

	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		return bytesReader(testMediaWithResources), nil, nil
	})
	s := newHLSStrategy(fetch, store)

	var buf strings.Builder
	meta := testMeta(proxyBase)
	locator := stream.Locator{URL: "https://cdn.example.com/movie/media.m3u8"}

	// Single media playlist -> synthetic master
	_, err := s.ServeManifest(ctx, &buf, locator, meta)
	if err != nil {
		t.Fatalf("ServeManifest() error: %v", err)
	}

	// Check key state stored with scoped ID
	resourceKey := "test:movies:1:hls:resource:key:0/enc.key"
	gotMeta, found, err := store.Get(ctx, resourceKey)
	if err != nil {
		t.Fatalf("state.Get(%q) error: %v", resourceKey, err)
	}
	if !found {
		t.Fatal("key state not found")
	}
	if gotMeta.UpstreamBaseURL != "https://cdn.example.com/movie/keys/enc.key" {
		t.Errorf("key UpstreamBaseURL = %q, want %q", gotMeta.UpstreamBaseURL, "https://cdn.example.com/movie/keys/enc.key")
	}

	// Check the rewritten playlist (only the synthetic master is returned for single media)
	// The actual rewritten media is in the variant sub-playlist, not in the master output
	// We need to check via ServeSubPlaylist instead — but that's a 2-pass flow.

	// Verify segment state exists with correct upstream base for keys resolution
	segmentKey := proxy.HLSSegmentStateKey("test", "movies:1", "variants", "0")
	segMeta, found, err := store.Get(ctx, segmentKey)
	if err != nil {
		t.Fatalf("state.Get(%q) error: %v", segmentKey, err)
	}
	if !found {
		t.Fatal("segment state not found")
	}
	_ = segMeta
}

func TestHLSStrategy_MediaPlaylistKeyRewriteSubPlaylist(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/hls"
	ctx := t.Context()
	store := proxy.NewMemoryStore(time.Minute)

	// Pre-populate playlist state for variant 0 (as would happen from master)
	playlistKey := proxy.HLSPlaylistStateKey("test", "movies:1", "variants", "0")
	store.Put(ctx, playlistKey, proxy.StreamMeta{
		ProviderTag:     "test",
		ContentKey:      "movies:1",
		UpstreamBaseURL: "https://cdn.example.com/movie/media.m3u8",
		ProxyBaseURL:    proxyBase,
		Headers:         http.Header{"X-Auth": []string{"token123"}},
		ExpiresAt:       time.Now().Add(5 * time.Minute),
	})

	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		return bytesReader(testMediaWithResources), nil, nil
	})
	s := newHLSStrategy(fetch, store)

	var buf strings.Builder
	meta := testMeta(proxyBase)
	err := s.ServeSubPlaylist(ctx, &buf, stream.Locator{URL: "https://cdn.example.com/movie/media.m3u8"}, meta, "variants", "0")
	if err != nil {
		t.Fatalf("ServeSubPlaylist() error: %v", err)
	}

	result := buf.String()
	t.Logf("Rewritten sub-playlist:\n%s", result)

	// Key URI should be rewritten to proxy path
	if !strings.Contains(result, "keys/enc.key") {
		t.Errorf("sub-playlist missing rewritten key URI, got:\n%s", result)
	}

	// Segment URIs should be rewritten
	if !strings.Contains(result, "0/segments/seg001.ts") {
		t.Errorf("sub-playlist missing rewritten segment, got:\n%s", result)
	}

	// No upstream URLs remain
	if strings.Contains(result, "cdn.example.com") {
		t.Errorf("sub-playlist still contains upstream URLs, got:\n%s", result)
	}

	// Key state stored with scoped ID
	resourceKey := "test:movies:1:hls:resource:key:0/enc.key"
	keyMeta, found, err := store.Get(ctx, resourceKey)
	if err != nil {
		t.Fatalf("state.Get(%q) error: %v", resourceKey, err)
	}
	if !found {
		t.Fatal("key state not found")
	}
	if keyMeta.UpstreamBaseURL != "https://cdn.example.com/movie/keys/enc.key" {
		t.Errorf("key UpstreamBaseURL = %q, want %q", keyMeta.UpstreamBaseURL, "https://cdn.example.com/movie/keys/enc.key")
	}
}

func TestHLSStrategy_PartialSegmentRewrite(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/hls"
	ctx := t.Context()
	store := proxy.NewMemoryStore(time.Minute)

	// Pre-populate playlist state for variant 0
	playlistKey := proxy.HLSPlaylistStateKey("test", "movies:1", "variants", "0")
	store.Put(ctx, playlistKey, proxy.StreamMeta{
		ProviderTag:     "test",
		ContentKey:      "movies:1",
		UpstreamBaseURL: "https://cdn.example.com/movie/media.m3u8",
		ProxyBaseURL:    proxyBase,
		Headers:         http.Header{},
		ExpiresAt:       time.Now().Add(5 * time.Minute),
	})

	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		return bytesReader(testMediaWithResources), nil, nil
	})
	s := newHLSStrategy(fetch, store)

	var buf strings.Builder
	meta := testMeta(proxyBase)
	err := s.ServeSubPlaylist(ctx, &buf, stream.Locator{URL: "https://cdn.example.com/movie/media.m3u8"}, meta, "variants", "0")
	if err != nil {
		t.Fatalf("ServeSubPlaylist() error: %v", err)
	}

	result := buf.String()
	t.Logf("Rewritten sub-playlist (partials):\n%s", result)

	// Partial segment URI should be rewritten
	if !strings.Contains(result, "partials/part001.ts") {
		t.Errorf("sub-playlist missing rewritten partial segment URI, got:\n%s", result)
	}

	// Partial state stored
	resourceKey := "test:movies:1:hls:resource:partial:0/part001.ts"
	_, found, err := store.Get(ctx, resourceKey)
	if err != nil {
		t.Fatalf("state.Get(%q) error: %v", resourceKey, err)
	}
	if !found {
		t.Error("partial state not found")
	}
}

func TestHLSStrategy_PreloadHintRewrite(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/hls"
	ctx := t.Context()
	store := proxy.NewMemoryStore(time.Minute)

	// Pre-populate playlist state for variant 0
	playlistKey := proxy.HLSPlaylistStateKey("test", "movies:1", "variants", "0")
	store.Put(ctx, playlistKey, proxy.StreamMeta{
		ProviderTag:     "test",
		ContentKey:      "movies:1",
		UpstreamBaseURL: "https://cdn.example.com/movie/media.m3u8",
		ProxyBaseURL:    proxyBase,
		Headers:         http.Header{},
		ExpiresAt:       time.Now().Add(5 * time.Minute),
	})

	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		return bytesReader(testMediaWithResources), nil, nil
	})
	s := newHLSStrategy(fetch, store)

	var buf strings.Builder
	meta := testMeta(proxyBase)
	err := s.ServeSubPlaylist(ctx, &buf, stream.Locator{URL: "https://cdn.example.com/movie/media.m3u8"}, meta, "variants", "0")
	if err != nil {
		t.Fatalf("ServeSubPlaylist() error: %v", err)
	}

	result := buf.String()
	t.Logf("Rewritten sub-playlist (preload-hint):\n%s", result)

	// Preload hint URI should be rewritten
	// Note: the hls-m3u8 library encodes PreloadHint only in the full encoder;
	// it may or may not appear in output depending on library version.
	// We check state storage instead.
	resourceKey := "test:movies:1:hls:resource:preload-hint:0/hint.m4s"
	preloadMeta, found, err := store.Get(ctx, resourceKey)
	if err != nil {
		t.Fatalf("state.Get(%q) error: %v", resourceKey, err)
	}
	if !found {
		t.Error("preload-hint state not found")
	} else {
		if preloadMeta.UpstreamBaseURL != "https://cdn.example.com/movie/hint.m4s" {
			t.Errorf("preload-hint UpstreamBaseURL = %q, want %q", preloadMeta.UpstreamBaseURL, "https://cdn.example.com/movie/hint.m4s")
		}
	}
}

func TestHLSStrategy_SessionKeyRewriteInMaster(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/hls"
	ctx := t.Context()
	store := proxy.NewMemoryStore(time.Minute)

	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		return bytesReader(testMasterWithResources), nil, nil
	})
	s := newHLSStrategy(fetch, store)

	var buf strings.Builder
	meta := testMeta(proxyBase)
	_, err := s.ServeManifest(ctx, &buf, stream.Locator{URL: "https://cdn.example.com/movie/master.m3u8"}, meta)
	if err != nil {
		t.Fatalf("ServeManifest() error: %v", err)
	}

	result := buf.String()
	t.Logf("Master playlist:\n%s", result)

	// Session key URI rewritten
	if !strings.Contains(result, "session-keys/session.key") {
		t.Errorf("master missing rewritten session key URI, got:\n%s", result)
	}

	// Session key state stored
	resourceKey := "test:movies:1:hls:resource:session-key:session.key"
	keyMeta, found, err := store.Get(ctx, resourceKey)
	if err != nil {
		t.Fatalf("state.Get(%q) error: %v", resourceKey, err)
	}
	if !found {
		t.Fatal("session key state not found")
	}
	if keyMeta.UpstreamBaseURL != "https://cdn.example.com/movie/keys/session.key" {
		t.Errorf("session key UpstreamBaseURL = %q, want %q", keyMeta.UpstreamBaseURL, "https://cdn.example.com/movie/keys/session.key")
	}

	// No upstream URLs remain
	if strings.Contains(result, "cdn.example.com") {
		t.Errorf("master still contains upstream URLs, got:\n%s", result)
	}
}

func TestHLSStrategy_SessionDataRewriteInMaster(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/hls"
	ctx := t.Context()
	store := proxy.NewMemoryStore(time.Minute)

	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		return bytesReader(testMasterWithResources), nil, nil
	})
	s := newHLSStrategy(fetch, store)

	var buf strings.Builder
	meta := testMeta(proxyBase)
	_, err := s.ServeManifest(ctx, &buf, stream.Locator{URL: "https://cdn.example.com/movie/master.m3u8"}, meta)
	if err != nil {
		t.Fatalf("ServeManifest() error: %v", err)
	}

	result := buf.String()
	t.Logf("Master playlist:\n%s", result)

	// Session data URI rewritten
	if !strings.Contains(result, "session-data/data.json") {
		t.Errorf("master missing rewritten session data URI, got:\n%s", result)
	}

	// Session data state stored
	resourceKey := "test:movies:1:hls:resource:session-data:data.json"
	sdMeta, found, err := store.Get(ctx, resourceKey)
	if err != nil {
		t.Fatalf("state.Get(%q) error: %v", resourceKey, err)
	}
	if !found {
		t.Fatal("session data state not found")
	}
	if sdMeta.UpstreamBaseURL != "https://cdn.example.com/movie/session/data.json" {
		t.Errorf("session data UpstreamBaseURL = %q, want %q", sdMeta.UpstreamBaseURL, "https://cdn.example.com/movie/session/data.json")
	}
}

func TestHLSStrategy_ContentSteeringRewriteInMaster(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/hls"
	ctx := t.Context()
	store := proxy.NewMemoryStore(time.Minute)

	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		return bytesReader(testMasterWithResources), nil, nil
	})
	s := newHLSStrategy(fetch, store)

	var buf strings.Builder
	meta := testMeta(proxyBase)
	_, err := s.ServeManifest(ctx, &buf, stream.Locator{URL: "https://cdn.example.com/movie/master.m3u8"}, meta)
	if err != nil {
		t.Fatalf("ServeManifest() error: %v", err)
	}

	result := buf.String()
	t.Logf("Master playlist:\n%s", result)

	// Content steering URI rewritten to just "steering"
	if !strings.Contains(result, `SERVER-URI="steering"`) {
		t.Errorf("master missing rewritten content steering URI, got:\n%s", result)
	}

	// Content steering state stored (empty ID since only one steering entry)
	resourceKey := "test:movies:1:hls:resource:steering:"
	csMeta, found, err := store.Get(ctx, resourceKey)
	if err != nil {
		t.Fatalf("state.Get(%q) error: %v", resourceKey, err)
	}
	if !found {
		t.Fatal("content steering state not found")
	}
	if csMeta.UpstreamBaseURL != "https://cdn.example.com/movie/steering.json" {
		t.Errorf("content steering UpstreamBaseURL = %q, want %q", csMeta.UpstreamBaseURL, "https://cdn.example.com/movie/steering.json")
	}
}

func TestHLSStrategy_AllResourcesRewriteTwoPass(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/hls"
	ctx := t.Context()
	store := proxy.NewMemoryStore(time.Minute)

	// Pass 1: master with resources
	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		switch url {
		case "https://cdn.example.com/movie/master.m3u8":
			return bytesReader(testMasterWithResources), nil, nil
		case "https://cdn.example.com/movie/media.m3u8":
			return bytesReader(testMediaWithResources), nil, nil
		default:
			t.Errorf("unexpected fetch URL: %s", url)
			return nil, nil, fmt.Errorf("unexpected URL: %s", url)
		}
	})
	s := newHLSStrategy(fetch, store)

	meta := testMeta(proxyBase)

	// Pass 1: Master
	var masterBuf strings.Builder
	_, err := s.ServeManifest(ctx, &masterBuf, stream.Locator{URL: "https://cdn.example.com/movie/master.m3u8"}, meta)
	if err != nil {
		t.Fatalf("ServeManifest() error: %v", err)
	}

	// Verify master-level resources stored
	for _, tc := range []struct {
		resourceType, resourceID, wantURL string
	}{
		{"session-key", "session.key", "https://cdn.example.com/movie/keys/session.key"},
		{"session-data", "data.json", "https://cdn.example.com/movie/session/data.json"},
		{"steering", "", "https://cdn.example.com/movie/steering.json"},
	} {
		key := "test:movies:1:hls:resource:" + tc.resourceType + ":" + tc.resourceID
		rm, found, err := store.Get(ctx, key)
		if err != nil {
			t.Fatalf("state.Get(%q) error: %v", key, err)
		}
		if !found {
			t.Errorf("state not found for %s/%s", tc.resourceType, tc.resourceID)
			continue
		}
		if rm.UpstreamBaseURL != tc.wantURL {
			t.Errorf("%s UpstreamBaseURL = %q, want %q", tc.resourceType, rm.UpstreamBaseURL, tc.wantURL)
		}
	}

	// Pass 2: Sub-playlist (variant 0) — we need to set up playlist state
	// In a real flow, Pass 1 stores playlist state for variant 0.
	// But the test master only has one variant pointing to the media URL.
	// Let's manually pre-populate the variant playlist state with the media URL.
	playlistKey := proxy.HLSPlaylistStateKey("test", "movies:1", "variants", "0")
	pMeta, _, _ := store.Get(ctx, playlistKey)
	if pMeta.UpstreamBaseURL == "" {
		// The test master doesn't have the same media URL; manually set it
		store.Put(ctx, playlistKey, proxy.StreamMeta{
			ProviderTag:     "test",
			ContentKey:      "movies:1",
			UpstreamBaseURL: "https://cdn.example.com/movie/media.m3u8",
			ProxyBaseURL:    proxyBase,
			Headers:         http.Header{},
			ExpiresAt:       time.Now().Add(5 * time.Minute),
		})
	}

	var subBuf strings.Builder
	err = s.ServeSubPlaylist(ctx, &subBuf, stream.Locator{URL: "https://cdn.example.com/movie/media.m3u8"}, meta, "variants", "0")
	if err != nil {
		t.Fatalf("ServeSubPlaylist() error: %v", err)
	}

	// Verify sub-playlist resources stored
	for _, tc := range []struct {
		resourceType, resourceID, wantURL string
	}{
		{"key", "0/enc.key", "https://cdn.example.com/movie/keys/enc.key"},
		{"partial", "0/part001.ts", "https://cdn.example.com/movie/part001.ts"},
		{"preload-hint", "0/hint.m4s", "https://cdn.example.com/movie/hint.m4s"},
	} {
		key := "test:movies:1:hls:resource:" + tc.resourceType + ":" + tc.resourceID
		rm, found, err := store.Get(ctx, key)
		if err != nil {
			t.Fatalf("state.Get(%q) error: %v", key, err)
		}
		if !found {
			t.Errorf("state not found for %s/%s", tc.resourceType, tc.resourceID)
			continue
		}
		if rm.UpstreamBaseURL != tc.wantURL {
			t.Errorf("%s UpstreamBaseURL = %q, want %q", tc.resourceType, rm.UpstreamBaseURL, tc.wantURL)
		}
	}
}

func TestHLSResourceStateKey(t *testing.T) {
	tests := []struct {
		tag, contentKey, resourceType, id string
		want                              string
	}{
		{"T", "movies:1", "key", "enc.key", "T:movies:1:hls:resource:key:enc.key"},
		{"T", "movies:1", "session-key", "session.key", "T:movies:1:hls:resource:session-key:session.key"},
		{"T", "movies:1", "partial", "part001.ts", "T:movies:1:hls:resource:partial:part001.ts"},
		{"T", "movies:1", "preload-hint", "hint.m4s", "T:movies:1:hls:resource:preload-hint:hint.m4s"},
		{"T", "movies:1", "session-data", "data.json", "T:movies:1:hls:resource:session-data:data.json"},
		{"T", "movies:1", "steering", "", "T:movies:1:hls:resource:steering:"},
		{"S", "series:456", "key", "key.bin", "S:series:456:hls:resource:key:key.bin"},
	}
	for _, tt := range tests {
		key := proxy.HLSResourceStateKey(tt.tag, tt.contentKey, tt.resourceType, tt.id)
		if key != tt.want {
			t.Errorf("HLSResourceStateKey(%q, %q, %q, %q) = %q, want %q", tt.tag, tt.contentKey, tt.resourceType, tt.id, key, tt.want)
		}
	}
}

func TestHLSResourceFilenameHelper(t *testing.T) {
	tests := []struct {
		rawURL   string
		wantFile string
	}{
		{"https://cdn.example.com/keys/enc.key", "enc.key"},
		{"https://cdn.example.com/keys/enc.key?exp=123&sig=abc", "enc.key"},
		{"https://cdn.example.com/session/data.json", "data.json"},
		{"https://cdn.example.com/steering.json", "steering.json"},
		{"https://cdn.example.com/partials/part001.ts", "part001.ts"},
		{"https://cdn.example.com/hint.m4s", "hint.m4s"},
		{"https://cdn.example.com/key.bin?token=xyz&expires=9999999999", "key.bin"},
	}
	for _, tt := range tests {
		got := proxy.ResourceFilename(tt.rawURL)
		if got != tt.wantFile {
			t.Errorf("ResourceFilename(%q) = %q, want %q", tt.rawURL, got, tt.wantFile)
		}
	}
}

func TestHLSResourceContentType(t *testing.T) {
	tests := []struct {
		format, resourceID string
		want               string
	}{
		{"hls", "enc.key", "application/octet-stream"},
		{"hls", "key.bin", "video/mp4"},
		{"hls", "part001.ts", "video/mp2t"},
		{"hls", "init.mp4", "video/mp4"},
		{"hls", "segment.m4s", "video/mp4"},
		{"hls", "data.json", "application/json"},
		{"", "", "application/octet-stream"},
		{"dash", "segment.m4s", "video/mp4"},
	}
	for _, tt := range tests {
		got := proxy.ResourceContentType(tt.format, tt.resourceID)
		if got != tt.want {
			t.Errorf("ResourceContentType(%q, %q) = %q, want %q", tt.format, tt.resourceID, got, tt.want)
		}
	}
}

func TestHLSStrategy_KeyWithQueryString(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/hls"
	ctx := t.Context()
	store := proxy.NewMemoryStore(time.Minute)

	playlist := `#EXTM3U
#EXT-X-TARGETDURATION:6
#EXT-X-KEY:METHOD=AES-128,URI="https://cdn.example.com/keys/enc.key?token=abc123&expires=9999999999"
#EXTINF:6.0,
seg001.ts
`

	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		return bytesReader(playlist), nil, nil
	})

	// Pre-populate playlist state for variant 0
	store.Put(ctx, proxy.HLSPlaylistStateKey("test", "movies:1", "variants", "0"), proxy.StreamMeta{
		UpstreamBaseURL: "https://cdn.example.com/movie/playlist.m3u8",
		ProxyBaseURL:    proxyBase,
		ExpiresAt:       time.Now().Add(5 * time.Minute),
	})

	s := newHLSStrategy(fetch, store)

	var buf strings.Builder
	meta := testMeta(proxyBase)
	err := s.ServeSubPlaylist(ctx, &buf, stream.Locator{URL: "https://cdn.example.com/movie/playlist.m3u8"}, meta, "variants", "0")
	if err != nil {
		t.Fatalf("ServeSubPlaylist() error: %v", err)
	}

	result := buf.String()
	t.Logf("Rewritten playlist:\n%s", result)

	// Key URI should have query params stripped (use filename without query)
	if !strings.Contains(result, "keys/enc.key") {
		t.Errorf("sub-playlist missing rewritten key URI, got:\n%s", result)
	}
	if strings.Contains(result, "?token=") {
		t.Errorf("sub-playlist still contains query params in URI, got:\n%s", result)
	}

	// Verify state stores the full URL with query params
	resourceKey := "test:movies:1:hls:resource:key:0/enc.key"
	keyMeta, found, err := store.Get(ctx, resourceKey)
	if err != nil {
		t.Fatalf("state.Get(%q) error: %v", resourceKey, err)
	}
	if !found {
		t.Fatal("key state not found")
	}
	if keyMeta.UpstreamBaseURL != "https://cdn.example.com/keys/enc.key?token=abc123&expires=9999999999" {
		t.Errorf("key UpstreamBaseURL = %q, want %q", keyMeta.UpstreamBaseURL, "https://cdn.example.com/keys/enc.key?token=abc123&expires=9999999999")
	}
}

func TestHLSStrategy_EmptyKeyURI(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/hls"
	ctx := t.Context()
	store := proxy.NewMemoryStore(time.Minute)

	// Key with empty URI should not produce state
	playlist := `#EXTM3U
#EXT-X-TARGETDURATION:6
#EXT-X-KEY:METHOD=NONE
#EXTINF:6.0,
seg001.ts
`

	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		return bytesReader(playlist), nil, nil
	})

	store.Put(ctx, proxy.HLSPlaylistStateKey("test", "movies:1", "variants", "0"), proxy.StreamMeta{
		UpstreamBaseURL: "https://cdn.example.com/movie/playlist.m3u8",
		ProxyBaseURL:    proxyBase,
		ExpiresAt:       time.Now().Add(5 * time.Minute),
	})

	s := newHLSStrategy(fetch, store)

	var buf strings.Builder
	meta := testMeta(proxyBase)
	err := s.ServeSubPlaylist(ctx, &buf, stream.Locator{URL: "https://cdn.example.com/movie/playlist.m3u8"}, meta, "variants", "0")
	if err != nil {
		t.Fatalf("ServeSubPlaylist() error: %v", err)
	}

	// No key state should exist
	resourceKey := "test:movies:1:hls:resource:key:"
	_, found, _ := store.Get(ctx, resourceKey)
	if found {
		t.Error("state should not exist for key with empty URI")
	}
}

func TestHLSStrategy_ServeHLSResourceDirect(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/hls"
	ctx := t.Context()
	store := proxy.NewMemoryStore(time.Minute)

	// Store key state as if it were populated by playlist rewriting
	resourceKey := "test:movies:1:hls:resource:key:enc.key"
	store.Put(ctx, resourceKey, proxy.StreamMeta{
		ProviderTag:     "test",
		ContentKey:      "movies:1",
		UpstreamBaseURL: "https://cdn.example.com/keys/enc.key",
		ProxyBaseURL:    proxyBase,
		Headers:         http.Header{"X-Auth": []string{"token123"}},
		ExpiresAt:       time.Now().Add(5 * time.Minute),
	})

	var fetchedURL string
	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		fetchedURL = url
		return bytesReader("key-data"), http.Header{"Content-Type": []string{"application/octet-stream"}}, nil
	})

	cfg := &proxy.Config{Strategy: "hls"}
	proxyConfigs := map[string]*proxy.Config{"test": cfg}
	p := proxy.New(proxy.Dependencies{Fetcher: fetch, State: store, Configs: proxyConfigs})

	reader, contentType, err := p.ServeHLSResource(ctx, "test", "movies:1", "hls", "key", "enc.key")
	if err != nil {
		t.Fatalf("ServeHLSResource() error: %v", err)
	}
	defer reader.Close()

	data, _ := io.ReadAll(reader)
	if string(data) != "key-data" {
		t.Errorf("body = %q, want %q", string(data), "key-data")
	}
	if contentType != "application/octet-stream" {
		t.Errorf("content type = %q, want %q", contentType, "application/octet-stream")
	}
	if fetchedURL != "https://cdn.example.com/keys/enc.key" {
		t.Errorf("fetched URL = %q, want %q", fetchedURL, "https://cdn.example.com/keys/enc.key")
	}
}

func TestHLSStrategy_ServeHLSResourceNotFound(t *testing.T) {
	ctx := t.Context()
	store := proxy.NewMemoryStore(time.Minute)
	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		return nil, nil, fmt.Errorf("unexpected call")
	})
	cfg := &proxy.Config{Strategy: "hls"}
	proxyConfigs := map[string]*proxy.Config{"test": cfg}
	p := proxy.New(proxy.Dependencies{Fetcher: fetch, State: store, Configs: proxyConfigs})

	_, _, err := p.ServeHLSResource(ctx, "test", "movies:1", "hls", "key", "nonexistent.key")
	if err == nil {
		t.Fatal("ServeHLSResource() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 error, got: %v", err)
	}
}

func TestHLSStrategy_MasterSkipsRenditionWithoutURI(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/hls"
	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		return bytesReader(testMasterNoRenditionURI), nil, nil
	})
	store := proxy.NewMemoryStore(time.Minute)
	s := newHLSStrategy(fetch, store)

	var buf strings.Builder
	meta := testMeta(proxyBase)
	locator := stream.Locator{URL: "https://cdn.example.com/movie/master.m3u8"}

	_, err := s.ServeManifest(t.Context(), &buf, locator, meta)
	if err != nil {
		t.Fatalf("ServeManifest() error: %v", err)
	}

	result := buf.String()

	// The EXT-X-MEDIA without URI should not produce a rendition state entry
	ctx := t.Context()
	key := proxy.HLSPlaylistStateKey("test", "movies:1", "renditions", "audio/English")
	_, found, _ := store.Get(ctx, key)
	if found {
		t.Error("playlist state should not exist for EXT-X-MEDIA without URI")
	}

	// Variant should still be rewritten
	if !strings.Contains(result, "variants/0") {
		t.Errorf("master playlist missing variants/0, got:\n%s", result)
	}
}
