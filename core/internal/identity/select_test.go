package identity

import (
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

func xid(ns, val string) *slotsv1.ExternalId {
	return &slotsv1.ExternalId{Namespace: ns, Value: val}
}

func candItem(mutate func(*corev1.TitleMetadata), ids ...*slotsv1.ExternalId) Item {
	m := &corev1.TitleMetadata{Title: "The Thing", Year: 1982}
	if mutate != nil {
		mutate(m)
	}
	return Item{Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE, ExternalIDs: ids, Metadata: m}
}

func entryEvidence() Item {
	return Item{
		Kind:        slotsv1.ItemKind_ITEM_KIND_MOVIE,
		ExternalIDs: []*slotsv1.ExternalId{xid("jellyfin-native", "x")},
		Metadata: &corev1.TitleMetadata{
			Title:            "the thing", // normalization applies
			Year:             1982,
			Directors:        []string{"John Carpenter"},
			OriginalLanguage: "en",
		},
	}
}

func TestUniqueSurvivorIsAdopted(t *testing.T) {
	cands := []Item{
		candItem(func(m *corev1.TitleMetadata) { m.Year = 2011 }), // year gate rejects
		candItem(func(m *corev1.TitleMetadata) { m.Directors = []string{"John Carpenter"} },
			xid("tmdb", "11576")), // title+year+director
	}
	if picked, ok := Adopt(entryEvidence(), cands); !ok || picked != 1 {
		t.Fatalf("Adopt = (%d, %v), want (1, true)", picked, ok)
	}
}

func TestTieAfterFullScoringAbstains(t *testing.T) {
	// Both candidates share title, year and director — nothing separates
	// them, so the gate must abstain rather than guess.
	same := func(m *corev1.TitleMetadata) {}
	cands := []Item{
		candItem(same, xid("tmdb", "111")),
		candItem(same, xid("tmdb", "222")),
	}
	if picked, ok := Adopt(entryEvidence(), cands); ok {
		t.Fatalf("tie adopted candidate %d", picked)
	}
}

func TestCorroboratedCandidateWinsAmongLookalikes(t *testing.T) {
	entry := entryEvidence()
	entry.ExternalIDs = append(entry.ExternalIDs, xid("imdb", "tt0084787"))

	cands := []Item{
		candItem(nil, xid("tmdb", "111")),
		candItem(func(m *corev1.TitleMetadata) {}, xid("imdb", "tt0084787"), xid("tmdb", "11576")),
	}
	picked, ok := Adopt(entry, cands)
	if !ok || picked != 1 {
		t.Fatalf("Adopt = (%d, %v), want corroborated index 1", picked, ok)
	}
}

func TestTwoCandidatesAssertingSameIDAbstain(t *testing.T) {
	entry := entryEvidence()
	entry.ExternalIDs = append(entry.ExternalIDs, xid("imdb", "tt0084787"))
	cands := []Item{
		candItem(nil, xid("imdb", "tt0084787")),
		candItem(nil, xid("imdb", "tt0084787")),
	}
	if picked, ok := Adopt(entry, cands); ok {
		t.Fatalf("contradictory assertions adopted %d", picked)
	}
}

func TestContradictoryExternalIDRejectsCandidate(t *testing.T) {
	entry := entryEvidence()
	entry.ExternalIDs = append(entry.ExternalIDs, xid("imdb", "tt0084787"))

	cands := []Item{
		candItem(nil, xid("imdb", "tt9999999"), xid("tmdb", "11576")),
	}
	if picked, ok := Adopt(entry, cands); ok {
		t.Fatalf("contradicted ID still adopted %d", picked)
	}
}

func TestKindMismatchNeverAdopts(t *testing.T) {
	series := candItem(nil)
	series.Kind = slotsv1.ItemKind_ITEM_KIND_SERIES
	if _, ok := Adopt(entryEvidence(), []Item{series}); ok {
		t.Fatal("series candidate adopted for a movie")
	}
}

func TestYearlessMovieEntryStaysUnenriched(t *testing.T) {
	entry := entryEvidence()
	entry.Metadata.Year = 0 // provider did not supply a year
	if picked, ok := Adopt(entry, []Item{candItem(nil)}); ok {
		t.Fatalf("yearless movie adopted %d — unknown years must fail the gate", picked)
	}
}

func TestSelectVerdictsStayParallel(t *testing.T) {
	cands := []Item{
		candItem(func(m *corev1.TitleMetadata) { m.Title = "Something Else" }),
		candItem(func(m *corev1.TitleMetadata) { m.OriginalLanguage = "en" }), // signal via language
	}
	got := Select(entryEvidence(), cands)
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Merge || !got[1].Merge {
		t.Fatalf("verdicts not parallel to candidates: %+v", got)
	}
	if got[1].Signal != SignalOriginalLanguage {
		t.Fatalf("expected original-language signal, got %v", got[1].Signal)
	}
}

func TestNoCandidatesAbstains(t *testing.T) {
	if picked, ok := Adopt(entryEvidence(), nil); ok {
		t.Fatalf("empty candidate list adopted %d", picked)
	}
}
