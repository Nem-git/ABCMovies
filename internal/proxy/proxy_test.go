package proxy_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nem-git/abcmovies/internal/proxy"
	"github.com/nem-git/abcmovies/internal/stream"
)

func TestMemoryStore_PutGet(t *testing.T) {
	s := proxy.NewMemoryStore(time.Minute)
	ctx := context.Background()

	meta := proxy.StreamMeta{
		ProviderTag:     "TEST",
		ContentKey:      "movies:123",
		Format:          "hls",
		UpstreamBaseURL: "https://cdn.example.com/movie/720p/",
		ExpiresAt:       time.Now().Add(time.Minute),
	}

	if err := s.Put(ctx, "key1", meta); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	got, found, err := s.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if !found {
		t.Fatal("Get() found = false, want true")
	}
	if got.ProviderTag != "TEST" {
		t.Errorf("ProviderTag = %q, want %q", got.ProviderTag, "TEST")
	}
	if got.UpstreamBaseURL != "https://cdn.example.com/movie/720p/" {
		t.Errorf("UpstreamBaseURL = %q, want %q", got.UpstreamBaseURL, "https://cdn.example.com/movie/720p/")
	}
}

func TestMemoryStore_Miss(t *testing.T) {
	s := proxy.NewMemoryStore(time.Minute)
	ctx := context.Background()

	_, found, err := s.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if found {
		t.Error("Get() found = true, want false")
	}
}

func TestMemoryStore_Expiry(t *testing.T) {
	s := proxy.NewMemoryStore(time.Millisecond)
	ctx := context.Background()

	meta := proxy.StreamMeta{
		ProviderTag: "TEST",
		ExpiresAt:   time.Now().Add(time.Millisecond),
	}

	s.Put(ctx, "key1", meta)
	time.Sleep(5 * time.Millisecond)

	_, found, err := s.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if found {
		t.Error("Get() found = true after expiry, want false")
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	s := proxy.NewMemoryStore(time.Minute)
	ctx := context.Background()

	meta := proxy.StreamMeta{
		ProviderTag: "TEST",
		ExpiresAt:   time.Now().Add(time.Minute),
	}

	s.Put(ctx, "key1", meta)
	s.Delete(ctx, "key1")

	_, found, _ := s.Get(ctx, "key1")
	if found {
		t.Error("Get() found = true after Delete, want false")
	}
}

func TestMemoryStore_Cleanup(t *testing.T) {
	s := proxy.NewMemoryStore(time.Minute)
	ctx := context.Background()

	s.Put(ctx, "expired", proxy.StreamMeta{ProviderTag: "TEST", ExpiresAt: time.Now().Add(-time.Minute)})
	s.Put(ctx, "valid", proxy.StreamMeta{ProviderTag: "TEST", ExpiresAt: time.Now().Add(time.Hour)})

	s.Cleanup(ctx)

	_, found, _ := s.Get(ctx, "expired")
	if found {
		t.Error("expired key still found after Cleanup")
	}
	_, found, _ = s.Get(ctx, "valid")
	if !found {
		t.Error("valid key not found after Cleanup")
	}
}

func TestMemoryStore_Concurrent(t *testing.T) {
	s := proxy.NewMemoryStore(time.Minute)
	ctx := context.Background()

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			defer func() { done <- struct{}{} }()
			key := "key"
			meta := proxy.StreamMeta{
				ProviderTag: "TEST",
				ExpiresAt:   time.Now().Add(time.Minute),
			}
			s.Put(ctx, key, meta)
			s.Get(ctx, key)
			s.Delete(ctx, key)
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestHTTPFetcher_Success(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("hello"))
	}))
	defer upstream.Close()

	f := proxy.NewHTTPFetcher(upstream.Client())
	body, headers, err := f.Fetch(t.Context(), upstream.URL+"/test", nil, nil)
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	defer body.Close()

	if headers.Get("Content-Type") != "text/plain" {
		t.Errorf("Content-Type = %q, want %q", headers.Get("Content-Type"), "text/plain")
	}
	data, _ := io.ReadAll(body)
	if string(data) != "hello" {
		t.Errorf("body = %q, want %q", string(data), "hello")
	}
}

func TestHTTPFetcher_Error(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer upstream.Close()

	f := proxy.NewHTTPFetcher(upstream.Client())
	_, _, err := f.Fetch(t.Context(), upstream.URL+"/missing", nil, nil)
	if err == nil {
		t.Fatal("Fetch() error = nil, want error")
	}
}

func TestHTTPFetcher_HeadersAndQuery(t *testing.T) {
	var gotHeaders http.Header
	var gotQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		gotQuery = r.URL.RawQuery
		w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	f := proxy.NewHTTPFetcher(upstream.Client())
	headers := http.Header{"Authorization": []string{"Bearer token123"}}
	query := map[string][]string{"key": {"value"}}

	_, _, err := f.Fetch(t.Context(), upstream.URL+"/test", headers, query)
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	if gotHeaders.Get("Authorization") != "Bearer token123" {
		t.Errorf("Authorization = %q, want %q", gotHeaders.Get("Authorization"), "Bearer token123")
	}
	if gotQuery != "key=value" {
		t.Errorf("query = %q, want %q", gotQuery, "key=value")
	}
}

