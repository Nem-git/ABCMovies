package tmdb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// newFixture spins an adapter against a canned mux; no test ever touches
// the live API (TECHNICAL-DECISIONS.md §1.27 consequence).
func newFixture(t *testing.T, handle func(w http.ResponseWriter, r *http.Request)) *Slot {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(handle))
	t.Cleanup(srv.Close)
	t.Setenv("TMDB_TOKEN_TEST", "secret-token")
	s, err := New("TMDB_TOKEN_TEST",
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithPace(time.Nanosecond),
	)
	if err != nil {
		t.Fatalf("new slot: %v", err)
	}
	return s
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
}

func TestNewFailsLoudlyWithoutCredential(t *testing.T) {
	if _, err := New("TMDB_TOKEN_DEFINITELY_UNSET"); err == nil {
		t.Fatal("missing token accepted")
	}
	if _, err := New(""); err == nil {
		t.Fatal("empty token-env name accepted")
	}
}

func TestLookupTitleMapsMovieResults(t *testing.T) {
	var gotQuery, gotYear, gotLang string
	s := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/search/movie") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		q := r.URL.Query()
		gotQuery, gotYear, gotLang = q.Get("query"), q.Get("year"), q.Get("language")
		writeJSON(t, w, map[string]any{
			"results": []map[string]any{{
				"id": 11576, "title": "The Thing", "original_title": "The Thing",
				"release_date": "1982-06-25", "imdb_id": "tt0084787",
			}},
		})
	})
	resp, err := s.LookupTitle(context.Background(), &slotsv1.LookupTitleRequest{
		Query: "The Thing", Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE, Year: 1982,
	})
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if gotQuery != "The Thing" || gotYear != "1982" || gotLang != language {
		t.Fatalf("search params = %q %q %q", gotQuery, gotYear, gotLang)
	}
	if len(resp.Candidates) != 1 {
		t.Fatalf("candidates = %d, want 1", len(resp.Candidates))
	}
	c := resp.Candidates[0]
	if c.Ref != "tmdb:11576" || c.Title != "The Thing" || c.Year != 1982 ||
		c.Kind != slotsv1.ItemKind_ITEM_KIND_MOVIE || len(c.ExternalIds) != 1 ||
		c.ExternalIds[0].Namespace+":"+c.ExternalIds[0].Value != "imdb:tt0084787" {
		t.Fatalf("candidate mapping wrong: %+v", c)
	}
}

func TestLookupTitleMultiSkipsPeople(t *testing.T) {
	s := newFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{
			"results": []map[string]any{
				{"media_type": "person", "id": 5081, "name": "Kurt Russell"},
				{"media_type": "tv", "id": 42, "name": "Dark", "first_air_date": "2017-12-01"},
			},
		})
	})
	resp, err := s.LookupTitle(context.Background(), &slotsv1.LookupTitleRequest{Query: "dark"})
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if len(resp.Candidates) != 1 || resp.Candidates[0].Kind != slotsv1.ItemKind_ITEM_KIND_SERIES ||
		resp.Candidates[0].GetRef() != "tmdb:42" {
		t.Fatalf("multi mapping wrong: %+v", resp.Candidates)
	}
}

