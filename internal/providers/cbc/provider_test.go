package cbc_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/nem-git/abcmovies/internal/oas"
	"github.com/nem-git/abcmovies/internal/provider"
	"github.com/nem-git/abcmovies/internal/providers/cbc"
)

func TestTag(t *testing.T) {
	p := cbc.New(cbc.Config{Tag: "CBC"})
	if got := p.Tag(); got != "CBC" {
		t.Errorf("Tag() = %q, want %q", got, "CBC")
	}
}

func TestService(t *testing.T) {
	srv := &oas.Service{Tag: "CBC", Name: "CBC Gem"}
	p := cbc.New(cbc.Config{Service: srv})
	got, err := p.Service(t.Context())
	if err != nil {
		t.Fatalf("Service() error: %v", err)
	}
	if got != srv {
		t.Errorf("Service() returned different pointer")
	}
}

func TestHealth(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ott/catalog/v2/gem/browse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"formats": []map[string]string{}})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	p := newTestProvider(ts.URL)
	got, err := p.Health(t.Context())
	if err != nil {
		t.Fatalf("Health() error: %v", err)
	}
	if got.Status != oas.HealthStatusOk {
		t.Errorf("Health().Status = %q, want %q", got.Status, oas.HealthStatusOk)
	}
}

func TestGetSeriesByID(t *testing.T) {
	ts := showTestServer(t, `
	{
		"content": [{
			"items": {"results": [
				{
					"type": "show",
					"title": "Schitt's Creek",
					"url": "schitts-creek",
					"description": "A wealthy family loses everything.",
					"images": {
						"card": {"url": "https://img.example.com/poster.jpg"},
						"background": {"url": "https://img.example.com/bg.jpg"}
					},
					"metadata": {"country": "Canada", "duration": "PT30M"}
				}
			]},
			"lineups": [
				{"seasonNumber": 1, "title": "Season 1", "items": [{"idMedia": 101, "title": "Pilot"}]},
				{"seasonNumber": 2, "title": "Season 2", "items": [{"idMedia": 201, "title": "Happy Anniversary"}]}
			]
		}]
	}`)
	defer ts.Close()

	p := newTestProvider(ts.URL)
	s, err := p.GetSeriesByID(t.Context(), "schitts-creek")
	if err != nil {
		t.Fatalf("GetSeriesByID() error: %v", err)
	}
	if s.GetName() != "Schitt's Creek" {
		t.Errorf("Name = %q, want %q", s.GetName(), "Schitt's Creek")
	}
	if s.NumberOfSeasons != 2 {
		t.Errorf("NumberOfSeasons = %d, want 2", s.NumberOfSeasons)
	}
	if s.Description.Value != "A wealthy family loses everything." {
		t.Errorf("Description = %q", s.Description.Value)
	}
}

func TestGetSeriesByID_nonexistent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ott/catalog/v2/gem/show/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	p := newTestProvider(ts.URL)
	_, err := p.GetSeriesByID(t.Context(), "nonexistent")
	if err == nil {
		t.Fatal("GetSeriesByID() expected error")
	}
}