func TestBuildStateKey(t *testing.T) {
	tests := []struct {
		contentType string
		ids         []string
		want        string
	}{
		{"movies", []string{"123"}, "movies:123"},
		{"series", []string{"456", "789", "101"}, "series:456:789:101"},
	}
	for _, tt := range tests {
		got := proxy.BuildStateKey(append([]string{tt.contentType}, tt.ids...)...)
		if got != tt.want {
			t.Errorf("BuildStateKey(%q, %v) = %q, want %q", tt.contentType, tt.ids, got, tt.want)
		}
	}
}

func TestResolveBaseURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://cdn.example.com/movie/720p/playlist.m3u8", "https://cdn.example.com/movie/720p/"},
		{"https://cdn.example.com/movie/master.m3u8", "https://cdn.example.com/movie/"},
		{"https://cdn.example.com/manifest.mpd", "https://cdn.example.com/"},
	}
	for _, tt := range tests {
		got := proxy.ResolveBaseURL(tt.input)
		if got != tt.want {
			t.Errorf("ResolveBaseURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestProxy_ServeDASHSegmentReconstructsFromFilename(t *testing.T) {
	var fetchedURL string
	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		fetchedURL = url
		return bytesReader("segment-data"), nil, nil
	})
	store := proxy.NewMemoryStore(time.Minute)
	p := proxy.New(proxy.Dependencies{
		Fetcher: fetch,
		State:   store,
		Configs: map[string]*proxy.Config{"test": {Strategy: "dash"}},
	})

	ctx := t.Context()
	key := proxy.DASHStateKey("test", "movies:1", 0, 0, "v1")
	store.Put(ctx, key, proxy.StreamMeta{
		ProviderTag:           "test",
		ContentKey:            "movies:1",
		Format:                "dash",
		UpstreamMediaTemplate: "https://cdn.example.com/movie/v1/seg_$Number%05d$.m4s",
		UpstreamRepID:         "v1",
		UpstreamBandwidth:     "5000000",
		Headers:               http.Header{},
		ExpiresAt:             time.Now().Add(5 * time.Minute),
	})

	reader, _, err := p.ServeDASHSegment(ctx, "test", "movies:1", "dash", 0, 0, "v1", "seg_00007.m4s")
	if err != nil {
		t.Fatalf("ServeDASHSegment() error: %v", err)
	}
	defer reader.Close()
	io.Copy(io.Discard, reader) // drain to ensure the upstream fetch ran

	want := "https://cdn.example.com/movie/v1/seg_00007.m4s"
	if fetchedURL != want {
		t.Errorf("fetched upstream URL = %q, want %q", fetchedURL, want)
	}
}

func TestProxy_ServeManifestStoresStateWithoutFile(t *testing.T) {
	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		return bytesReader(testDASHNumberMPD), nil, nil
	})
	store := proxy.NewMemoryStore(time.Minute)
	p := proxy.New(proxy.Dependencies{
		Fetcher: fetch,
		State:   store,
		Configs: map[string]*proxy.Config{"test": {Strategy: "dash"}},
	})

	locator := stream.Locator{URL: "https://cdn.example.com/movie/manifest.mpd"}
	reader, _, err := p.ServeManifest(t.Context(), "test", "movies:1", "dash", "manifest.mpd", locator, "/api/v1alpha/services/test/movies/1/streams/dash")
	if err != nil {
		t.Fatalf("ServeManifest() error: %v", err)
	}
	defer reader.Close()
	io.Copy(io.Discard, reader)

	key := proxy.BuildStateKey("test", "movies:1", "dash")
	_, found, err := store.Get(t.Context(), key)
	if err != nil {
		t.Fatalf("state.Get(%q) error: %v", key, err)
	}
	if !found {
		t.Errorf("state not stored at %q (no file component)", key)
	}
}

func TestProxy_ServeDASHInitStripsSegmentPlaceholders(t *testing.T) {
	var fetchedURL string
	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		fetchedURL = url
		return bytesReader("init-data"), nil, nil
	})
	store := proxy.NewMemoryStore(time.Minute)
	p := proxy.New(proxy.Dependencies{
		Fetcher: fetch,
		State:   store,
		Configs: map[string]*proxy.Config{"test": {Strategy: "dash"}},
	})

	ctx := t.Context()
	key := proxy.DASHStateKey("test", "movies:1", 0, 0, "v1")
	// Nonconformant init template carrying $Number$/$Time$ must not leak into the fetched URL.
	store.Put(ctx, key, proxy.StreamMeta{
		ProviderTag:          "test",
		ContentKey:           "movies:1",
		Format:               "dash",
		UpstreamInitTemplate: "https://cdn.example.com/movie/$RepresentationID$_$Bandwidth$/$Number%05d$_init.mp4",
		UpstreamRepID:        "v1",
		UpstreamBandwidth:    "5000000",
		Headers:              http.Header{},
		ExpiresAt:            time.Now().Add(5 * time.Minute),
	})

	reader, _, err := p.ServeDASHInit(ctx, "test", "movies:1", "dash", 0, 0, "v1")
	if err != nil {
		t.Fatalf("ServeDASHInit() error: %v", err)
	}
	defer reader.Close()
	io.Copy(io.Discard, reader)

	want := "https://cdn.example.com/movie/v1_5000000/_init.mp4"
	if fetchedURL != want {
		t.Errorf("fetched upstream URL = %q, want %q", fetchedURL, want)
	}
}
