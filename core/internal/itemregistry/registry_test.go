package itemregistry

import (
	"context"
	"path/filepath"
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/schema"
	"github.com/nem-git/abcmovies/core/internal/store"
)

func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	r, err := New(store.NewInMemory(), "owner-1")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return r
}

func movie(id, title string, year uint32, tweak func(*corev1.TitleMetadata), ids ...*slotsv1.ExternalId) *slotsv1.CatalogueItem {
	md := &corev1.TitleMetadata{Title: title, Year: year}
	if tweak != nil {
		tweak(md)
	}
	return &slotsv1.CatalogueItem{
		NativeId:    id,
		Kind:        slotsv1.ItemKind_ITEM_KIND_MOVIE,
		Metadata:    md,
		ExternalIds: ids,
	}
}

func dir(name string) func(*corev1.TitleMetadata) {
	return func(m *corev1.TitleMetadata) { m.Directors = []string{name} }
}

func TestFirstSeenCreatesEntryThenPureLookup(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	out, err := r.Resolve(ctx, "jellyfin", movie("m1", "The Matrix", 1999, nil, &slotsv1.ExternalId{Namespace: "imdb", Value: "tt0133093"}))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if out.Status != StatusCreated || out.EntryID == "" || out.Recycled || len(out.Events) != 0 {
		t.Fatalf("first-seen outcome = %+v", out)
	}

	// Identical item again: a pure lookup with no identity work.
	again, err := r.Resolve(ctx, "jellyfin", movie("m1", "The Matrix", 1999, nil, &slotsv1.ExternalId{Namespace: "imdb", Value: "tt0133093"}))
	if err != nil {
		t.Fatalf("Resolve again: %v", err)
	}
	if again.Status != StatusUnchanged || again.EntryID != out.EntryID {
		t.Fatalf("refresh outcome = %+v, want unchanged on %s", again, out.EntryID)
	}
	if _, ok, err := r.Lookup(ctx, "jellyfin", "m1"); err != nil || !ok {
		t.Fatalf("Lookup: %v (ok=%v)", err, ok)
	}

	// Inert metadata changes never re-open identity (refresh is a pure
	// lookup against the proof).
	repainted := movie("m1", "The Matrix", 1999,
		func(m *corev1.TitleMetadata) { m.PosterUrl = "https://x/new.jpg"; m.Rating = 9.1 },
		&slotsv1.ExternalId{Namespace: "imdb", Value: "tt0133093"})
	same, err := r.Resolve(ctx, "jellyfin", repainted)
	if err != nil {
		t.Fatalf("Resolve repainted: %v", err)
	}
	if same.Status != StatusUnchanged || same.EntryID != out.EntryID {
		t.Fatalf("inert change reopened identity: %+v", same)
	}
}

func TestCorroborationAttachesAcrossDifferentTitles(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()
	imdb := &slotsv1.ExternalId{Namespace: "imdb", Value: "tt0113243"}

	first, err := r.Resolve(ctx, "jellyfin", movie("j1", "Heat", 1995, nil, imdb))
	if err != nil || first.Status != StatusCreated {
		t.Fatalf("first: %+v err=%v", first, err)
	}
	// Another provider item asserts the same ID under different words.
	second, err := r.Resolve(ctx, "tmdb", movie("t9", "Heat (1995)", 1995, nil, imdb))
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.Status != StatusAttached || second.EntryID != first.EntryID {
		t.Fatalf("matching provider-supplied IDs must attach: %+v vs %s", second, first.EntryID)
	}
}

func TestHeuristicAttachAccumulatesSignalsAndIDs(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	a, err := r.Resolve(ctx, "p", movie("a1", "The Matrix", 1999, dir("Wachowskis"), &slotsv1.ExternalId{Namespace: "imdb", Value: "tt0133093"}))
	if err != nil || a.Status != StatusCreated {
		t.Fatalf("a: %+v err=%v", a, err)
	}
	b, err := r.Resolve(ctx, "p", movie("b2", "matrix", 1999, dir("Wachowskis"), &slotsv1.ExternalId{Namespace: "tmdb", Value: "603"}))
	if err != nil {
		t.Fatalf("b: %v", err)
	}
	if b.Status != StatusAttached || b.EntryID != a.EntryID {
		t.Fatalf("heuristic merge expected: %+v want entry %s", b, a.EntryID)
	}

	// The third sibling corroborates via the ID the second one contributed.
	c, err := r.Resolve(ctx, "q", movie("c3", "MATRIX", 1999, nil, &slotsv1.ExternalId{Namespace: "tmdb", Value: "603"}))
	if err != nil {
		t.Fatalf("c: %v", err)
	}
	if c.Status != StatusAttached || c.EntryID != a.EntryID {
		t.Fatalf("absorbed assertion must corroborate later items: %+v", c)
	}
}