func TestGetSeries(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ott/catalog/v2/gem/category/shows", func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case page == 1 && pageSize == 2:
			json.NewEncoder(w).Encode(map[string]any{
				"content": []map[string]any{
					{
						"items": map[string]any{
							"totalPages":   2,
							"totalRecords": 3,
							"pageNumber":   1,
							"pageSize":     2,
							"results": []map[string]any{
								{"type": "show", "title": "Show A", "url": "show-a", "description": "Desc A", "images": map[string]any{"card": map[string]any{"url": "https://img.example.com/a.jpg"}}},
								{"type": "show", "title": "Show B", "url": "show-b", "description": "Desc B"},
							},
						},
					},
				},
			})
		case page == 2 && pageSize == 2:
			json.NewEncoder(w).Encode(map[string]any{
				"content": []map[string]any{
					{
						"items": map[string]any{
							"totalPages":   2,
							"totalRecords": 3,
							"pageNumber":   2,
							"pageSize":     2,
							"results": []map[string]any{
								{"type": "show", "title": "Show C", "url": "show-c", "description": "Desc C"},
							},
						},
					},
				},
			})
		default:
			json.NewEncoder(w).Encode(map[string]any{
				"content": []map[string]any{
					{
						"items": map[string]any{
							"totalPages":   2,
							"totalRecords": 3,
							"pageNumber":   page,
							"pageSize":     pageSize,
							"results":      []map[string]any{},
						},
					},
				},
			})
		}
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	p := newTestProvider(ts.URL)

	t.Run("first page", func(t *testing.T) {
		series, total, err := p.GetSeries(t.Context(), 2, 0)
		if err != nil {
			t.Fatalf("GetSeries() error: %v", err)
		}
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
		if len(series) != 2 {
			t.Fatalf("got %d series, want 2", len(series))
		}
		if series[0].GetName() != "Show A" {
			t.Errorf("series[0].Name = %q, want %q", series[0].GetName(), "Show A")
		}
		if series[0].GetID() != "show-a" {
			t.Errorf("series[0].ID = %q, want %q", series[0].GetID(), "show-a")
		}
		if series[0].Description.Value != "Desc A" {
			t.Errorf("series[0].Description = %q, want %q", series[0].Description.Value, "Desc A")
		}
		if !series[0].Poster.IsSet() {
			t.Error("series[0].Poster should be set")
		}
	})

	t.Run("second page", func(t *testing.T) {
		series, total, err := p.GetSeries(t.Context(), 2, 2)
		if err != nil {
			t.Fatalf("GetSeries() error: %v", err)
		}
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
		if len(series) != 1 {
			t.Fatalf("got %d series, want 1", len(series))
		}
		if series[0].GetName() != "Show C" {
			t.Errorf("series[0].Name = %q, want %q", series[0].GetName(), "Show C")
		}
	})

	t.Run("offset past end", func(t *testing.T) {
		series, total, err := p.GetSeries(t.Context(), 10, 10)
		if err != nil {
			t.Fatalf("GetSeries() error: %v", err)
		}
		if len(series) != 0 {
			t.Errorf("got %d series, want 0", len(series))
		}
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
	})
}

