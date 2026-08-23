package identity

import (
	"fmt"
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

func extID(ns, value string) *slotsv1.ExternalId {
	return &slotsv1.ExternalId{Namespace: ns, Value: value}
}

// meta builds TitleMetadata with only the given corroborating fields set;
// inert fields are exercised separately.
func meta(title string, year uint32) *corev1.TitleMetadata {
	return &corev1.TitleMetadata{Title: title, Year: year}
}

func movieItem(title string, year uint32, m func(*corev1.TitleMetadata), ids ...*slotsv1.ExternalId) Item {
	md := meta(title, year)
	if m != nil {
		m(md)
	}
	return Item{
		Kind:        slotsv1.ItemKind_ITEM_KIND_MOVIE,
		ExternalIDs: ids,
		Metadata:    md,
	}
}

func TestDecideMergesOnMatchingProviderExternalID(t *testing.T) {
	t.Parallel()
	a := movieItem("Shawshank", 1994, nil, extID("imdb", "tt0111161"))
	b := movieItem("The Shawshank Redemption (completely different words)", 1999, nil, extID("imdb", "tt0111161"), extID("tmdb", "278"))
	got := Decide(a, b)
	if !got.Merge || !got.Corroborated || got.Signal != SignalNone {
		t.Fatalf("provider ID assertion must merge outright, got %+v", got)
	}
}

func TestDecideCorroborationRequiresNamespaceAndValueToMatch(t *testing.T) {
	t.Parallel()
	a := movieItem("Up", 2009, nil, extID("imdb", "tt0435761"))
	b := movieItem("Up", 2009, nil, extID("tmdb", "tt0435761")) // same value, other namespace
	if got := Decide(a, b); got.Merge {
		t.Fatalf("cross-namespace IDs are not assertions: %+v", got)
	}
	c := movieItem("Up", 2009, nil, extID("", "x"))
	d := movieItem("Up", 2009, nil, extID("", "x"))
	if got := Decide(c, d); got.Corroborated {
		t.Fatalf("empty namespaces must never corroborate: %+v", got)
	}
}

func TestDecideHeuristicMergesPerSignal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		tweakA func(*corev1.TitleMetadata)
		tweakB func(*corev1.TitleMetadata)
		want   Signal
	}{
		{"director", func(m *corev1.TitleMetadata) { m.Directors = []string{"Frank Darabont"} }, func(m *corev1.TitleMetadata) { m.Directors = []string{"Roger Avary", "Frank Darabont"} }, SignalDirector},
		{"cast member", func(m *corev1.TitleMetadata) { m.Cast = []string{"Tim Robbins"} }, func(m *corev1.TitleMetadata) { m.Cast = []string{"Morgan Freeman", "Tim Robbins"} }, SignalCastMember},
		{"original language", func(m *corev1.TitleMetadata) { m.OriginalLanguage = "en" }, func(m *corev1.TitleMetadata) { m.OriginalLanguage = "en" }, SignalOriginalLanguage},
		{"duration", func(m *corev1.TitleMetadata) {
			m.KindSpecific = &corev1.TitleMetadata_Movie{Movie: &corev1.MovieSpecific{RuntimeMinutes: 142}}
		}, func(m *corev1.TitleMetadata) {
			m.KindSpecific = &corev1.TitleMetadata_Movie{Movie: &corev1.MovieSpecific{RuntimeMinutes: 142}}
		}, SignalDuration},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := movieItem("The Shawshank Redemption", 1994, tc.tweakA)
			b := movieItem("the   shawshank redemption", 1994, tc.tweakB)
			got := Decide(a, b)
			if !got.Merge || got.Corroborated || got.Signal != tc.want {
				t.Fatalf("got %+v, want heuristic merge via %v", got, tc.want)
			}
		})
	}
}

func TestDecideReportsHighestRankedSignal(t *testing.T) {
	t.Parallel()
	a := movieItem("Heat", 1995, func(m *corev1.TitleMetadata) {
		m.Directors = []string{"Michael Mann"}
		m.Cast = []string{"Al Pacino"}
	})
	b := movieItem("Heat", 1995, func(m *corev1.TitleMetadata) {
		m.Directors = []string{"Michael Mann"}
		m.Cast = []string{"Robert De Niro"}
	})
	if got := Decide(a, b); got.Signal != SignalDirector {
		t.Fatalf("director outranks cast member, got %v (%+v)", got.Signal, got)
	}
}

func TestDecideSeparatesWithoutSignal(t *testing.T) {
	t.Parallel()
	a := movieItem("Heat", 1995, func(m *corev1.TitleMetadata) { m.OriginalLanguage = "en" })
	b := movieItem("Heat", 1995, func(m *corev1.TitleMetadata) { m.OriginalLanguage = "de" })
	if got := Decide(a, b); got.Merge {
		t.Fatalf("same title+year without a signal stays separate: %+v", got)
	}
}

func TestDecideInertFieldsNeverDriveOrBlockMerges(t *testing.T) {
	t.Parallel()
	rich := func(m *corev1.TitleMetadata) {
		m.Description = "A tale of two robbers."
		m.PosterUrl = "https://example.com/poster.jpg"
		m.Rating = 8.3
		m.ContentRating = "R"
		m.Genres = []string{"Crime", "Drama"}
	}
	a := movieItem("Heat", 1995, rich)
	// Same title and year, no corroborating signal, wildly different inert
	// fields: must stay separate — inert fields cannot drive a merge.
	b := movieItem("Heat", 1995, func(m *corev1.TitleMetadata) {})
	if got := Decide(a, b); got.Merge {
		t.Fatalf("inert fields drove a merge: %+v", got)
	}
	// And the reverse shape: identical inert fields still need a real signal.
	if got := Decide(movieItem("Heat", 1995, rich), movieItem("Heat", 1995, rich)); got.Merge {
		t.Fatalf("identical inert fields acted as a signal: %+v", got)
	}
	// But they do not block one either.
	if got := Decide(
		movieItem("Heat", 1995, func(m *corev1.TitleMetadata) { rich(m); m.Directors = []string{"Michael Mann"} }),
		movieItem("Heat", 1995, func(m *corev1.TitleMetadata) { m.PosterUrl = "other"; m.Directors = []string{"Michael Mann"} }),
	); !got.Merge {
		t.Fatalf("inert fields blocked a signal-backed merge: %+v", got)
	}
}

