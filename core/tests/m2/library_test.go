// Package m2 holds the milestone acceptance tests for M2 (LibraryEntry +
// matching + merge + provider item registry). They run the production
// pipeline — source cache → item registry → derived per-user library —
// through the same seams slotwiring wires in main.go, covering the criteria
// the data fixtures cannot express: caching, invalidation, and recycled-ID
// conflict emission end to end.
package m2_test

import (
	"context"
	"log/slog"
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/itemregistry"
	"github.com/nem-git/abcmovies/core/internal/library"
	"github.com/nem-git/abcmovies/core/internal/sourcecache"
	"github.com/nem-git/abcmovies/core/internal/store"
)

type pages struct{ items []*slotsv1.CatalogueItem }

func (p *pages) CatalogueSync(_ context.Context, _ *slotsv1.CatalogueSyncRequest) (*slotsv1.CatalogueSyncResponse, error) {
	return &slotsv1.CatalogueSyncResponse{Items: p.items}, nil
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

func withDirectors(m *slotsv1.CatalogueItem, directors ...string) *slotsv1.CatalogueItem {
	m.GetMetadata().Directors = directors
	return m
}

// recordingResolver resolves into the registry and keeps every outcome and
// emitted envelope for inspection.
type recordingResolver struct {
	r      *itemregistry.Registry
	events []*corev1.EventEnvelope
}

func (a *recordingResolver) Resolve(ctx context.Context, provider string, item *slotsv1.CatalogueItem) error {
	out, err := a.r.Resolve(ctx, provider, item)
	if err != nil {
		return err
	}
	a.events = append(a.events, out.Events...)
	return nil
}

// stack is the M2 pipeline wired the way main.go wires it, with knobs the
// tests need to observe behavior.
type stack struct {
	reg     *itemregistry.Registry
	cache   store.Store
	lib     *library.Service
	sink    *captureSink
	provA   *pages
	provB   *pages
	syncA   *sourcecache.Synchronizer
	syncB   *sourcecache.Synchronizer
	resolve *recordingResolver
}

type captureSink struct {
	envs []*corev1.EventEnvelope
}

func (s *captureSink) Publish(e *corev1.EventEnvelope) { s.envs = append(s.envs, e) }

func newStack(t *testing.T) *stack {
	t.Helper()
	st := store.NewInMemory()
	reg, err := itemregistry.New(st, "core")
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	stk := &stack{
		reg:     reg,
		cache:   st,
		sink:    &captureSink{},
		provA:   &pages{},
		provB:   &pages{},
		resolve: &recordingResolver{r: reg},
	}
	mk := func(provider string, p *pages) *sourcecache.Synchronizer {
		syncer, err := sourcecache.New(provider, p, st, slog.Default(),
			sourcecache.WithEntryLookup(reg),
			sourcecache.WithItemResolver(stk.resolve),
			sourcecache.WithEventsSink(stk.sink))
		if err != nil {
			t.Fatalf("sourcecache %q: %v", provider, err)
		}
		return syncer
	}
	syncA := mk("slot-a", stk.provA)
	syncB := mk("slot-b", stk.provB)
	stk.syncA, stk.syncB = syncA, syncB
	svc, err := library.NewService([]library.Reach{
		{Sync: syncA, AccountID: "acct-a", Visibility: accounts.VisibilityPublic},
		{Sync: syncB, AccountID: "acct-b", Visibility: accounts.VisibilityPublic},
	}, reg, st, slog.Default())
	if err != nil {
		t.Fatalf("library service: %v", err)
	}
	for _, e := range stk.sink.envs {
		svc.Publish(e)
	}
	stk.lib = svc
	return stk
}

func (stk *stack) syncAll(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	if _, err := stk.syncA.SyncAccount(ctx, "acct-a"); err != nil {
		t.Fatalf("sync acct-a: %v", err)
	}
	if _, err := stk.syncB.SyncAccount(ctx, "acct-b"); err != nil {
		t.Fatalf("sync acct-b: %v", err)
	}
}

func find(t *testing.T, entries []*corev1.LibraryEntry, coverKey string) *corev1.LibraryEntry {
	t.Helper()
	for _, e := range entries {
		if _, ok := e.GetCoverage()[coverKey]; ok {
			return e
		}
	}
	t.Fatalf("no entry carries coverage %q (%d entries)", coverKey, len(entries))
	return nil
}

// TestDerivedLibraryIsCachedUntilInvalidated pins the two-cache contract
// (PLAN.md §5.1): the derived library is served from cache even when the
// upstream catalogues move underneath it, and the availability events the
// synchronizer emits are what bring it back in step.
func TestDerivedLibraryIsCachedUntilInvalidated(t *testing.T) {
	ctx := context.Background()
	stk := newStack(t)
	stk.provA.items = []*slotsv1.CatalogueItem{
		withDirectors(movie("a1", "Up", 2009), "Pete Docter"),
	}

	stk.syncAll(t)
	first, err := stk.lib.Library(ctx, "alice")
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	find(t, first, "slot-a:a1")

	// A second account appears upstream and syncs. Its arrival event is
	// captured by the sink but not yet forwarded to the library service.
	stk.provB.items = []*slotsv1.CatalogueItem{
		movie("b9", "Coco", 2017),
	}
	stk.syncAll(t)

	// The derived library is cached: it does not see the new account's item
	// until something invalidates it.
	cached, err := stk.lib.Library(ctx, "alice")
	if err != nil {
		t.Fatalf("cached derive: %v", err)
	}
	find(t, cached, "slot-a:a1")
	for _, e := range cached {
		if _, ok := e.GetCoverage()["slot-b:b9"]; ok {
			t.Fatal("derived library moved without an invalidation event")
		}
	}

	// Forwarding the captured events — main.go's router forwards every
	// availability envelope to the service — invalidates alice's cache, and
	// the next derivation reflects the new catalogue.
	forwarded := false
	for _, env := range stk.sink.envs {
		stk.lib.Publish(env)
		forwarded = true
	}
	if !forwarded {
		t.Fatal("synchronizer emitted no availability events to forward")
	}
	fresh, err := stk.lib.Library(ctx, "alice")
	if err != nil {
		t.Fatalf("post-invalidation derive: %v", err)
	}
	find(t, fresh, "slot-a:a1")
	find(t, fresh, "slot-b:b9")
}

// TestRecycledNativeIDKeepsEntriesApartAndReportsConflict runs the T14
// scenario through the seam: a native id whose upstream meaning changes
// without corroboration must not drag the old entry's identity onto the new
// item. The old entry survives untouched, a fresh entry is created, and an
// owner-visible merge-conflict event reports the divergence.
func TestRecycledNativeIDKeepsEntriesApartAndReportsConflict(t *testing.T) {
	ctx := context.Background()
	stk := newStack(t)
	stk.provA.items = []*slotsv1.CatalogueItem{
		movie("r1", "The Matrix", 1999, imdb("tt0133093")),
	}
	stk.syncAll(t)

	original, okOrig, err := stk.reg.Lookup(ctx, "slot-a", "r1")
	if err != nil || !okOrig {
		t.Fatalf("initial lookup: %v ok=%v", err, okOrig)
	}
	canonOriginal, okCanon, err := stk.reg.Canonical(ctx, original)
	if err != nil || !okCanon || canonOriginal.Title != "The Matrix" {
		t.Fatalf("initial canonical: %+v ok=%v err=%v", canonOriginal, okCanon, err)
	}

	// The provider reuses r1 for an unrelated film: no shared signal, a
	// different external ID, a different title and year.
	stk.provA.items = []*slotsv1.CatalogueItem{
		movie("r1", "Battlefield Earth", 2000, imdb("tt0185183")),
	}
	stk.syncAll(t)

	recycled, okRec, err := stk.reg.Lookup(ctx, "slot-a", "r1")
	if err != nil || !okRec {
		t.Fatalf("lookup after recycle: %v ok=%v", err, okRec)
	}
	if recycled == original {
		t.Fatal("recycled mapping still points at the original entry")
	}
	canonOld, okOld, err := stk.reg.Canonical(ctx, original)
	if err != nil || !okOld || canonOld.Title != "The Matrix" {
		t.Fatalf("original entry damaged by recycle: %+v ok=%v err=%v", canonOld, okOld, err)
	}
	canonNew, okNew, err := stk.reg.Canonical(ctx, recycled)
	if err != nil || !okNew || canonNew.Title != "Battlefield Earth" {
		t.Fatalf("new entry: %+v ok=%v err=%v", canonNew, okNew, err)
	}
	if len(stk.resolve.events) == 0 {
		t.Fatal("recycled identity produced no merge-conflict event")
	}
	env := stk.resolve.events[len(stk.resolve.events)-1]
	if env.GetType() != corev1.EventType_EVENT_TYPE_MERGE_CONFLICT ||
		env.GetAudience() != corev1.EventAudience_EVENT_AUDIENCE_OWNER ||
		env.GetMergeConflict().GetProviderId() != "r1" ||
		env.GetMergeConflict().GetEntryId() != original {
		t.Fatalf("conflict envelope = %+v", env)
	}
}