func TestGetSeasons(t *testing.T) {
	ts := showTestServer(t, `
	{
		"content": [{
			"items": {"results": [{"type": "show", "title": "S", "url": "s"}]},
			"lineups": [
				{"seasonNumber": 1, "title": "Season 1", "items": [{"idMedia": 101, "title": "Ep1"}]},
				{"seasonNumber": 2, "title": "Season 2", "items": [{"idMedia": 201, "title": "Ep2"}]},
				{"seasonNumber": 3, "title": "Season 3", "items": [{"idMedia": 301, "title": "Ep3"}]}
			]
		}]
	}`)
	defer ts.Close()

	p := newTestProvider(ts.URL)
	seasons, total, err := p.GetSeasons(t.Context(), "s", 10, 0)
	if err != nil {
		t.Fatalf("GetSeasons() error: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(seasons) != 3 {
		t.Fatalf("got %d seasons, want 3", len(seasons))
	}
	if seasons[0].GetName() != "Season 1" {
		t.Errorf("season[0].Name = %q, want %q", seasons[0].GetName(), "Season 1")
	}
	if seasons[0].SeasonNumber.Value != 1 {
		t.Errorf("season[0].SeasonNumber = %d, want 1", seasons[0].SeasonNumber.Value)
	}
}

func TestGetSeasons_pagination(t *testing.T) {
	ts := showTestServer(t, `
	{
		"content": [{
			"items": {"results": [{"type": "show", "title": "S", "url": "s"}]},
			"lineups": [
				{"seasonNumber": 1, "items": [{"idMedia": 1}]},
				{"seasonNumber": 2, "items": [{"idMedia": 2}]},
				{"seasonNumber": 3, "items": [{"idMedia": 3}]},
				{"seasonNumber": 4, "items": [{"idMedia": 4}]},
				{"seasonNumber": 5, "items": [{"idMedia": 5}]}
			]
		}]
	}`)
	defer ts.Close()

	p := newTestProvider(ts.URL)

	t.Run("first page", func(t *testing.T) {
		seasons, total, err := p.GetSeasons(t.Context(), "s", 2, 0)
		if err != nil {
			t.Fatalf("GetSeasons() error: %v", err)
		}
		if total != 5 {
			t.Errorf("total = %d, want 5", total)
		}
		if len(seasons) != 2 {
			t.Fatalf("got %d seasons, want 2", len(seasons))
		}
		if seasons[0].SeasonNumber.Value != 1 {
			t.Errorf("first season number = %d, want 1", seasons[0].SeasonNumber.Value)
		}
	})

	t.Run("second page", func(t *testing.T) {
		seasons, total, err := p.GetSeasons(t.Context(), "s", 2, 2)
		if err != nil {
			t.Fatalf("GetSeasons() error: %v", err)
		}
		if total != 5 {
			t.Errorf("total = %d, want 5", total)
		}
		if len(seasons) != 2 {
			t.Fatalf("got %d seasons, want 2", len(seasons))
		}
		if seasons[0].SeasonNumber.Value != 3 {
			t.Errorf("first season number = %d, want 3", seasons[0].SeasonNumber.Value)
		}
	})

	t.Run("offset past end", func(t *testing.T) {
		seasons, total, err := p.GetSeasons(t.Context(), "s", 10, 10)
		if err != nil {
			t.Fatalf("GetSeasons() error: %v", err)
		}
		if total != 5 {
			t.Errorf("total = %d, want 5", total)
		}
		if len(seasons) != 0 {
			t.Errorf("got %d seasons, want 0", len(seasons))
		}
	})
}

func TestGetSeasonById(t *testing.T) {
	ts := showTestServer(t, `
	{
		"content": [{
			"items": {"results": [{"type": "show", "title": "S", "url": "s"}]},
			"lineups": [
				{"seasonNumber": 1, "items": []},
				{"seasonNumber": 2, "items": []}
			]
		}]
	}`)
	defer ts.Close()

	p := newTestProvider(ts.URL)

	t.Run("found", func(t *testing.T) {
		s, err := p.GetSeasonById(t.Context(), "s", "s02")
		if err != nil {
			t.Fatalf("GetSeasonById() error: %v", err)
		}
		if s.SeasonNumber.Value != 2 {
			t.Errorf("SeasonNumber = %d, want 2", s.SeasonNumber.Value)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := p.GetSeasonById(t.Context(), "s", "s99")
		if err != provider.ErrNotSupported {
			t.Errorf("GetSeasonById() error = %v, want ErrNotSupported", err)
		}
	})
}

func TestGetEpisodes(t *testing.T) {
	ts := showTestServer(t, `
	{
		"content": [{
			"items": {"results": [{"type": "show", "title": "S", "url": "s"}]},
			"lineups": [
				{
					"seasonNumber": 1,
					"items": [
						{"idMedia": 101, "title": "Episode 1", "description": "First ep", "metadata": {"duration": 1800}},
						{"idMedia": 102, "title": "Episode 2", "metadata": {"duration": 1860}}
					]
				}
			]
		}]
	}`)
	defer ts.Close()

	p := newTestProvider(ts.URL)
	eps, total, err := p.GetEpisodes(t.Context(), "s", "s01", 10, 0)
	if err != nil {
		t.Fatalf("GetEpisodes() error: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(eps) != 2 {
		t.Fatalf("got %d episodes, want 2", len(eps))
	}
	if eps[0].GetName() != "Episode 1" {
		t.Errorf("episode[0].Name = %q, want %q", eps[0].GetName(), "Episode 1")
	}
	if eps[0].GetID() != "101" {
		t.Errorf("episode[0].ID = %q, want %q", eps[0].GetID(), "101")
	}
	if eps[0].Duration != "PT30M" {
		t.Errorf("episode[0].Duration = %q, want %q", eps[0].Duration, "PT30M")
	}
}

func TestGetEpisodeById(t *testing.T) {
	ts := showTestServer(t, `
	{
		"content": [{
			"items": {"results": [{"type": "show", "title": "S", "url": "s"}]},
			"lineups": [
				{"seasonNumber": 1, "items": [
					{"idMedia": 101, "title": "First"},
					{"idMedia": 102, "title": "Second"}
				]}
			]
		}]
	}`)
	defer ts.Close()

	p := newTestProvider(ts.URL)

	t.Run("found", func(t *testing.T) {
		ep, err := p.GetEpisodeById(t.Context(), "s", "s01", "102")
		if err != nil {
			t.Fatalf("GetEpisodeById() error: %v", err)
		}
		if ep.GetName() != "Second" {
			t.Errorf("Name = %q, want %q", ep.GetName(), "Second")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := p.GetEpisodeById(t.Context(), "s", "s01", "999")
		if err != provider.ErrNotSupported {
			t.Errorf("GetEpisodeById() error = %v, want ErrNotSupported", err)
		}
	})
}

func TestGetEpisodeStreams(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/media/meta/v1/index.ashx", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"idMedia": "101",
			"availableTechs": []map[string]any{
				{"name": "dash", "manifestVersions": []string{"1", "2"}},
				{"name": "hls", "manifestVersions": []string{"1"}},
			},
		})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	p := newTestProvider(ts.URL)
	streams, total, err := p.GetEpisodeStreams(t.Context(), "s", "s01", "101")
	if err != nil {
		t.Fatalf("GetEpisodeStreams() error: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(streams) != 2 {
		t.Fatalf("got %d streams, want 2", len(streams))
	}
	if streams[0].GetID() != "manifest.mpd" {
		t.Errorf("stream[0].ID = %q, want %q", streams[0].GetID(), "manifest.mpd")
	}
	if streams[1].GetID() != "master.m3u8" {
		t.Errorf("stream[1].ID = %q, want %q", streams[1].GetID(), "master.m3u8")
	}
}

func TestGetEpisodeStreamFile_dash(t *testing.T) {
	var ts *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/media/validation/v2/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"url":    ts.URL + "/manifest.mpd",
			"params": []map[string]string{},
		})
	})
	mux.HandleFunc("/manifest.mpd", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/dash+xml")
		w.Write([]byte(`<?xml version="1.0"?><MPD></MPD>`))
	})
	ts = httptest.NewServer(mux)
	defer ts.Close()

	p := newTestProvider(ts.URL)
	rc, mime, err := p.GetEpisodeStreamFile(t.Context(), "s", "s01", "101", "manifest.mpd")
	if err != nil {
		t.Fatalf("GetEpisodeStreamFile() error: %v", err)
	}
	defer rc.Close()
	if mime != "application/dash+xml" {
		t.Errorf("mime = %q, want %q", mime, "application/dash+xml")
	}
	data, _ := io.ReadAll(rc)
	if !strings.Contains(string(data), "<MPD>") {
		t.Errorf("body = %q, want MPD XML", string(data))
	}
}

