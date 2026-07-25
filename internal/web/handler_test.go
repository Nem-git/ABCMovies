package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nem-git/abcmovies/internal/oas"
	"github.com/nem-git/abcmovies/internal/providers/stub"
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
