package library

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/itemregistry"
	"github.com/nem-git/abcmovies/core/internal/sourcecache"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// fakeProvider serves canned pages; pages can be swapped between syncs to
// simulate upstream catalogue changes.
type fakeProvider struct {
	pages []*slotsv1.CatalogueSyncResponse
}

func (f *fakeProvider) CatalogueSync(_ context.Context, req *slotsv1.CatalogueSyncRequest) (*slotsv1.CatalogueSyncResponse, error) {
	idx := 0
	if req.GetPageToken() != "" {
		if _, err := fmt.Sscanf(req.GetPageToken(), "p%d", &idx); err != nil {
			return nil, fmt.Errorf("bad page token")
		}
	}
	if idx >= len(f.pages) {
		return nil, fmt.Errorf("page out of range")
	}
	return f.pages[idx], nil
}

func movie(id, title string, year uint32, ids ...*slotsv1.ExternalId) *slotsv1.CatalogueItem {
	return &slotsv1.CatalogueItem{
		NativeId:    id,
		Kind:        slotsv1.ItemKind_ITEM_KIND_MOVIE,
		Metadata:    &corev1.TitleMetadata{Title: title, Year: year},
		ExternalIds: ids,
	}
}

func imdb(v string) *slotsv1.ExternalId { return &slotsv1.ExternalId{Namespace: "imdb", Value: v} }

// bridgeSink records envelopes and forwards them to the service once wired,
// mirroring how main routes availability events after SetupAll.
type bridgeSink struct {
	envs []*corev1.EventEnvelope
	svc  *Service
}

func (b *bridgeSink) Publish(e *corev1.EventEnvelope) {
	b.envs = append(b.envs, e)
	if b.svc != nil {
		b.svc.Publish(e)
	}
}

// resolveVia adapts the registry to the synchronizer's ItemResolver.
type resolveVia struct{ r *itemregistry.Registry }

func (a resolveVia) Resolve(ctx context.Context, provider string, item *slotsv1.CatalogueItem) error {
	_, err := a.r.Resolve(ctx, provider, item)
	return err
}

// fixture wires one provider ("jellyfin") with two accounts over a shared
// store: registry mappings and source-cache rows live side by side under
// disjoint key prefixes.
type fixture struct {
	svc   *Service
	reg   *itemregistry.Registry
	cache store.Store
	sink  *bridgeSink
	f1    *fakeProvider
	f2    *fakeProvider
	acct1 *sourcecache.Synchronizer
	acct2 *sourcecache.Synchronizer
}

func newFixture(t *testing.T, withResolver bool) *fixture {
	t.Helper()
	cache := store.NewInMemory()
	reg, err := itemregistry.New(cache, "")
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	fx := &fixture{
		reg:   reg,
		cache: cache,
		sink:  &bridgeSink{},
		f1: &fakeProvider{pages: []*slotsv1.CatalogueSyncResponse{{Items: []*slotsv1.CatalogueItem{
			movie("jf1", "The Matrix", 1999, imdb("tt0133093")),
		}}}},
		f2: &fakeProvider{pages: []*slotsv1.CatalogueSyncResponse{{Items: []*slotsv1.CatalogueItem{
			movie("jf9", "the MATRIX", 1999, imdb("tt0133093")),
			movie("jf5", "Up", 2009),
		}}}},
	}
	mk := func(f *fakeProvider) *sourcecache.Synchronizer {
		opts := []sourcecache.Option{sourcecache.WithEntryLookup(reg), sourcecache.WithEventsSink(fx.sink)}
		if withResolver {
			opts = append(opts, sourcecache.WithItemResolver(resolveVia{reg}))
		}
		s, err := sourcecache.New("jellyfin", f, cache, slog.Default(), opts...)
		if err != nil {
			t.Fatalf("sourcecache: %v", err)
		}
		return s
	}
	fx.acct1 = mk(fx.f1)
	fx.acct2 = mk(fx.f2)

	svc, err := NewService([]Reach{{fx.acct1, "acct-1"}, {fx.acct2, "acct-2"}}, reg, cache, slog.Default())
	if err != nil {
		t.Fatalf("service: %v", err)
	}
	fx.sink.svc = svc
	fx.svc = svc
	return fx
}