func TestGetEpisodeStreamFile_unknown(t *testing.T) {
	p := cbc.New(cbc.Config{Tag: "CBC"})
	_, _, err := p.GetEpisodeStreamFile(t.Context(), "s", "s01", "101", "unknown.mpd")
	if err == nil {
		t.Fatal("expected error for unknown stream file")
	}
}

func TestGetEpisodeSubtitle(t *testing.T) {
	p := cbc.New(cbc.Config{Tag: "CBC"})
	_, _, err := p.GetEpisodeSubtitles(t.Context(), "s", "s01", "101")
	if err != provider.ErrNotSupported {
		t.Errorf("GetEpisodeSubtitles() error = %v, want ErrNotSupported", err)
	}
	_, _, err = p.GetEpisodeSubtitleFile(t.Context(), "s", "s01", "101", "en.vtt")
	if err != provider.ErrNotSupported {
		t.Errorf("GetEpisodeSubtitleFile() error = %v, want ErrNotSupported", err)
	}
}

func TestGetEpisodeThumbnail(t *testing.T) {
	var ts *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/ott/catalog/v2/gem/show/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{
					"items": map[string]any{
						"results": []map[string]any{{"type": "show", "title": "S", "url": "s"}},
					},
					"lineups": []map[string]any{
						{
							"seasonNumber": 1,
							"items": []map[string]any{
								{
									"idMedia": 101,
									"title":   "Episode 1",
									"images": map[string]any{
										"card": map[string]any{"url": ts.URL + "/thumb.jpg"},
									},
								},
							},
						},
					},
				},
			},
		})
	})
	mux.HandleFunc("/thumb.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte("fake-thumb-data"))
	})
	ts = httptest.NewServer(mux)
	defer ts.Close()

	p := newTestProvider(ts.URL)
	rc, err := p.GetEpisodeThumbnail(t.Context(), "s", "s01", "101")
	if err != nil {
		t.Fatalf("GetEpisodeThumbnail() error: %v", err)
	}
	defer rc.Close()
	data, _ := io.ReadAll(rc)
	if string(data) != "fake-thumb-data" {
		t.Errorf("thumbnail data = %q, want %q", string(data), "fake-thumb-data")
	}
}

