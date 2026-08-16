package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nem-git/abcmovies/internal/oas"
	"github.com/nem-git/abcmovies/internal/providers/stub"
	"github.com/nem-git/abcmovies/internal/proxy"
	"github.com/nem-git/abcmovies/internal/registry"
	"github.com/nem-git/abcmovies/internal/web"
)

func setupTest(t *testing.T, cfg stub.Config) *web.Handler {
	r := registry.New()
	if err := r.Register(stub.New(cfg)); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return web.New(r, "", "/api/v1alpha")
}

func TestServicesList(t *testing.T) {
	h := setupTest(t, stub.Config{
		Service: &oas.Service{Name: "TestService"},
	})

	req := httptest.NewRequest(http.MethodGet, "/services", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "TestService") {
		t.Errorf("expected body to contain 'TestService', got: %s", rec.Body.String())
	}
}

func TestServiceDetail(t *testing.T) {
	h := setupTest(t, stub.Config{
		Tag:     "TEST",
		Service: &oas.Service{Tag: "TEST", Name: "TestService"},
	})

	req := httptest.NewRequest(http.MethodGet, "/services/TEST", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "TestService") {
		t.Errorf("expected body to contain 'TestService', got: %s", rec.Body.String())
	}
}

func TestMoviesList(t *testing.T) {
	h := setupTest(t, stub.Config{
		Tag: "TEST",
		Movies: []oas.Movie{
			{Type: oas.MovieTypeMovie, ID: "m1", Name: "Test Movie"},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/services/TEST/movies", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Test Movie") {
		t.Errorf("expected body to contain 'Test Movie', got: %s", rec.Body.String())
	}
}

func TestMovieDetail(t *testing.T) {
	h := setupTest(t, stub.Config{
		Tag: "TEST",
		Movies: []oas.Movie{
			{Type: oas.MovieTypeMovie, ID: "m1", Name: "Test Movie"},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/services/TEST/movies/m1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Test Movie") {
		t.Errorf("expected body to contain 'Test Movie', got: %s", rec.Body.String())
	}
}

func TestSeriesList(t *testing.T) {
	h := setupTest(t, stub.Config{
		Tag: "TEST",
		Series: []oas.Series{
			{Type: oas.SeriesTypeTVSeries, ID: "s1", Name: "Test Series"},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/services/TEST/series", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Test Series") {
		t.Errorf("expected body to contain 'Test Series', got: %s", rec.Body.String())
	}
}

func TestSeriesDetail(t *testing.T) {
	h := setupTest(t, stub.Config{
		Tag: "TEST",
		Series: []oas.Series{
			{Type: oas.SeriesTypeTVSeries, ID: "s1", Name: "Test Series"},
		},
		Seasons: []oas.Season{
			{Type: oas.SeasonTypeTVSeason, ID: "ss1", Name: "Season 1"},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/services/TEST/series/s1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Test Series") {
		t.Errorf("expected body to contain 'Test Series', got: %s", body)
	}
	if !strings.Contains(body, "Season 1") {
		t.Errorf("expected body to contain 'Season 1', got: %s", body)
	}
}

func TestSeasonDetail(t *testing.T) {
	h := setupTest(t, stub.Config{
		Tag: "TEST",
		Series: []oas.Series{
			{Type: oas.SeriesTypeTVSeries, ID: "s1", Name: "Test Series"},
		},
		Seasons: []oas.Season{
			{Type: oas.SeasonTypeTVSeason, ID: "ss1", Name: "Season 1"},
		},
		Episodes: []oas.Episode{
			{Type: oas.EpisodeTypeTVEpisode, ID: "e1", Name: "Episode 1"},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/services/TEST/series/s1/seasons/ss1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Season 1") {
		t.Errorf("expected body to contain 'Season 1', got: %s", body)
	}
	if !strings.Contains(body, "Episode 1") {
		t.Errorf("expected body to contain 'Episode 1', got: %s", body)
	}
}

func TestEpisodeDetail(t *testing.T) {
	h := setupTest(t, stub.Config{
		Tag: "TEST",
		Series: []oas.Series{
			{Type: oas.SeriesTypeTVSeries, ID: "s1", Name: "Test Series"},
		},
		Seasons: []oas.Season{
			{Type: oas.SeasonTypeTVSeason, ID: "ss1", Name: "Season 1"},
		},
		Episodes: []oas.Episode{
			{Type: oas.EpisodeTypeTVEpisode, ID: "e1", Name: "Episode 1"},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/services/TEST/series/s1/seasons/ss1/episodes/e1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Episode 1") {
		t.Errorf("expected body to contain 'Episode 1', got: %s", rec.Body.String())
	}
}

func TestEpisodeDetailNoDownloadButton(t *testing.T) {
	h := setupTest(t, stub.Config{
		Tag: "TEST",
		Series: []oas.Series{
			{Type: oas.SeriesTypeTVSeries, ID: "s1", Name: "Test Series"},
		},
		Seasons: []oas.Season{
			{Type: oas.SeasonTypeTVSeason, ID: "ss1", Name: "Season 1"},
		},
		Episodes: []oas.Episode{
			{Type: oas.EpisodeTypeTVEpisode, ID: "e1", Name: "Episode 1"},
		},
		Streams: []oas.Stream{
			{Type: oas.StreamTypeVideoObject, ID: "master.m3u8", Name: "HLS Stream", EncodingFormat: oas.StreamEncodingFormatApplicationVndAppleMpegurl},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/services/TEST/series/s1/seasons/ss1/episodes/e1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "Download MP4") {
		t.Errorf("expected body to NOT contain 'Download MP4' for HLS-only content without conversion, got: %s", rec.Body.String())
	}
}

func TestEpisodeDetailNativeMP4(t *testing.T) {
	h := setupTest(t, stub.Config{
		Tag: "TEST",
		Series: []oas.Series{
			{Type: oas.SeriesTypeTVSeries, ID: "s1", Name: "Test Series"},
		},
		Seasons: []oas.Season{
			{Type: oas.SeasonTypeTVSeason, ID: "ss1", Name: "Season 1"},
		},
		Episodes: []oas.Episode{
			{Type: oas.EpisodeTypeTVEpisode, ID: "e1", Name: "Episode 1"},
		},
		Streams: []oas.Stream{
			{Type: oas.StreamTypeVideoObject, ID: "video.mp4", Name: "MP4 Stream", EncodingFormat: oas.StreamEncodingFormatVideoMP4},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/services/TEST/series/s1/seasons/ss1/episodes/e1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Download MP4") {
		t.Errorf("expected body to contain 'Download MP4' for native MP4 stream, got: %s", rec.Body.String())
	}
}

func TestEpisodeDetailConvertEnabled(t *testing.T) {
	r := registry.New()
	r.Register(stub.New(stub.Config{
		Tag: "TEST",
		Series: []oas.Series{
			{Type: oas.SeriesTypeTVSeries, ID: "s1", Name: "Test Series"},
		},
		Seasons: []oas.Season{
			{Type: oas.SeasonTypeTVSeason, ID: "ss1", Name: "Season 1"},
		},
		Episodes: []oas.Episode{
			{Type: oas.EpisodeTypeTVEpisode, ID: "e1", Name: "Episode 1"},
		},
		Streams: []oas.Stream{
			{Type: oas.StreamTypeVideoObject, ID: "master.m3u8", Name: "HLS Stream", EncodingFormat: oas.StreamEncodingFormatApplicationVndAppleMpegurl},
		},
	}))
	px := proxy.New(proxy.Dependencies{
		State:   proxy.NewMemoryStore(5 * time.Minute),
		Configs: map[string]*proxy.Config{"TEST": {Strategy: "hls", Convert: true}},
	})
	h := web.New(r, "", "/api/v1alpha", px)

	req := httptest.NewRequest(http.MethodGet, "/services/TEST/series/s1/seasons/ss1/episodes/e1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Download MP4") {
		t.Errorf("expected body to contain 'Download MP4' when conversion is enabled, got: %s", rec.Body.String())
	}
}

func TestSearchResults(t *testing.T) {
	h := setupTest(t, stub.Config{
		Tag: "TEST",
		Movies: []oas.Movie{
			{Type: oas.MovieTypeMovie, ID: "m1", Name: "Batman Begins"},
			{Type: oas.MovieTypeMovie, ID: "m2", Name: "Superman Returns"},
		},
		Search: []oas.SearchResultItem{
			{Resource: oas.SearchResultItemResource{Type: oas.MovieSearchResultItemResource, Movie: oas.Movie{ID: "m1", Name: "Batman Begins"}}},
			{Resource: oas.SearchResultItemResource{Type: oas.MovieSearchResultItemResource, Movie: oas.Movie{ID: "m2", Name: "Superman Returns"}}},
		},
	})

	t.Run("search returns scored results", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/search?q=Batman", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "Batman Begins") {
			t.Errorf("expected body to contain 'Batman Begins', got: %s", body)
		}
		if !strings.Contains(body, "Results for") {
			t.Errorf("expected body to contain 'Results for', got: %s", body)
		}
	})

	t.Run("search empty query shows empty state", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/search?q=", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Type something to search") {
			t.Errorf("expected empty state message, got: %s", rec.Body.String())
		}
	})

	t.Run("htmx search returns fragment", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/search?q=Batman", nil)
		req.Header.Set("HX-Request", "true")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Batman Begins") {
			t.Errorf("expected body to contain 'Batman Begins', got: %s", rec.Body.String())
		}
	})

	t.Run("filters by type", func(t *testing.T) {
		h2 := setupTest(t, stub.Config{
			Tag: "FLT",
			Search: []oas.SearchResultItem{
				{Resource: oas.SearchResultItemResource{Type: oas.MovieSearchResultItemResource, Movie: oas.Movie{ID: "m1", Name: "Batman Begins"}}},
				{Resource: oas.SearchResultItemResource{Type: oas.SeriesSearchResultItemResource, Series: oas.Series{ID: "s1", Name: "Batman Animated"}}},
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/search?q=Batman&type=movie", nil)
		rec := httptest.NewRecorder()
		h2.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "Batman Begins") {
			t.Errorf("expected body to contain 'Batman Begins', got: %s", body)
		}
		if strings.Contains(body, "Batman Animated") {
			t.Errorf("expected body to NOT contain 'Batman Animated', got: %s", body)
		}
	})
}