func TestNoSignalStaysSeparate(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()
	a, err := r.Resolve(ctx, "p", movie("a1", "Heat", 1995, nil))
	if err != nil {
		t.Fatalf("a: %v", err)
	}
	b, err := r.Resolve(ctx, "p", movie("b1", "heat", 1995, nil))
	if err != nil {
		t.Fatalf("b: %v", err)
	}
	if b.Status != StatusCreated || b.EntryID == a.EntryID {
		t.Fatalf("same title+year without signal must stay separate: %+v vs %s", b, a.EntryID)
	}
}

func TestKindMismatchNeverMergesEvenWithSharedID(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()
	imdb := &slotsv1.ExternalId{Namespace: "imdb", Value: "tt4906026"}
	a, err := r.Resolve(ctx, "p", movie("mv", "It", 2017, nil, imdb))
	if err != nil {
		t.Fatalf("a: %v", err)
	}
	series := movie("sr", "It", 2017, nil, imdb)
	series.Kind = slotsv1.ItemKind_ITEM_KIND_SERIES
	b, err := r.Resolve(ctx, "p", series)
	if err != nil {
		t.Fatalf("b: %v", err)
	}
	if b.Status != StatusCreated || b.EntryID == a.EntryID {
		t.Fatalf("contradictory kinds must not merge: %+v", b)
	}
}

func TestProofEvolutionOnSameEntryIsUpdated(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()
	doct := dir("Peter Docter")
	a, err := r.Resolve(ctx, "p", movie("m1", "Up", 2009, doct))
	if err != nil || a.Status != StatusCreated {
		t.Fatalf("a: %+v err=%v", a, err)
	}
	// The provider begins asserting an external ID for the same item; the
	// shared director re-anchors it onto the same entry.
	grown, err := r.Resolve(ctx, "p", movie("m1", "Up", 2009, doct, &slotsv1.ExternalId{Namespace: "imdb", Value: "tt0435761"}))
	if err != nil {
		t.Fatalf("grown: %v", err)
	}
	if grown.Status != StatusUpdated || grown.EntryID != a.EntryID || grown.Recycled || len(grown.Events) != 0 {
		t.Fatalf("proof evolution on the same entry = %+v", grown)
	}
}

func TestProofChangeWithoutSignalBecomesNewInstance(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()
	a, err := r.Resolve(ctx, "p", movie("m1", "Up", 2009, nil))
	if err != nil || a.Status != StatusCreated {
		t.Fatalf("a: %+v err=%v", a, err)
	}
	// The proof changed (an assertion appeared) but nothing corroborates the
	// live item against the old entry — no shared signal. The conservative
	// rule wins: new instance, conflict surfaced, old mapping aliased
	// (PLAN.md §5.3).
	grown, err := r.Resolve(ctx, "p", movie("m1", "Up", 2009, nil, &slotsv1.ExternalId{Namespace: "imdb", Value: "tt0435761"}))
	if err != nil {
		t.Fatalf("grown: %v", err)
	}
	if !grown.Recycled || grown.Status != StatusCreated || grown.EntryID == a.EntryID || len(grown.Events) != 1 {
		t.Fatalf("uncorroborated proof change = %+v", grown)
	}
}

func TestRecycledIDEmitsConflictKeepsAliasAndOldEntry(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	first, err := r.Resolve(ctx, "streamco", movie("id7", "Old Feature", 1984, dir("Jane Director"), &slotsv1.ExternalId{Namespace: "imdb", Value: "tt0087000"}))
	if err != nil || first.Status != StatusCreated {
		t.Fatalf("first: %+v err=%v", first, err)
	}

	recycled := movie("id7", "Totally Different Show", 2021, nil, &slotsv1.ExternalId{Namespace: "imdb", Value: "tt13999999"})
	out, err := r.Resolve(ctx, "streamco", recycled)
	if err != nil {
		t.Fatalf("recycled: %v", err)
	}
	if !out.Recycled || out.SupersededEntryID != first.EntryID || out.Status != StatusCreated {
		t.Fatalf("recycled outcome = %+v", out)
	}
	if out.EntryID == first.EntryID {
		t.Fatal("recycled id must become a new instance")
	}
	if len(out.Events) != 1 {
		t.Fatalf("expected exactly one conflict event, got %d", len(out.Events))
	}
	env := out.Events[0]
	if env.GetType() != corev1.EventType_EVENT_TYPE_MERGE_CONFLICT ||
		env.GetAudience() != corev1.EventAudience_EVENT_AUDIENCE_OWNER ||
		env.GetMergeConflict().GetEntryId() != first.EntryID ||
		env.GetMergeConflict().GetProvider() != "streamco" ||
		env.GetMergeConflict().GetProviderId() != "id7" {
		t.Fatalf("conflict envelope wrong: %+v", env)
	}
	if err := schema.ValidateEventEnvelope(env); err != nil {
		t.Fatalf("event fails schema validation: %v", err)
	}

	// The superseded generation still resolves to the old entry.
	old, ok, err := r.Alias(ctx, "streamco", "id7", 1)
	if err != nil || !ok || old != first.EntryID {
		t.Fatalf("alias gen1 = %q ok=%v err=%v, want old entry", old, ok, err)
	}
	// And a further refresh of the recycled item is a pure lookup.
	again, err := r.Resolve(ctx, "streamco", recycled)
	if err != nil || again.Status != StatusUnchanged || again.EntryID != out.EntryID {
		t.Fatalf("post-recycle refresh = %+v err=%v", again, err)
	}
}