func TestSearch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ott/catalog/v2/gem/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"totalRecords": 3,
			"results": []map[string]any{
				{"type": "show", "title": "Schitt's Creek", "url": "schitts-creek", "description": "A show"},
				{"type": "media", "title": "A Movie", "url": "movie-999001", "description": "A film"},
				{"type": "liveevent", "title": "Live News", "url": "live-999002"},
			},
		})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	p := newTestProvider(ts.URL)
	results, total, err := p.Search(t.Context(), "schitt", 10, 0)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}

	// First should be a series
	if results[0].Resource.Type != oas.SeriesSearchResultItemResource {
		t.Errorf("result[0].Type = %q, want Series", results[0].Resource.Type)
	}
	// Second should be a movie
	if results[1].Resource.Type != oas.MovieSearchResultItemResource {
		t.Errorf("result[1].Type = %q, want Movie", results[1].Resource.Type)
	}
	// Third should also be a movie (liveevent maps to Movie)
	if results[2].Resource.Type != oas.MovieSearchResultItemResource {
		t.Errorf("result[2].Type = %q, want Movie", results[2].Resource.Type)
	}
}

func TestSearch_pagination(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ott/catalog/v2/gem/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"totalRecords": 3,
			"results": []map[string]any{
				{"type": "show", "title": "A", "url": "a"},
				{"type": "show", "title": "B", "url": "b"},
				{"type": "show", "title": "C", "url": "c"},
			},
		})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	p := newTestProvider(ts.URL)

	t.Run("limit 2", func(t *testing.T) {
		results, total, err := p.Search(t.Context(), "test", 2, 0)
		if err != nil {
			t.Fatalf("Search() error: %v", err)
		}
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
		if len(results) != 2 {
			t.Fatalf("got %d results, want 2", len(results))
		}
	})

	t.Run("offset 1 limit 1", func(t *testing.T) {
		results, total, err := p.Search(t.Context(), "test", 1, 1)
		if err != nil {
			t.Fatalf("Search() error: %v", err)
		}
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		s, _ := results[0].Resource.GetSeries()
		if s.GetName() != "B" {
			t.Errorf("result name = %q, want %q", s.GetName(), "B")
		}
	})
}