func (fx *fixture) syncAll(t *testing.T) {
	t.Helper()
	if _, err := fx.acct1.SyncAccount(context.Background(), "acct-1"); err != nil {
		t.Fatalf("sync acct-1: %v", err)
	}
	if _, err := fx.acct2.SyncAccount(context.Background(), "acct-2"); err != nil {
		t.Fatalf("sync acct-2: %v", err)
	}
}

func find(t *testing.T, entries []*corev1.LibraryEntry, coverKey string) *corev1.LibraryEntry {
	t.Helper()
	for _, e := range entries {
		if _, ok := e.GetCoverage()[coverKey]; ok {
			return e
		}
	}
	t.Fatalf("no entry carries coverage %q in %d entries", coverKey, len(entries))
	return nil
}

func TestDerivesMergedLibraryFromTwoAccounts(t *testing.T) {
	fx := newFixture(t, true)
	fx.syncAll(t)

	entries, err := fx.svc.Library(context.Background(), "alice")
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("library has %d entries, want the merged Matrix plus Up", len(entries))
	}

	matrix := find(t, entries, "jellyfin:jf1")
	if got := len(matrix.GetCoverage()); got != 2 {
		t.Fatalf("Matrix coverage rows = %d, want both accounts merged into one entry", got)
	}
	row := matrix.GetCoverage()["jellyfin:jf9"]
	if !row.GetPresent() || row.GetVerdict() != corev1.CoverageVerdict_COVERAGE_VERDICT_CORROBORATED {
		t.Fatalf("second account's row = %+v, want present and corroborated by the imdb claim", row)
	}
	if via := row.GetVia(); len(via) != 1 || via[0] != "acct-2:jellyfin:host" {
		t.Fatalf("via = %v, want the single observing account", via)
	}
	if len(matrix.GetExternalIdentities()) != 1 || matrix.GetExternalIdentities()[0].GetNamespace() != "imdb" ||
		matrix.GetExternalIdentities()[0].GetVerdict() != corev1.IdentityVerdict_IDENTITY_VERDICT_CORROBORATED {
		t.Fatalf("identities = %+v", matrix.GetExternalIdentities())
	}

	up := find(t, entries, "jellyfin:jf5")
	upRow := up.GetCoverage()["jellyfin:jf5"]
	if upRow.GetVerdict() != corev1.CoverageVerdict_COVERAGE_VERDICT_PRESENCE_ONLY || len(up.GetExternalIdentities()) != 0 {
		t.Fatalf("id-less title must be presence-only without identities: %+v %+v", upRow, up.GetExternalIdentities())
	}
}

func TestAvailabilityEventInvalidatesDerivedCache(t *testing.T) {
	fx := newFixture(t, true)
	fx.syncAll(t)
	ctx := context.Background()

	if _, err := fx.svc.Library(ctx, "alice"); err != nil {
		t.Fatalf("first view: %v", err)
	}

	// The Matrix leaves acct-1; the sync publishes an availability event that
	// the service itself consumes as its invalidation trigger.
	fx.f1.pages = []*slotsv1.CatalogueSyncResponse{{Items: []*slotsv1.CatalogueItem{}}}
	before := len(fx.sink.envs)
	if _, err := fx.acct1.SyncAccount(ctx, "acct-1"); err != nil {
		t.Fatalf("departure sync: %v", err)
	}
	if len(fx.sink.envs) == before {
		t.Fatal("departure emitted no availability events")
	}

	entries, err := fx.svc.Library(ctx, "alice") // lazy rebuild after invalidation
	if err != nil {
		t.Fatalf("post-invalidation view: %v", err)
	}
	for _, e := range entries {
		if _, ok := e.GetCoverage()["jellyfin:jf1"]; ok {
			t.Fatalf("departed item still cached in the derived library: %+v", e.GetCoverage())
		}
	}
	// The Matrix remains reachable through acct-2's identical assertion; the
	// entry survives with its remaining coverage row.
	if e := find(t, entries, "jellyfin:jf9"); len(e.GetCoverage()) != 1 {
		t.Fatalf("surviving Matrix entry should carry exactly acct-2 now: %+v", e.GetCoverage())
	}
}