func TestGetMetadataNativeMovieRecord(t *testing.T) {
	s := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/movie/603"):
			if r.URL.RawQuery == "" {
				writeJSON(t, w, map[string]any{"success": true}) // native-ref existence probe
				return
			}
			if q := r.URL.Query().Get("append_to_response"); q != "external_ids,credits,release_dates" {
				t.Errorf("append_to_response = %q", q)
			}
			writeJSON(t, w, map[string]any{
				"id": 603, "title": "The Matrix", "overview": "A computer hacker…",
				"release_date": "1999-03-31", "vote_average": 8.2, "poster_path": "/abc.jpg",
				"original_language": "en", "runtime": 136,
				"credits": map[string]any{
					"cast": []map[string]any{{"name": "Keanu Reeves", "order": 0}},
					"crew": []map[string]any{
						{"name": "Lana Wachowski", "job": "Director"},
						{"name": "Lilly Wachowski", "job": "Director"},
					},
				},
				"external_ids": map[string]any{
					"imdb_id": "tt0133093", "wikidata_id": "Q83495", "tvdb_id": nil,
				},
				"release_dates": map[string]any{
					"results": []map[string]any{{
						"iso_3166_1":    "US",
						"release_dates": []map[string]any{{"certification": "R"}},
					}},
				},
			})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})
	resp, err := s.GetMetadata(context.Background(), &slotsv1.GetMetadataRequest{Ref: "tmdb:603"})
	if err != nil {
		t.Fatalf("get-metadata: %v", err)
	}
	md := resp.Metadata
	if md.Title != "The Matrix" || md.Year != 1999 || md.Rating < 8.19 || md.Rating > 8.21 ||
		md.PosterUrl != imageBase+posterSize+"/abc.jpg" || md.ContentRating != "R" ||
		len(md.Directors) != 2 || len(md.Cast) != 1 {
		t.Fatalf("metadata mapping wrong: %+v", md)
	}
	mov := md.GetMovie()
	if mov == nil || mov.RuntimeMinutes != 136 {
		t.Fatalf("movie-specific mapping wrong: %+v", md.GetKindSpecific())
	}
	ns := map[string]string{}
	for _, id := range resp.ExternalIds {
		ns[id.Namespace] = id.Value
	}
	if ns["tmdb"] != "603" || ns["imdb"] != "tt0133093" || ns["wikidata"] != "Q83495" {
		t.Fatalf("external ids wrong: %v", ns)
	}
}

func TestGetMetadataForeignIDBridgesThroughFind(t *testing.T) {
	s := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/find/tt0133093"):
			if r.URL.Query().Get("external_source") != "imdb_id" {
				t.Errorf("external_source = %q", r.URL.Query().Get("external_source"))
			}
			writeJSON(t, w, map[string]any{
				"tv_results": []map[string]any{{"id": 94605}},
			})
		case strings.HasPrefix(r.URL.Path, "/tv/94605"):
			writeJSON(t, w, map[string]any{
				"id": 94605, "name": "Severance", "first_air_date": "2022-02-18",
				"number_of_seasons": 2, "number_of_episodes": 19,
			})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})
	resp, err := s.GetMetadata(context.Background(), &slotsv1.GetMetadataRequest{Ref: "imdb:tt0133093"})
	if err != nil {
		t.Fatalf("get-metadata: %v", err)
	}
	if resp.Metadata.Title != "Severance" || resp.Metadata.GetSeries() == nil {
		t.Fatalf("bridged record wrong: %+v", resp.Metadata)
	}
}

func TestGetMetadataRejectsUnknownNamespacesAndMalformedRefs(t *testing.T) {
	s := newFixture(t, func(w http.ResponseWriter, _ *http.Request) {})
	for _, tc := range []struct{ ref, want string }{
		{"trakt:123", "unknown namespace"},
		{"justanid", "not namespace:value"},
	} {
		if _, err := s.GetMetadata(context.Background(), &slotsv1.GetMetadataRequest{Ref: tc.ref}); err == nil ||
			!strings.Contains(err.Error(), tc.want) {
			t.Fatalf("ref %q: want %q error, got %v", tc.ref, tc.want, err)
		}
	}
}

func TestRateLimitBacksOffOncePerRetryAfter(t *testing.T) {
	attempts := 0
	s := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeJSON(t, w, map[string]any{"results": []map[string]any{}})
	})
	start := time.Now()
	if _, err := s.LookupTitle(context.Background(), &slotsv1.LookupTitleRequest{Query: "x"}); err != nil {
		t.Fatalf("retry did not recover: %v", err)
	}
	if waited := time.Since(start); waited < time.Second {
		t.Fatalf("Retry-After ignored; returned after %v", waited)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want exactly one retry", attempts)
	}
}