func TestGetMovies(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ott/catalog/v2/gem/category/all-films", func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case page == 1 && pageSize == 2:
			json.NewEncoder(w).Encode(map[string]any{
				"content": []map[string]any{
					{
						"items": map[string]any{
							"totalPages":   2,
							"totalRecords": 3,
							"pageNumber":   1,
							"pageSize":     2,
							"results": []map[string]any{
								{"type": "show", "title": "Film A", "url": "film-a", "description": "Desc A", "infoTitle": "Documentary | 23 min"},
								{"type": "show", "title": "Film B", "url": "film-b", "description": "Desc B", "infoTitle": "Drama | 1 h 32 min"},
							},
						},
					},
				},
			})
		case page == 2 && pageSize == 2:
			json.NewEncoder(w).Encode(map[string]any{
				"content": []map[string]any{
					{
						"items": map[string]any{
							"totalPages":   2,
							"totalRecords": 3,
							"pageNumber":   2,
							"pageSize":     2,
							"results": []map[string]any{
								{"type": "show", "title": "Film C", "url": "film-c", "description": "Desc C", "infoTitle": "Comedy | 45 min"},
							},
						},
					},
				},
			})
		default:
			json.NewEncoder(w).Encode(map[string]any{
				"content": []map[string]any{
					{
						"items": map[string]any{
							"totalPages":   2,
							"totalRecords": 3,
							"pageNumber":   page,
							"pageSize":     pageSize,
							"results":      []map[string]any{},
						},
					},
				},
			})
		}
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	p := newTestProvider(ts.URL)

	t.Run("first page", func(t *testing.T) {
		movies, total, err := p.GetMovies(t.Context(), 2, 0)
		if err != nil {
			t.Fatalf("GetMovies() error: %v", err)
		}
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
		if len(movies) != 2 {
			t.Fatalf("got %d movies, want 2", len(movies))
		}
		if movies[0].GetName() != "Film A" {
			t.Errorf("movies[0].Name = %q, want %q", movies[0].GetName(), "Film A")
		}
		if movies[0].GetID() != "film-a" {
			t.Errorf("movies[0].ID = %q, want %q", movies[0].GetID(), "film-a")
		}
		if movies[0].Duration.Value != "PT23M" {
			t.Errorf("movies[0].Duration = %q, want %q", movies[0].Duration.Value, "PT23M")
		}
		if movies[1].Duration.Value != "PT92M" {
			t.Errorf("movies[1].Duration = %q, want %q", movies[1].Duration.Value, "PT92M")
		}
	})

	t.Run("second page", func(t *testing.T) {
		movies, total, err := p.GetMovies(t.Context(), 2, 2)
		if err != nil {
			t.Fatalf("GetMovies() error: %v", err)
		}
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
		if len(movies) != 1 {
			t.Fatalf("got %d movies, want 1", len(movies))
		}
		if movies[0].GetName() != "Film C" {
			t.Errorf("movies[0].Name = %q, want %q", movies[0].GetName(), "Film C")
		}
		if movies[0].Duration.Value != "PT45M" {
			t.Errorf("movies[0].Duration = %q, want %q", movies[0].Duration.Value, "PT45M")
		}
	})

	t.Run("offset past end", func(t *testing.T) {
		movies, total, err := p.GetMovies(t.Context(), 10, 10)
		if err != nil {
			t.Fatalf("GetMovies() error: %v", err)
		}
		if len(movies) != 0 {
			t.Errorf("got %d movies, want 0", len(movies))
		}
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
	})
}

func TestMovieEndpoints_unsupported(t *testing.T) {
	p := cbc.New(cbc.Config{Tag: "CBC"})

	t.Run("GetMovieById", func(t *testing.T) {
		_, err := p.GetMovieById(t.Context(), "x")
		if err != provider.ErrNotSupported {
			t.Errorf("GetMovieById() error = %v, want ErrNotSupported", err)
		}
	})

	t.Run("GetMovieStreams", func(t *testing.T) {
		_, _, err := p.GetMovieStreams(t.Context(), "x")
		if err != provider.ErrNotSupported {
			t.Errorf("GetMovieStreams() error = %v, want ErrNotSupported", err)
		}
	})

	t.Run("GetMoviePoster", func(t *testing.T) {
		_, err := p.GetMoviePoster(t.Context(), "x")
		if err != provider.ErrNotSupported {
			t.Errorf("GetMoviePoster() error = %v, want ErrNotSupported", err)
		}
	})

	t.Run("GetMovieBackdrop", func(t *testing.T) {
		_, err := p.GetMovieBackdrop(t.Context(), "x")
		if err != provider.ErrNotSupported {
			t.Errorf("GetMovieBackdrop() error = %v, want ErrNotSupported", err)
		}
	})
}