func TestInvalidItemsRejected(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()
	cases := []struct {
		name string
		item *slotsv1.CatalogueItem
	}{
		{"nil item", nil},
		{"missing native id", &slotsv1.CatalogueItem{Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE, Metadata: &corev1.TitleMetadata{Title: "X"}}},
		{"unspecified kind", &slotsv1.CatalogueItem{NativeId: "n1", Metadata: &corev1.TitleMetadata{Title: "X"}}},
		{"missing metadata", &slotsv1.CatalogueItem{NativeId: "n1", Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE}},
		{"missing title", &slotsv1.CatalogueItem{NativeId: "n1", Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE, Metadata: &corev1.TitleMetadata{Year: 2001}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := r.Resolve(ctx, "p", tc.item); err == nil {
				t.Fatal("invalid item accepted")
			}
		})
	}
}

func TestSQLiteBackendDurability(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "identity.db")
	ctx := context.Background()

	open := func() *Registry {
		t.Helper()
		st, err := store.NewSQLite(ctx, path)
		if err != nil {
			t.Fatalf("NewSQLite: %v", err)
		}
		t.Cleanup(func() { _ = st.Close() })
		r, err := New(st, "owner-1")
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		return r
	}

	r := open()
	first, err := r.Resolve(ctx, "p", movie("m1", "The Matrix", 1999, nil, &slotsv1.ExternalId{Namespace: "imdb", Value: "tt0133093"}))
	if err != nil || first.Status != StatusCreated {
		t.Fatalf("first: %+v err=%v", first, err)
	}

	// Reopen: durable class semantics — mappings and aliases survive.
	r = open()
	entryID, ok, err := r.Lookup(ctx, "p", "m1")
	if err != nil || !ok || entryID != first.EntryID {
		t.Fatalf("after reopen: %q ok=%v err=%v", entryID, ok, err)
	}
	recycled := movie("m1", "Brand New Title Entirely", 2077, nil)
	out, err := r.Resolve(ctx, "p", recycled)
	if err != nil {
		t.Fatalf("recycle after reopen: %v", err)
	}
	if !out.Recycled || len(out.Events) != 1 {
		t.Fatalf("recycle after reopen = %+v", out)
	}
}

func TestCanonicalExposesClaimsWithProvenance(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()
	a, err := r.Resolve(ctx, "jellyfin", movie("a1", "The Matrix", 1999, nil, &slotsv1.ExternalId{Namespace: "imdb", Value: "tt0133093"}))
	if err != nil || a.Status != StatusCreated {
		t.Fatalf("a: %+v err=%v", a, err)
	}
	// A second provider asserting the same claim corroborates it; a new claim
	// arrives single-supplier.
	if _, err := r.Resolve(ctx, "tmdb", movie("b1", "The Matrix", 1999, nil, &slotsv1.ExternalId{Namespace: "imdb", Value: "tt0133093"})); err != nil {
		t.Fatalf("b: %v", err)
	}

	canon, ok, err := r.Canonical(ctx, a.EntryID)
	if err != nil || !ok {
		t.Fatalf("Canonical: %v ok=%v", err, ok)
	}
	if canon.Title != "The Matrix" || canon.Year != 1999 || canon.Kind != slotsv1.ItemKind_ITEM_KIND_MOVIE {
		t.Fatalf("canonical = %+v", canon)
	}
	if len(canon.Claims) != 1 {
		t.Fatalf("claims = %+v, want exactly the imdb assertion", canon.Claims)
	}
	claim := canon.Claims[0]
	if claim.Namespace != "imdb" || claim.Value != "tt0133093" {
		t.Fatalf("claim = %+v", claim)
	}
	if len(claim.Suppliers) != 2 || claim.Suppliers[0] != "jellyfin" || claim.Suppliers[1] != "tmdb" {
		t.Fatalf("suppliers = %v, want both providers recorded", claim.Suppliers)
	}
}
