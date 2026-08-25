package enrichment

import (
	"context"
	"testing"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// --- fakes ---

func xid(ns, val string) *slotsv1.ExternalId {
	return &slotsv1.ExternalId{Namespace: ns, Value: val}
}

func md2(mutate func(*corev1.TitleMetadata)) *corev1.TitleMetadata {
	m := &corev1.TitleMetadata{}
	if mutate != nil {
		mutate(m)
	}
	return m
}

type fakeSource struct {
	ev EntryEvidence
	ok bool
}

func (f fakeSource) Evidence(context.Context, string) (EntryEvidence, bool, error) {
	return f.ev, f.ok, nil
}

type fakeStore struct {
	aliases map[string]string
	records map[string]*corev1.TitleMetadata
}

func newFakeStore() *fakeStore {
	return &fakeStore{aliases: map[string]string{}, records: map[string]*corev1.TitleMetadata{}}
}

func (f *fakeStore) Resolve(_ context.Context, id string) (string, bool, error) {
	ref, ok := f.aliases[id]
	return ref, ok, nil
}

func (f *fakeStore) GetRecord(_ context.Context, ref string) (*corev1.TitleMetadata, bool, error) {
	m, ok := f.records[ref]
	return m, ok, nil
}

func (f *fakeStore) PutRecord(_ context.Context, ref string, m *corev1.TitleMetadata) error {
	f.records[ref] = m
	return nil
}

func (f *fakeStore) LinkAlias(_ context.Context, alias, ref string) error {
	f.aliases[alias] = ref
	return nil
}

type fakeCatalogue struct {
	lookup   func(*slotsv1.LookupTitleRequest) (*slotsv1.LookupTitleResponse, error)
	metadata func(*slotsv1.GetMetadataRequest) (*slotsv1.GetMetadataResponse, error)
	lookups  int
}

func (f *fakeCatalogue) LookupTitle(_ context.Context, r *slotsv1.LookupTitleRequest) (*slotsv1.LookupTitleResponse, error) {
	f.lookups++
	return f.lookup(r)
}

func (f *fakeCatalogue) GetMetadata(_ context.Context, r *slotsv1.GetMetadataRequest) (*slotsv1.GetMetadataResponse, error) {
	return f.metadata(r)
}

// --- tests ---

var testNow = time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)

func entryEvidenceWith(ids ...*slotsv1.ExternalId) EntryEvidence {
	return EntryEvidence{
		Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE,
		Metadata: &corev1.TitleMetadata{
			Title:     "The Thing",
			Year:      1982,
			Directors: []string{"John Carpenter"},
		},
		ExternalIDs: ids,
	}
}

func thingRecord() *corev1.TitleMetadata {
	return &corev1.TitleMetadata{
		Title:     "The Thing",
		Year:      1982,
		Rating:    8.2,
		Directors: []string{"John Carpenter"}, // corroborates the entry evidence
	}
}

func TestCacheHitShortCircuitsLookup(t *testing.T) {
	st := newFakeStore()
	st.aliases["imdb:tt0084787"] = "tmdb:11576"
	st.records["tmdb:11576"] = thingRecord()

	cat := &fakeCatalogue{}
	e := NewEngine(fakeSource{ev: entryEvidenceWith(xid("imdb", "tt0084787")), ok: true}, st,
		[]Catalogue{{Slot: "tmdb", Client: cat}}, nil)
	if err := e.Enrich(context.Background(), "le_1"); err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	if cat.lookups != 0 {
		t.Fatalf("cache hit still searched: %d lookups", cat.lookups)
	}
}

func TestLookupFallbackAdoptsAndStores(t *testing.T) {
	st := newFakeStore()
	cat := &fakeCatalogue{
		lookup: func(r *slotsv1.LookupTitleRequest) (*slotsv1.LookupTitleResponse, error) {
			return &slotsv1.LookupTitleResponse{Candidates: []*slotsv1.TitleCandidate{
				{Ref: "tmdb:111", Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE, Title: "The Thing", Year: 2011},
				{Ref: "tmdb:11576", Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE, Title: "The Thing", Year: 1982},
			}}, nil
		},
		metadata: func(r *slotsv1.GetMetadataRequest) (*slotsv1.GetMetadataResponse, error) {
			if r.GetRef() != "tmdb:11576" {
				t.Fatalf("wrong ref fetched: %q", r.GetRef())
			}
			md := thingRecord()
			md.Source = nil // engine stamps provenance via Merge
			return &slotsv1.GetMetadataResponse{
				Metadata: md,
				ExternalIds: []*slotsv1.ExternalId{
					xid("tmdb", "11576"),
					xid("imdb", "tt0084787"),
				},
			}, nil
		},
	}
	e := NewEngine(fakeSource{ev: entryEvidenceWith(), ok: true}, st,
		[]Catalogue{{Slot: "tmdb", Client: cat}}, nil)
	e.now = func() time.Time { return testNow }

	if err := e.Enrich(context.Background(), "le_1"); err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	rec := st.records["tmdb:11576"]
	if rec == nil || rec.GetRating() != 8.2 {
		t.Fatalf("record not stored: %+v", rec)
	}
	if got := rec.GetSource()["base.rating"].GetSlot(); got != "catalogue:tmdb" {
		t.Fatalf("provenance owner %q", got)
	}
	if st.aliases["imdb:tt0084787"] != "tmdb:11576" {
		t.Fatalf("alias not linked: %+v", st.aliases)
	}
}

