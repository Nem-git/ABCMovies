package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/library"
	"github.com/nem-git/abcmovies/core/internal/registry"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// fakeAddonServer implements the minimal Stremio addon wire shape for a
// composed-slot LiveSearch test: one catalog, one meta entry, one stream.
func fakeAddonServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /manifest.json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "org.test.addon", "name": "TestAddon",
			"types":    []string{"movie"},
			"catalogs": []map[string]any{{"type": "movie", "id": "top", "name": "Top"}},
		})
	})
	mux.HandleFunc("GET /catalog/movie/top.json", func(w http.ResponseWriter, r *http.Request) {
		metas := []map[string]any{}
		if r.URL.Query().Get("search") == "buck" {
			metas = append(metas, map[string]any{"id": "tt_buck", "type": "movie", "name": "Big Buck Bunny", "year": 2008})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"metas": metas})
	})
	mux.HandleFunc("GET /meta/movie/tt_buck.json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"meta": map[string]any{
			"id": "tt_buck", "type": "movie", "name": "Big Buck Bunny", "year": 2008,
		}})
	})
	mux.HandleFunc("GET /stream/movie/tt_buck.json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"streams": []map[string]any{{"url": "https://example.test/buck.mp4"}}})
	})
	s := httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

func TestLiveSearchComposedEndToEnd(t *testing.T) {
	ctx := context.Background()
	fsrv := fakeAddonServer(t)

	reg := registry.NewInProcess()
	defer reg.Close()
	sourceCache := store.NewInMemory()
	metaCache := store.NewInMemory()
	vault := store.NewInMemory()
	cache := store.NewInMemory()
	bus := apiserver.NewInMemoryBus()

	cfg := config.SlotsConfig{Providers: []config.SlotEntry{{
		Adapter: "stremio", ID: "stremio-test", Enabled: true,
		Server:   fsrv.URL + "/manifest.json",
		Accounts: []config.AccountConfig{{ID: "archiveorg"}},
	}}}
	rt, err := ComposeSlots(ctx, cfg, config.EnrichmentConfig{}, config.LibraryConfig{}, reg, sourceCache, metaCache, vault, cache, bus, nil)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	defer rt.Bus.Close()

	searcher := &liveSearcher{built: rt.Providers, library: rt.Library, log: nil}
	resp, err := searcher.Run(ctx, &apiv1.LiveSearchRequest{Query: "buck"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(resp.GetHits()) != 1 || resp.GetHits()[0].GetNativeId() != "movie:tt_buck" {
		t.Fatalf("hits = %+v", resp.GetHits())
	}
	if len(resp.GetRejected()) != 0 {
		t.Fatalf("rejected = %v", resp.GetRejected())
	}

	// The ingested item landed in the source cache.
	_, err = sourceCache.Get(ctx, "stremio-test/archiveorg/movie:tt_buck")
	if err != nil {
		t.Fatalf("source cache missing the ingested item: %v", err)
	}

	// And the derived library reflects it for the public operator account.
	libSvc, err := library.NewService(rt.Library.Reaches(), rt.ItemRegistry, cache, nil)
	if err != nil {
		t.Fatalf("new library service: %v", err)
	}
	lib, err := libSvc.Library(ctx, "")
	if err != nil {
		t.Fatalf("derive library: %v", err)
	}
	if len(lib) != 1 {
		t.Fatalf("derived library entries = %d, want 1", len(lib))
	}
	found := false
	for _, row := range lib[0].GetCoverage() {
		if row.GetPresent() {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected at least one coverage row with present=true, got %+v", lib[0].GetCoverage())
	}
}