func TestUnresolvedItemsAreSkippedWithoutFailing(t *testing.T) {
	// No resolver wired: the cache fills, identity work never runs.
	fx := newFixture(t, false)
	fx.syncAll(t)

	entries, err := fx.svc.Library(context.Background(), "alice")
	if err != nil {
		t.Fatalf("Library must tolerate unmapped items: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("unmapped items contributed entries: %d", len(entries))
	}
}

func TestSharedItemAcrossTwoAccountsRecordsBothObservers(t *testing.T) {
	fx := newFixture(t, true)
	// acct-1 also carries jf9, the item acct-2 has; one coverage row must
	// end up listing both observing accounts.
	fx.f1.pages = []*slotsv1.CatalogueSyncResponse{{Items: []*slotsv1.CatalogueItem{
		movie("jf1", "The Matrix", 1999, imdb("tt0133093")),
		movie("jf9", "The Matrix", 1999, imdb("tt0133093")),
	}}}
	fx.syncAll(t)

	entries, err := fx.svc.Library(context.Background(), "alice")
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	matrix := find(t, entries, "jellyfin:jf9")
	row := matrix.GetCoverage()["jellyfin:jf9"]
	want := []string{"acct-1:jellyfin:host", "acct-2:jellyfin:host"}
	if got := row.GetVia(); len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("via = %v, want both observers in deterministic order %v", got, want)
	}
}

func TestSameAdapterTwoSlotsDoNotCollide(t *testing.T) {
	fx := newFixture(t, true)
	fx.syncAll(t)
	ctx := context.Background()

	// A second deployment of the same adapter is a different slot: its items
	// live in their own namespace even when native ids coincide. Here "jf1"
	// on slotB is an entirely different film.
	slotB, err := sourcecache.New("server-b", &fakeProvider{pages: []*slotsv1.CatalogueSyncResponse{{Items: []*slotsv1.CatalogueItem{
		movie("jf1", "Persona", 1966),
	}}}}, fx.cache, slog.Default(),
		sourcecache.WithEntryLookup(fx.reg), sourcecache.WithItemResolver(resolveVia{fx.reg}))
	if err != nil {
		t.Fatalf("slotB: %v", err)
	}
	if _, err := slotB.SyncAccount(ctx, "acct-b"); err != nil {
		t.Fatalf("slotB sync: %v", err)
	}

	idA, okA, err := fx.reg.Lookup(ctx, "jellyfin", "jf1")
	if err != nil || !okA {
		t.Fatalf("slotA mapping: %v ok=%v", err, okA)
	}
	idB, okB, err := fx.reg.Lookup(ctx, "server-b", "jf1")
	if err != nil || !okB {
		t.Fatalf("slotB mapping: %v ok=%v", err, okB)
	}
	if idA == idB {
		t.Fatal("identical native ids on two slots of one adapter must not share a mapping")
	}
	canonB, ok, err := fx.reg.Canonical(ctx, idB)
	if err != nil || !ok || canonB.Title != "Persona" {
		t.Fatalf("slotB canonical = %+v ok=%v err=%v", canonB, ok, err)
	}
	// And the first slot's identity state was untouched by the collision
	// that would have occurred under adapter-name scoping.
	canonA, ok, err := fx.reg.Canonical(ctx, idA)
	if err != nil || !ok || canonA.Title != "The Matrix" {
		t.Fatalf("slotA canonical = %+v ok=%v err=%v", canonA, ok, err)
	}
}