func TestAbstainedVerdictWritesNothing(t *testing.T) {
	st := newFakeStore()
	cat := &fakeCatalogue{
		lookup: func(*slotsv1.LookupTitleRequest) (*slotsv1.LookupTitleResponse, error) {
			// Two indistinguishable candidates — a genuine tie.
			return &slotsv1.LookupTitleResponse{Candidates: []*slotsv1.TitleCandidate{
				{Ref: "tmdb:1", Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE, Title: "The Thing", Year: 1982},
				{Ref: "tmdb:2", Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE, Title: "The Thing", Year: 1982},
			}}, nil
		},
		metadata: func(*slotsv1.GetMetadataRequest) (*slotsv1.GetMetadataResponse, error) {
			// Full records still carry nothing that separates them.
			return &slotsv1.GetMetadataResponse{Metadata: md2(func(m *corev1.TitleMetadata) {
				m.Title = "The Thing"
				m.Year = 1982
			})}, nil
		},
	}
	e := NewEngine(fakeSource{ev: entryEvidenceWith(), ok: true}, st,
		[]Catalogue{{Slot: "tmdb", Client: cat}}, nil)
	if err := e.Enrich(context.Background(), "le_1"); err != nil {
		t.Fatalf("abstention must not be an error: %v", err)
	}
	if len(st.records) != 0 || len(st.aliases) != 0 {
		t.Fatalf("abstained run wrote state: %+v %+v", st.records, st.aliases)
	}
}

func TestSecondSlotServesWhenFirstFails(t *testing.T) {
	st := newFakeStore()
	broken := &fakeCatalogue{lookup: func(*slotsv1.LookupTitleRequest) (*slotsv1.LookupTitleResponse, error) {
		return nil, context.DeadlineExceeded
	}}
	good := &fakeCatalogue{
		lookup: func(*slotsv1.LookupTitleRequest) (*slotsv1.LookupTitleResponse, error) {
			return &slotsv1.LookupTitleResponse{Candidates: []*slotsv1.TitleCandidate{
				{Ref: "ref:42", Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE, Title: "The Thing", Year: 1982},
			}}, nil
		},
		metadata: func(*slotsv1.GetMetadataRequest) (*slotsv1.GetMetadataResponse, error) {
			return &slotsv1.GetMetadataResponse{Metadata: thingRecord()}, nil
		},
	}
	e := NewEngine(fakeSource{ev: entryEvidenceWith(), ok: true}, st,
		[]Catalogue{{Slot: "a", Client: broken}, {Slot: "b", Client: good}}, nil)
	e.now = func() time.Time { return testNow }
	if err := e.Enrich(context.Background(), "le_1"); err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	if st.records["ref:42"] == nil {
		t.Fatal("healthy slot's record missing")
	}
	if got := st.records["ref:42"].GetSource()["base.title"].GetSlot(); got != "catalogue:b" {
		t.Fatalf("provenance owner %q", got)
	}
}

func TestGetMetadataFailureIsReported(t *testing.T) {
	st := newFakeStore()
	cat := &fakeCatalogue{
		lookup: func(*slotsv1.LookupTitleRequest) (*slotsv1.LookupTitleResponse, error) {
			return &slotsv1.LookupTitleResponse{Candidates: []*slotsv1.TitleCandidate{
				{Ref: "tmdb:9", Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE, Title: "The Thing", Year: 1982},
			}}, nil
		},
		metadata: func(*slotsv1.GetMetadataRequest) (*slotsv1.GetMetadataResponse, error) {
			return nil, context.Canceled
		},
	}
	e := NewEngine(fakeSource{ev: entryEvidenceWith(), ok: true}, st,
		[]Catalogue{{Slot: "tmdb", Client: cat}}, nil)
	if err := e.Enrich(context.Background(), "le_1"); err == nil {
		t.Fatal("get-metadata failure swallowed")
	}
}

func TestMissingEntrySkipsQuietly(t *testing.T) {
	e := NewEngine(fakeSource{ok: false}, newFakeStore(), nil, nil)
	if err := e.Enrich(context.Background(), "le_gone"); err != nil {
		t.Fatalf("vanished entry errored: %v", err)
	}
}
