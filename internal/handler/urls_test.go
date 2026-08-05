package handler_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/nem-git/abcmovies/internal/handler"
	"github.com/nem-git/abcmovies/internal/oas"
	"github.com/nem-git/abcmovies/internal/providers/stub"
	"github.com/nem-git/abcmovies/internal/registry"
)

func uri(raw string) oas.OptURI {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return oas.NewOptURI(*u)
}

func newAPITestServer(t *testing.T, baseURL string, r *registry.Registry) *httptest.Server {
	t.Helper()
	h := handler.New(r, baseURL, "")
	srv, err := oas.NewServer(h, oas.WithPathPrefix("/api/v1alpha"))
	if err != nil {
		t.Fatalf("oas.NewServer() error: %v", err)
	}
	ts := httptest.NewServer(handler.WithRequest(srv))
	t.Cleanup(ts.Close)
	return ts
}

func apiGet(t *testing.T, ts *httptest.Server, path string, headers map[string]string) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error: %v", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s error: %v", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d", path, resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body error: %v", err)
	}
	return string(b)
}

func movieStub() *stub.Provider {
	return stub.New(stub.Config{
		Tag: "TEST",
		Movies: []oas.Movie{
			{
				ID:       "m1",
				Name:     "Movie 1",
				Poster:   uri("/api/v1alpha/services/TEST/movies/m1/poster"),
				Backdrop: uri("/api/v1alpha/services/TEST/movies/m1/backdrop"),
				Trailer:  uri("/api/v1alpha/services/TEST/movies/m1/trailer"),
			},
		},
	})
}

func TestImageURLsAbsoluteRequestFallback(t *testing.T) {
	r := registry.New()
	r.Register(movieStub())

	ts := newAPITestServer(t, "", r)
	body := apiGet(t, ts, "/api/v1alpha/services/TEST/movies/m1", nil)

	for _, suffix := range []string{"poster", "backdrop", "trailer"} {
		want := ts.URL + "/api/v1alpha/services/TEST/movies/m1/" + suffix
		if !strings.Contains(body, want) {
			t.Errorf("body missing absolute %s URL %q", suffix, want)
		}
	}
	if strings.Contains(body, "\"/api/v1alpha/services/TEST/movies/m1/poster\"") {
		t.Errorf("body still contains a relative poster URL: %s", body)
	}
}

func TestImageURLsAbsoluteConfigPrimary(t *testing.T) {
	r := registry.New()
	r.Register(movieStub())

	ts := newAPITestServer(t, "https://api.example.com", r)
	body := apiGet(t, ts, "/api/v1alpha/services/TEST/movies/m1", nil)

	if !strings.Contains(body, "https://api.example.com/api/v1alpha/services/TEST/movies/m1/poster") {
		t.Errorf("body missing base_url poster URL: %s", body)
	}
	if strings.Contains(body, ts.URL) {
		t.Errorf("configured base_url ignored, used request host %s: %s", ts.URL, body)
	}
}

func TestImageURLsHonorForwardedHeaders(t *testing.T) {
	r := registry.New()
	r.Register(movieStub())

	ts := newAPITestServer(t, "", r)
	body := apiGet(t, ts, "/api/v1alpha/services/TEST/movies/m1", map[string]string{
		"X-Forwarded-Proto": "https",
		"X-Forwarded-Host":  "edge.example.com",
	})

	if !strings.Contains(body, "https://edge.example.com/api/v1alpha/services/TEST/movies/m1/poster") {
		t.Errorf("X-Forwarded-* headers ignored: %s", body)
	}
}

func TestAbsoluteURLsUntouched(t *testing.T) {
	r := registry.New()
	p := stub.New(stub.Config{
		Tag: "TEST",
		Service: &oas.Service{
			Tag:     "test",
			Name:    "Test",
			Website: uri("https://www.example.com"),
			Logo:    uri("https://cdn.example.com/logo.png"),
		},
		Movies: []oas.Movie{
			{
				ID:      "m1",
				Name:    "Movie 1",
				Poster:  uri("https://cdn.example.com/m1/poster.jpg"),
				Trailer: uri("https://cdn.example.com/m1/trailer.m3u8"),
			},
		},
	})
	r.Register(p)

	ts := newAPITestServer(t, "http://localhost:8080", r)

	body := apiGet(t, ts, "/api/v1alpha/services/TEST/movies/m1", nil)
	for _, want := range []string{
		"https://cdn.example.com/m1/poster.jpg",
		"https://cdn.example.com/m1/trailer.m3u8",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing already-absolute URL %q", want)
		}
	}
	if strings.Contains(body, "http://localhost:8080/https://cdn.example.com") {
		t.Errorf("already-absolute URL was rewritten: %s", body)
	}

	body = apiGet(t, ts, "/api/v1alpha/services/TEST", nil)
	if !strings.Contains(body, "https://www.example.com") {
		t.Errorf("service website URL missing: %s", body)
	}
}

func TestSearchResultAbsoluteURLs(t *testing.T) {
	m := oas.Movie{
		ID:     "m1",
		Name:   "Movie 1",
		Poster: uri("/api/v1alpha/services/TEST/movies/m1/poster"),
	}
	item := oas.SearchResultItem{Score: 1}
	item.Resource.SetMovie(m)

	r := registry.New()
	r.Register(stub.New(stub.Config{Tag: "TEST", Search: []oas.SearchResultItem{item}}))

	ts := newAPITestServer(t, "", r)
	body := apiGet(t, ts, "/api/v1alpha/search?q=Movie", nil)

	want := ts.URL + "/api/v1alpha/services/TEST/movies/m1/poster"
	if !strings.Contains(body, want) {
		t.Errorf("search result missing absolute poster URL %q: %s", want, body)
	}
}

func TestSubtitleURLAbsolute(t *testing.T) {
	r := registry.New()
	p := stub.New(stub.Config{
		Tag:       "TEST",
		Subtitles: []oas.Subtitle{{ID: "en", URL: uri("/api/v1alpha/services/TEST/movies/m1/subtitles/en")}},
	})
	r.Register(p)

	ts := newAPITestServer(t, "", r)
	body := apiGet(t, ts, "/api/v1alpha/services/TEST/movies/m1/subtitles", nil)

	want := ts.URL + "/api/v1alpha/services/TEST/movies/m1/subtitles/en"
	if !strings.Contains(body, want) {
		t.Errorf("subtitle URL not absolute %q: %s", want, body)
	}
}
