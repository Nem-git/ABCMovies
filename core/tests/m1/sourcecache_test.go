// Package m1 holds the milestone acceptance tests for M1: one library-class
// provider adapter plus the account source cache (docs/IMPLEMENTATION.md §3).
// The acceptance criteria are proven through the seams that ship: a real
// adapter (jellyfin) pages its whole catalogue over the sync contract, the
// real source-cache synchronizer validates and lands every page, a fresh
// instance rebuilds the catalogue from the provider, and the slot registry
// surfaces the adapter's declared capability versions.
//
// "Adapter passes its fixture suite" lives in the gate itself: the provider
// handshake fixtures (fixtures/provider/v1) and the adapters' own suites run
// in `make check`. Providers are mocked behind an HTTP seam per docs/TESTING.md;
// a live Jellyfin server is never contacted.
package m1_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nem-git/abcmovies/adapters/jellyfin"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/registry"
	"github.com/nem-git/abcmovies/core/internal/sourcecache"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// jellyfinItem is the subset of Jellyfin's BaseItemDto the adapter consumes,
// mirrored here so the fake speaks the same wire shape as the real server.
type jellyfinItem struct {
	Id             string            `json:"Id"`
	Type           string            `json:"Type"`
	Name           string            `json:"Name"`
	ProductionYear int               `json:"ProductionYear"`
	ProviderIds    map[string]string `json:"ProviderIds"`
}

// fakeJellyfin is a minimal in-process Jellyfin index: it authenticates one
// user and serves a mutable item index with offset pagination.
type fakeJellyfin struct {
	server *httptest.Server
	items  []jellyfinItem
}

func newFakeJellyfin(t *testing.T, items []jellyfinItem) *fakeJellyfin {
	t.Helper()
	f := &fakeJellyfin{items: items}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /Users/AuthenticateByName", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string
			Pw       string
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"AccessToken": "test-access-token",
			"User":        map[string]any{"Id": "user-1"},
		})
	})
	mux.HandleFunc("GET /Items", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("fields") != "ProviderIds" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		offset := queryInt(r, "startIndex")
		limit := queryInt(r, "limit")
		page := []jellyfinItem{}
		if offset < len(f.items) {
			page = f.items[offset:min(offset+limit, len(f.items))]
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items":            page,
			"TotalRecordCount": len(f.items),
			"StartIndex":       offset,
		})
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func queryInt(r *http.Request, key string) int {
	var v int
	_, _ = fmt.Sscanf(r.URL.Query().Get(key), "%d", &v)
	return v
}

const m1TestPasswordEnv = "JELLYFIN_TEST_PASSWORD"

// newM1Slot builds a real jellyfin adapter pointed at the fake server. The
// password comes from the environment, exactly as production resolves it.
func newM1Slot(t *testing.T, f *fakeJellyfin) *jellyfin.Slot {
	t.Helper()
	t.Setenv(m1TestPasswordEnv, "sekret")
	slot, err := jellyfin.New([]jellyfin.Account{{
		ID:          "primary",
		URL:         f.server.URL,
		Username:    "bob",
		PasswordEnv: m1TestPasswordEnv,
	}}, jellyfin.WithHTTPClient(f.server.Client()))
	if err != nil {
		t.Fatalf("jellyfin.New: %v", err)
	}
	return slot
}

// makeCatalogue yields n movies, each asserting an IMDb id.
func makeCatalogue(n int) []jellyfinItem {
	out := make([]jellyfinItem, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, jellyfinItem{
			Id:             fmt.Sprintf("item-%04d", i),
			Type:           "Movie",
			Name:           fmt.Sprintf("Film %04d", i),
			ProductionYear: 1990 + i%35,
			ProviderIds:    map[string]string{"IMDb": fmt.Sprintf("tt%070d", i)},
		})
	}
	return out
}

// nativeIDs projects a cached catalogue to its native ids.
func nativeIDs(items []*slotsv1.CatalogueItem) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, it := range items {
		out[it.GetNativeId()] = struct{}{}
	}
	return out
}

// TestM1WholeCatalogueLandsInAccountSourceCache proves the first half of the
// acceptance: a real adapter pages its whole catalogue across the page
// boundary (520 items, adapter page size 500) and the real synchronizer lands
// it — validated item-by-item against the contract — in the account source
// cache, with a completion manifest that marks the account fully synced.
func TestM1WholeCatalogueLandsInAccountSourceCache(t *testing.T) {
	items := makeCatalogue(520)
	f := newFakeJellyfin(t, items)
	slot := newM1Slot(t, f)

	sync, err := sourcecache.New("jellyfin", slot, store.NewInMemory(), slog.Default())
	if err != nil {
		t.Fatalf("sourcecache.New: %v", err)
	}
	stats, err := sync.SyncAccount(context.Background(), "primary")
	if err != nil {
		t.Fatalf("SyncAccount: %v", err)
	}
	if stats.Items != len(items) {
		t.Fatalf("synced %d items, want %d", stats.Items, len(items))
	}
	if stats.Pages < 2 {
		t.Fatalf("catalogue fit in %d page(s), the 520-item catalogue must cross the adapter's page boundary", stats.Pages)
	}
	manifest, ok, err := sync.Manifest(context.Background(), "primary")
	if err != nil || !ok {
		t.Fatalf("manifest: ok=%v err=%v", ok, err)
	}
	if manifest.Items != len(items) {
		t.Fatalf("manifest items = %d, want %d", manifest.Items, len(items))
	}
	if manifest.LastCompleteSync.IsZero() {
		t.Fatal("manifest must carry the completion timestamp")
	}

	got, err := sync.ListItems(context.Background(), "primary")
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if gotIDs := nativeIDs(got); len(gotIDs) != len(items) {
		t.Fatalf("cached %d items, want %d: %v", len(gotIDs), len(items), gotIDs)
	}
	// Identity assertions survive the whole path, lower-cased by the adapter.
	var withIMDb int
	for _, it := range got {
		if it.GetMetadata().GetTitle() == "Film 0000" {
			if it.GetKind() != slotsv1.ItemKind_ITEM_KIND_MOVIE {
				t.Errorf("Film 0000 kind = %v, want movie", it.GetKind())
			}
		}
		for _, id := range it.GetExternalIds() {
			if id.GetNamespace() == "imdb" {
				withIMDb++
			}
		}
	}
	if withIMDb != len(items) {
		t.Fatalf("%d items carry an imdb claim, want %d", withIMDb, len(items))
	}
}