func TestSeriesImages(t *testing.T) {
	var ts *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/ott/catalog/v2/gem/show/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{
					"items": map[string]any{
						"results": []map[string]any{
							{
								"type":  "show",
								"title": "S",
								"url":   "s",
								"images": map[string]any{
									"card":       map[string]any{"url": ts.URL + "/poster.jpg"},
									"background": map[string]any{"url": ts.URL + "/backdrop.jpg"},
								},
							},
						},
					},
				},
			},
		})
	})
	mux.HandleFunc("/poster.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte("poster-data"))
	})
	mux.HandleFunc("/backdrop.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte("backdrop-data"))
	})
	ts = httptest.NewServer(mux)
	defer ts.Close()

	p := newTestProvider(ts.URL)

	t.Run("poster", func(t *testing.T) {
		rc, err := p.GetSeriesPoster(t.Context(), "s")
		if err != nil {
			t.Fatalf("GetSeriesPoster() error: %v", err)
		}
		defer rc.Close()
		data, _ := io.ReadAll(rc)
		if string(data) != "poster-data" {
			t.Errorf("poster data = %q, want %q", string(data), "poster-data")
		}
	})

	t.Run("backdrop", func(t *testing.T) {
		rc, err := p.GetSeriesBackdrop(t.Context(), "s")
		if err != nil {
			t.Fatalf("GetSeriesBackdrop() error: %v", err)
		}
		defer rc.Close()
		data, _ := io.ReadAll(rc)
		if string(data) != "backdrop-data" {
			t.Errorf("backdrop data = %q, want %q", string(data), "backdrop-data")
		}
	})
}

func TestEmptyShow(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ott/catalog/v2/gem/show/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	p := newTestProvider(ts.URL)
	_, err := p.GetSeriesByID(t.Context(), "empty")
	if err != provider.ErrNotSupported {
		t.Errorf("GetSeriesByID() error = %v, want ErrNotSupported", err)
	}
}

func TestUnimplementedMethods(t *testing.T) {
	p := cbc.New(cbc.Config{Tag: "CBC"})

	t.Run("GetMovieStreamFile", func(t *testing.T) {
		_, _, err := p.GetMovieStreamFile(t.Context(), "x", "y")
		if err != provider.ErrNotSupported {
			t.Errorf("error = %v, want ErrNotSupported", err)
		}
	})

	t.Run("GetMovieSubtitleFile", func(t *testing.T) {
		_, _, err := p.GetMovieSubtitleFile(t.Context(), "x", "y")
		if err != provider.ErrNotSupported {
			t.Errorf("error = %v, want ErrNotSupported", err)
		}
	})

	t.Run("GetMovieSubtitles", func(t *testing.T) {
		_, _, err := p.GetMovieSubtitles(t.Context(), "x")
		if err != provider.ErrNotSupported {
			t.Errorf("error = %v, want ErrNotSupported", err)
		}
	})

	t.Run("GetSeasonPoster delegates to SeriesPoster", func(t *testing.T) {
		// Should try to fetch the series images
		_, err := p.GetSeasonPoster(t.Context(), "nonexistent", "s01")
		if err == nil {
			t.Error("expected error for nonexistent series")
		}
	})
}

// showTestServer creates an httptest.Server that responds to
// /ott/catalog/v2/gem/show/{id} with the given JSON body.
func showTestServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/ott/catalog/v2/gem/show/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})
	return httptest.NewServer(mux)
}

// newTestProvider creates a CBC provider pointed at tsURL.
func newTestProvider(tsURL string) *cbc.Provider {
	return cbc.New(cbc.Config{
		Tag:     "CBC",
		Service: &oas.Service{Tag: "CBC", Name: "CBC Gem"},
		BaseURL: tsURL,
	})
}
