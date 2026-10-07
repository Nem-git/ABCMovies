package stremio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// fakeAddon speaks the Stremio addon protocol's three endpoints, so the
// adapter's contract is exercised over the same wire shape as a real addon.
func fakeAddon(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /manifest.json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "org.example.fake", "name": "Fake",
			"types":    []string{"movie"},
			"catalogs": []map[string]any{{"type": "movie", "id": "top", "name": "Top"}},
		})
	})
	mux.HandleFunc("GET /catalog/movie/top.json", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("search")
		metas := []map[string]any{{"id": "tt1", "type": "movie", "name": "Big Buck Bunny", "year": 2008}}
		if q != "" && !strings.Contains(strings.ToLower("Big Buck Bunny"), strings.ToLower(q)) {
			metas = nil
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"metas": metas})
	})
	mux.HandleFunc("GET /meta/movie/tt1.json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"meta": map[string]any{"id": "tt1", "type": "movie", "name": "Big Buck Bunny", "year": 2008}})
	})
	mux.HandleFunc("GET /meta/movie/tt404.json", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("GET /stream/movie/tt1.json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"streams": []map[string]any{{"url": "https://example.test/v.m3u8"}}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newSlot(t *testing.T, srv *httptest.Server) *Slot {
	t.Helper()
	s, err := New([]string{"acct"}, singleAccountSource{ID: "acct", ManifestURL: srv.URL + "/manifest.json"}, WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestBrowseCatalogListsMovies(t *testing.T) {
	s := newSlot(t, fakeAddon(t))
	resp, err := s.BrowseCatalog(context.Background(), &slotsv1.BrowseCatalogRequest{AccountId: "acct"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetItems()) != 1 || resp.GetItems()[0].GetNativeId() != "movie:tt1" {
		t.Fatalf("browse = %v", resp.GetItems())
	}
}

func TestSearchCatalogQueriesTheAddon(t *testing.T) {
	s := newSlot(t, fakeAddon(t))
	resp, err := s.SearchCatalog(context.Background(), &slotsv1.SearchCatalogRequest{AccountId: "acct", Query: "buck"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetItems()) != 1 {
		t.Fatalf("search = %v", resp.GetItems())
	}
	if resp, _ := s.SearchCatalog(context.Background(), &slotsv1.SearchCatalogRequest{AccountId: "acct", Query: "sintel"}); len(resp.GetItems()) != 0 {
		t.Fatalf("search miss = %v", resp.GetItems())
	}
}

func TestRefreshAvailabilityFiltersDelisted(t *testing.T) {
	s := newSlot(t, fakeAddon(t))
	resp, err := s.RefreshAvailability(context.Background(), &slotsv1.RefreshAvailabilityRequest{
		AccountId: "acct", NativeIds: []string{"movie:tt1", "movie:tt404"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetItems()) != 1 || resp.GetItems()[0].GetNativeId() != "movie:tt1" {
		t.Fatalf("refresh = %v", resp.GetItems())
	}
}

func TestProduceSourcesResolvesStreams(t *testing.T) {
	s := newSlot(t, fakeAddon(t))
	resp, err := s.ProduceSources(context.Background(), &slotsv1.ProduceSourcesRequest{AccountId: "acct", NativeId: "movie:tt1"})
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.GetSource().GetTracks()[0].GetDelivery().GetLocations()[0]; got != "https://example.test/v.m3u8" {
		t.Fatalf("location = %q", got)
	}
}

func TestCatalogueSyncRefused(t *testing.T) {
	s := newSlot(t, fakeAddon(t))
	if _, err := s.CatalogueSync(context.Background(), &slotsv1.CatalogueSyncRequest{AccountId: "acct"}); err == nil {
		t.Fatal("lazy provider must refuse catalogue sync")
	}
}