// TestM1CacheRebuildsFromProvider proves the second half of the acceptance:
// a cache rebuilds purely from the provider. A wiped instance (a brand-new
// store) reconstructs the identical catalogue from provider pages alone, and
// the next sync reconciles titles the provider no longer reports.
func TestM1CacheRebuildsFromProvider(t *testing.T) {
	items := makeCatalogue(4)
	f := newFakeJellyfin(t, items)
	slot := newM1Slot(t, f)

	sync1, err := sourcecache.New("jellyfin", slot, store.NewInMemory(), slog.Default())
	if err != nil {
		t.Fatalf("sourcecache.New: %v", err)
	}
	if _, err := sync1.SyncAccount(context.Background(), "primary"); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	first, err := sync1.ListItems(context.Background(), "primary")
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}

	// A fresh instance starts from an empty store: the catalogue must be
	// reconstructed from the provider, never from leftover cache state.
	sync2, err := sourcecache.New("jellyfin", slot, store.NewInMemory(), slog.Default())
	if err != nil {
		t.Fatalf("sourcecache.New: %v", err)
	}
	if _, err := sync2.SyncAccount(context.Background(), "primary"); err != nil {
		t.Fatalf("rebuild sync: %v", err)
	}
	second, err := sync2.ListItems(context.Background(), "primary")
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	a, b := nativeIDs(first), nativeIDs(second)
	if len(a) != len(b) {
		t.Fatalf("rebuilt catalogue size differs: %d vs %d", len(a), len(b))
	}
	for id := range a {
		if _, ok := b[id]; !ok {
			t.Fatalf("rebuild lost %q", id)
		}
	}

	// Deletions reconcile: the provider drops one title; the next sync on the
	// original store removes it from the cache.
	f.items = items[1:]
	stats, err := sync1.SyncAccount(context.Background(), "primary")
	if err != nil {
		t.Fatalf("delete-reconcile sync: %v", err)
	}
	if stats.Removed != 1 {
		t.Fatalf("removed %d items, want 1", stats.Removed)
	}
	after, err := sync1.ListItems(context.Background(), "primary")
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if _, stillThere := nativeIDs(after)[items[0].Id]; stillThere {
		t.Fatalf("departed item %q still in cache", items[0].Id)
	}
}

// TestM1RegistryShowsCapabilityVersions proves the registry half of the
// acceptance: admitting the adapter makes its declared capability versions
// visible — meta v1, browse v1, and produce-sources v1 — to the operator
// surface (PLAN.md §3.3: nothing is assumed, everything is asked).
func TestM1RegistryShowsCapabilityVersions(t *testing.T) {
	f := newFakeJellyfin(t, nil)
	slot := newM1Slot(t, f)

	reg := registry.NewInProcess()
	defer reg.Close()

	caps, err := reg.Admit("jellyfin", slot)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}
	got := map[string]uint32{}
	for _, c := range caps {
		got[c.Name] = c.Version
	}
	if got["meta"] != 1 || got["browse"] != 1 || got["produce-sources"] != 1 {
		t.Fatalf("admitted capabilities = %v, want meta/browse/produce-sources v1", got)
	}

	again, ok := reg.Capabilities("jellyfin")
	if !ok || len(again) != 3 {
		t.Fatalf("Capabilities(jellyfin) = %v ok=%v", again, ok)
	}
	snap := reg.Snapshot()
	info, ok := snap["jellyfin"]
	if !ok {
		t.Fatalf("snapshot lacks the jellyfin slot; got %d slots", len(snap))
	}
	if info.Capabilities[0].Name != "meta" || info.Capabilities[0].Version != 1 {
		t.Fatalf("snapshot capabilities = %v", info.Capabilities)
	}

	// The handshake verifies freshness: the registry re-asks the adapter, so
	// the declared cadence policy is visible too.
	if gotPolicy, ok := reg.Policy("jellyfin"); !ok || gotPolicy["browse.sync-cadence"] == "" {
		t.Fatalf("sync-cadence policy not surfaced: %v ok=%v", gotPolicy, ok)
	}
}