func TestDecideMovieYearGate(t *testing.T) {
	t.Parallel()
	dir := func(m *corev1.TitleMetadata) { m.Directors = []string{"Michael Mann"} }
	if got := Decide(movieItem("Heat", 1995, dir), movieItem("Heat", 2019, dir)); got.Merge {
		t.Fatalf("differing known years separate movies: %+v", got)
	}
	if got := Decide(movieItem("Heat", 1995, dir), movieItem("Heat", 0, dir)); got.Merge {
		t.Fatalf("unknown year fails the gate: %+v", got)
	}
	if got := Decide(movieItem("Heat", 0, dir), movieItem("Heat", 0, dir)); got.Merge {
		t.Fatalf("two unknown years are not a match: %+v", got)
	}
}

func TestDecideSeriesIgnoreYears(t *testing.T) {
	t.Parallel()
	series := func(title string, year uint32, tweak func(*corev1.TitleMetadata)) Item {
		it := movieItem(title, year, tweak)
		it.Kind = slotsv1.ItemKind_ITEM_KIND_SERIES
		return it
	}
	lang := func(m *corev1.TitleMetadata) {
		m.OriginalLanguage = "en"
		m.KindSpecific = &corev1.TitleMetadata_Series{Series: &corev1.SeriesSpecific{TotalSeasons: 5}}
	}
	got := Decide(series("Breaking Bad", 2008, lang), series("breaking bad", 2099, lang))
	if !got.Merge || got.Signal != SignalOriginalLanguage {
		t.Fatalf("series must ignore years: %+v", got)
	}
}

func TestDecideKindMismatchStaysSeparate(t *testing.T) {
	t.Parallel()
	dir := func(m *corev1.TitleMetadata) { m.Directors = []string{"Frank Darabont"} }
	a := movieItem("It", 2017, dir)
	b := movieItem("It", 2017, dir)
	b.Kind = slotsv1.ItemKind_ITEM_KIND_SERIES
	if got := Decide(a, b); got.Merge {
		t.Fatalf("movie vs series never merges: %+v", got)
	}
	c := movieItem("It", 2017, dir)
	c.Kind = slotsv1.ItemKind_ITEM_KIND_UNSPECIFIED
	if got := Decide(a, c); got.Merge {
		t.Fatalf("unspecified kinds stay separate: %+v", got)
	}
}

func TestDecideExactnessOfSignalsAndTitles(t *testing.T) {
	t.Parallel()
	// Names compare exactly: casing differences are not the same person.
	a := movieItem("Fargo", 1996, func(m *corev1.TitleMetadata) { m.Directors = []string{"joel coen"} })
	b := movieItem("fargo", 1996, func(m *corev1.TitleMetadata) { m.Directors = []string{"Joel Coen"} })
	if got := Decide(a, b); got.Merge {
		t.Fatalf("signal comparison must be exact: %+v", got)
	}
	// Zero-valued signals cannot match each other.
	c := movieItem("Up", 2009, func(m *corev1.TitleMetadata) {
		m.KindSpecific = &corev1.TitleMetadata_Movie{Movie: &corev1.MovieSpecific{}}
	})
	d := movieItem("Up", 2009, func(m *corev1.TitleMetadata) {
		m.KindSpecific = &corev1.TitleMetadata_Movie{Movie: &corev1.MovieSpecific{}}
	})
	if got := Decide(c, d); got.Merge {
		t.Fatalf("zero runtime is unknown, not a shared duration: %+v", got)
	}
	e := movieItem("Up", 2009, func(m *corev1.TitleMetadata) { m.OriginalLanguage = "" })
	f := movieItem("Up", 2009, func(m *corev1.TitleMetadata) { m.OriginalLanguage = "" })
	if got := Decide(e, f); got.Merge {
		t.Fatalf("empty language is unknown, not a shared language: %+v", got)
	}
}

func TestDecideMissingMetadataStaysSeparate(t *testing.T) {
	t.Parallel()
	a := movieItem("Something", 2001, nil)
	b := Item{Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE} // no metadata at all
	if got := Decide(a, b); got.Merge {
		t.Fatalf("missing metadata never merges: %+v", got)
	}
	empty := Item{}
	if got := Decide(empty, empty); got.Merge {
		t.Fatalf("two empty items stay separate: %+v", got)
	}
	titleless := movieItem("", 2001, func(m *corev1.TitleMetadata) { m.Directors = []string{"X"} })
	if got := Decide(titleless, titleless); got.Merge {
		t.Fatalf("blank titles never merge, even with a signal: %+v", got)
	}
}

func TestSignalString(t *testing.T) {
	t.Parallel()
	for s, want := range map[Signal]string{
		SignalNone:             "none",
		SignalDirector:         "director",
		SignalCastMember:       "cast-member",
		SignalOriginalLanguage: "original-language",
		SignalDuration:         "duration",
		Signal(42):             fmt.Sprintf("Signal(%d)", 42),
	} {
		if got := s.String(); got != want {
			t.Fatalf("Signal(%d).String() = %q, want %q", int(s), got, want)
		}
	}
}
