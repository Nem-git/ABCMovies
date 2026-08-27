// Package m3 holds the milestone acceptance tests for M3 (enrichment):
// metadata cache, merge engine, corroboration gate, and the full catalogue
// enrichment pipeline (lookup → screen → detail-fetch → adopt → store).
package m3_test

import (
	"context"
	"log/slog"
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/enrichment"
	"github.com/nem-git/abcmovies/core/internal/metadatacache"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// entryStore is a trivial in-memory entry source for tests.
type entryStore struct {
	entries map[string]enrichment.EntryEvidence
}

func (s *entryStore) Evidence(_ context.Context, id string) (enrichment.EntryEvidence, bool, error) {
	ev, ok := s.entries[id]
	return ev, ok, nil
}

// fakeCatalogue is a canned CatalogueClient: every LookupTitle and
// GetMetadata call is recorded and the canned responses returned.
type fakeCatalogue struct {
	lookupResponse *slotsv1.LookupTitleResponse
	lookupErr      error
	getResponse    *slotsv1.GetMetadataResponse
	getErr         error

	lookupRequests []*slotsv1.LookupTitleRequest
	getRequests    []*slotsv1.GetMetadataRequest
}

func (f *fakeCatalogue) LookupTitle(_ context.Context, req *slotsv1.LookupTitleRequest) (*slotsv1.LookupTitleResponse, error) {
	f.lookupRequests = append(f.lookupRequests, req)
	return f.lookupResponse, f.lookupErr
}

func (f *fakeCatalogue) GetMetadata(_ context.Context, req *slotsv1.GetMetadataRequest) (*slotsv1.GetMetadataResponse, error) {
	f.getRequests = append(f.getRequests, req)
	return f.getResponse, f.getErr
}

// TestEnrichResolvesByExternalIDCacheShortcircuit pins the fast path:
// when an asserted external ID already resolves in the metadata cache,
// Enrich returns without hitting the catalogue (engine.go line 97-98).
func TestEnrichResolvesByExternalIDCacheShortcircuit(t *testing.T) {
	st := store.NewInMemory()
	cache, err := metadatacache.New(st, slog.Default())
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	// Seed the cache with a known record and alias.
	ref := "tmdb:27205"
	want := &corev1.TitleMetadata{Title: "Inception", Year: 2010}
	if err := cache.PutRecord(context.Background(), ref, want); err != nil {
		t.Fatalf("seed record: %v", err)
	}
	if err := cache.LinkAlias(context.Background(), "imdb:tt1375666", ref); err != nil {
		t.Fatalf("seed alias: %v", err)
	}

	src := &entryStore{entries: map[string]enrichment.EntryEvidence{
		"e1": {
			Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE,
			Metadata: &corev1.TitleMetadata{
				Title: "Inception",
				Year:  2010,
			},
			ExternalIDs: []*slotsv1.ExternalId{
				{Namespace: "imdb", Value: "tt1375666"},
			},
		},
	}}

	cat := &fakeCatalogue{}
	eng := enrichment.NewEngine(src, cache, []enrichment.Catalogue{
		{Slot: "tmdb", Client: cat},
	}, slog.Default())

	if err := eng.Enrich(context.Background(), "e1"); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if len(cat.lookupRequests) != 0 {
		t.Fatalf("catalogue was called despite cache hit: %d lookups", len(cat.lookupRequests))
	}
}

// TestEnrichCachesNewRecordAfterAdopt pins the full lookup → screen →
// detail-fetch → adopt → merge/store path. The catalogue returns a single
// candidate that matches on title and year; after Enrich the record and
// aliases are persisted.
func TestEnrichCachesNewRecordAfterAdopt(t *testing.T) {
	st := store.NewInMemory()
	cache, err := metadatacache.New(st, slog.Default())
	if err != nil {
		t.Fatalf("cache: %v", err)
	}

	src := &entryStore{entries: map[string]enrichment.EntryEvidence{
		"e2": {
			Kind:        slotsv1.ItemKind_ITEM_KIND_MOVIE,
			Metadata:    &corev1.TitleMetadata{Title: "Coco", Year: 2017},
			ExternalIDs: []*slotsv1.ExternalId{{Namespace: "imdb", Value: "tt2380307"}},
		},
	}}

	cat := &fakeCatalogue{
		lookupResponse: &slotsv1.LookupTitleResponse{
			Candidates: []*slotsv1.TitleCandidate{
				{
					Ref:           "tmdb:354912",
					Kind:          slotsv1.ItemKind_ITEM_KIND_MOVIE,
					Title:         "Coco",
					OriginalTitle: "Coco",
					Year:          2017,
					ExternalIds:   []*slotsv1.ExternalId{{Namespace: "imdb", Value: "tt2380307"}},
				},
			},
		},
		getResponse: &slotsv1.GetMetadataResponse{
			Metadata: &corev1.TitleMetadata{
				Title:            "Coco",
				Year:             2017,
				Description:      "A aspiring musician enters the Land of the Dead.",
				PosterUrl:        "https://image.tmdb.org/t/p/w500/example.jpg",
				Rating:           8.2,
				Genres:           []string{"Animation", "Family"},
				OriginalLanguage: "en",
			},
			ExternalIds: []*slotsv1.ExternalId{
				{Namespace: "tmdb", Value: "354912"},
				{Namespace: "imdb", Value: "tt2380307"},
			},
		},
	}

	eng := enrichment.NewEngine(src, cache, []enrichment.Catalogue{
		{Slot: "tmdb", Client: cat},
	}, slog.Default())

	if err := eng.Enrich(context.Background(), "e2"); err != nil {
		t.Fatalf("enrich: %v", err)
	}

	// Verify the record was cached under the canonical ref.
	got, ok, err := cache.GetRecord(context.Background(), "tmdb:354912")
	if err != nil || !ok {
		t.Fatalf("record not cached: ok=%v err=%v", ok, err)
	}
	if got.GetTitle() != "Coco" || got.GetYear() != 2017 {
		t.Fatalf("cached record = %+v", got)
	}

	// Verify aliases were linked.
	imdbRef, ok, err := cache.Resolve(context.Background(), "imdb:tt2380307")
	if err != nil || !ok || imdbRef != "tmdb:354912" {
		t.Fatalf("imdb alias not linked: ref=%q ok=%v err=%v", imdbRef, ok, err)
	}

	// Verify the correct request was sent.
	if len(cat.lookupRequests) != 1 {
		t.Fatalf("expected 1 lookup, got %d", len(cat.lookupRequests))
	}
	if cat.lookupRequests[0].GetQuery() != "Coco" {
		t.Fatalf("lookup query = %q", cat.lookupRequests[0].GetQuery())
	}
	if len(cat.getRequests) != 1 || cat.getRequests[0].GetRef() != "tmdb:354912" {
		t.Fatalf("get metadata ref = %+v", cat.getRequests)
	}
}

// TestEnrichAbstainsWhenNoCandidatesFound pins the "nothing to do" path:
// an empty catalogue response means no adoption, no cache write, no error.
func TestEnrichAbstainsWhenNoCandidatesFound(t *testing.T) {
	st := store.NewInMemory()
	cache, err := metadatacache.New(st, slog.Default())
	if err != nil {
		t.Fatalf("cache: %v", err)
	}

	src := &entryStore{entries: map[string]enrichment.EntryEvidence{
		"e3": {
			Kind:     slotsv1.ItemKind_ITEM_KIND_MOVIE,
			Metadata: &corev1.TitleMetadata{Title: "Obscure Film", Year: 2024},
		},
	}}

	cat := &fakeCatalogue{
		lookupResponse: &slotsv1.LookupTitleResponse{Candidates: nil},
	}

	eng := enrichment.NewEngine(src, cache, []enrichment.Catalogue{
		{Slot: "tmdb", Client: cat},
	}, slog.Default())

	if err := eng.Enrich(context.Background(), "e3"); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if len(cat.getRequests) != 0 {
		t.Fatalf("GetMetadata called despite empty lookup: %d calls", len(cat.getRequests))
	}
}

// TestEnrichReturnsErrorWhenAllDetailFetchesFail pins the error path:
// screening finds candidates but every detail fetch fails — the engine
// returns a concrete error rather than silently abstaining (engine.go:154).
func TestEnrichReturnsErrorWhenAllDetailFetchesFail(t *testing.T) {
	st := store.NewInMemory()
	cache, err := metadatacache.New(st, slog.Default())
	if err != nil {
		t.Fatalf("cache: %v", err)
	}

	src := &entryStore{entries: map[string]enrichment.EntryEvidence{
		"e4": {
			Kind:     slotsv1.ItemKind_ITEM_KIND_MOVIE,
			Metadata: &corev1.TitleMetadata{Title: "Doomed", Year: 2023},
		},
	}}

	cat := &fakeCatalogue{
		lookupResponse: &slotsv1.LookupTitleResponse{
			Candidates: []*slotsv1.TitleCandidate{
				{
					Ref:   "tmdb:99999",
					Kind:  slotsv1.ItemKind_ITEM_KIND_MOVIE,
					Title: "Doomed",
					Year:  2023,
				},
			},
		},
		getErr: context.DeadlineExceeded,
	}

	eng := enrichment.NewEngine(src, cache, []enrichment.Catalogue{
		{Slot: "tmdb", Client: cat},
	}, slog.Default())

	if err := eng.Enrich(context.Background(), "e4"); err == nil {
		t.Fatal("expected error when all detail fetches fail, got nil")
	}
}
