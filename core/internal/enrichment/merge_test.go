package enrichment

import (
	"testing"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"google.golang.org/protobuf/proto"
)

var now = time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)

func md(mutate func(*corev1.TitleMetadata)) *corev1.TitleMetadata {
	m := &corev1.TitleMetadata{}
	if mutate != nil {
		mutate(m)
	}
	return m
}

// ownerOf fetches the provenance slot string for one path.
func ownerOf(t *testing.T, m *corev1.TitleMetadata, path string) string {
	t.Helper()
	src, ok := m.Source[path]
	if !ok {
		t.Fatalf("no provenance recorded for %q in %+v", path, m.Source)
	}
	return src.GetSlot()
}

func TestNilCurrentClaimsEverythingContributed(t *testing.T) {
	out, err := Merge(nil, Contribution{
		Slot: "tmdb",
		Kind: SourceCatalogue,
		Metadata: md(func(m *corev1.TitleMetadata) {
			m.Title = "The Matrix"
			m.Year = 1999
			m.Genres = []string{"Science Fiction"}
			m.KindSpecific = &corev1.TitleMetadata_Movie{Movie: &corev1.MovieSpecific{RuntimeMinutes: 136}}
			m.Extra = map[string]string{"tmdb.tagline": "Free your mind"}
		}),
	}, now)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	for _, p := range []string{"base.title", "base.year", "base.genres", "movie.runtime_minutes", "extra.tmdb.tagline"} {
		if got := ownerOf(t, out, p); got != "catalogue:tmdb" {
			t.Fatalf("%q owned by %q", p, got)
		}
	}
	if out.Source["base.title"].GetFetchedAt().AsTime() != now {
		t.Fatal("fetched_at not stamped with the passed time")
	}
}

func TestProviderFillsGapsButNeverOverwritesCatalogue(t *testing.T) {
	current, err := Merge(nil, Contribution{Slot: "tmdb", Kind: SourceCatalogue, Metadata: md(func(m *corev1.TitleMetadata) {
		m.Title = "The Matrix"
		m.Rating = 8.7
	})}, now)
	if err != nil {
		t.Fatalf("catalogue merge: %v", err)
	}

	later := now.Add(time.Hour)
	out, err := Merge(current, Contribution{Slot: "home-jellyfin", Kind: SourceProvider, Metadata: md(func(m *corev1.TitleMetadata) {
		m.Title = "Matrix (Jellyfin local name)"
		m.Directors = []string{"Lana Wachowski"}
	})}, later)
	if err != nil {
		t.Fatalf("provider merge: %v", err)
	}
	if out.GetTitle() != "The Matrix" || ownerOf(t, out, "base.title") != "catalogue:tmdb" {
		t.Fatalf("provider overwrote catalogue title: %q owned by %q", out.GetTitle(), ownerOf(t, out, "base.title"))
	}
	if len(out.GetDirectors()) != 1 || ownerOf(t, out, "base.directors") != "provider:home-jellyfin" {
		t.Fatalf("provider gap-fill failed: %v", out.GetDirectors())
	}
}

func TestCatalogueTakesOverProviderField(t *testing.T) {
	current, err := Merge(nil, Contribution{Slot: "home-jellyfin", Kind: SourceProvider, Metadata: md(func(m *corev1.TitleMetadata) {
		m.Rating = 7.9
	})}, now)
	if err != nil {
		t.Fatalf("provider merge: %v", err)
	}
	out, err := Merge(current, Contribution{Slot: "tmdb", Kind: SourceCatalogue, Metadata: md(func(m *corev1.TitleMetadata) {
		m.Rating = 8.7
	})}, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("catalogue merge: %v", err)
	}
	if out.GetRating() != 8.7 || ownerOf(t, out, "base.rating") != "catalogue:tmdb" {
		t.Fatalf("catalogue did not take over: rating=%v owner=%q", out.GetRating(), ownerOf(t, out, "base.rating"))
	}
}

func TestSecondCatalogueNeverTakesOverFirst(t *testing.T) {
	current, err := Merge(nil, Contribution{Slot: "tmdb", Kind: SourceCatalogue, Metadata: md(func(m *corev1.TitleMetadata) {
		m.PosterUrl = "https://image.tmdb.org/t/p/w500/abc.jpg"
	})}, now)
	if err != nil {
		t.Fatalf("first catalogue: %v", err)
	}
	out, err := Merge(current, Contribution{Slot: "tvmaze", Kind: SourceCatalogue, Metadata: md(func(m *corev1.TitleMetadata) {
		m.PosterUrl = "https://static.tvmaze.com/xyz.jpg"
		m.Description = "Episode guides and schedules."
	})}, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("second catalogue: %v", err)
	}
	if out.GetPosterUrl() != "https://image.tmdb.org/t/p/w500/abc.jpg" {
		t.Fatalf("second catalogue stole the poster: %q", out.GetPosterUrl())
	}
	if out.GetDescription() == "" || ownerOf(t, out, "base.description") != "catalogue:tvmaze" {
		t.Fatal("second catalogue could not claim an unowned field")
	}
}

func TestSlotRefreshesItsOwnFields(t *testing.T) {
	current, err := Merge(nil, Contribution{Slot: "tmdb", Kind: SourceCatalogue, Metadata: md(func(m *corev1.TitleMetadata) {
		m.Rating = 8.5
	})}, now)
	if err != nil {
		t.Fatalf("first merge: %v", err)
	}
	later := now.Add(24 * time.Hour)
	out, err := Merge(current, Contribution{Slot: "tmdb", Kind: SourceCatalogue, Metadata: md(func(m *corev1.TitleMetadata) {
		m.Rating = 8.7
	})}, later)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if out.GetRating() != 8.7 {
		t.Fatalf("own refresh not applied: %v", out.GetRating())
	}
	if out.Source["base.rating"].GetFetchedAt().AsTime() != later {
		t.Fatal("refresh did not bump fetched_at")
	}
}

func TestAbsentFieldsClaimNothingAndClearNothing(t *testing.T) {
	current, err := Merge(nil, Contribution{Slot: "tmdb", Kind: SourceCatalogue, Metadata: md(func(m *corev1.TitleMetadata) {
		m.Title = "Up"
		m.Year = 2009
		m.Rating = 8.3
		m.Cast = []string{"Ed Asner"}
	})}, now)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	out, err := Merge(current, Contribution{Slot: "home-jellyfin", Kind: SourceProvider, Metadata: md(func(m *corev1.TitleMetadata) {
		m.Title = "" // absent
		m.Year = 0   // absent
		m.Rating = 0 // absent — must not wipe
		m.OriginalLanguage = "en"
	})}, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if out.GetTitle() != "Up" || out.GetYear() != 2009 || out.GetRating() != 8.3 {
		t.Fatalf("absent fields clobbered values: %+v", out)
	}
	if _, ok := out.Source["base.original_language"]; !ok {
		t.Fatal("populated field was not claimed")
	}
	if len(out.Source) != 5 {
		t.Fatalf("unexpected provenance growth: %+v", out.Source)
	}
}

func TestExtraMapMergesPerKeyWithSameRules(t *testing.T) {
	current, err := Merge(nil, Contribution{Slot: "tmdb", Kind: SourceCatalogue, Metadata: md(func(m *corev1.TitleMetadata) {
		m.Extra = map[string]string{"tmdb.tagline": "Free your mind", "shared.key": "from-catalogue"}
	})}, now)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	out, err := Merge(current, Contribution{Slot: "home-jellyfin", Kind: SourceProvider, Metadata: md(func(m *corev1.TitleMetadata) {
		m.Extra = map[string]string{"jellyfin.path": "/library/movies/matrix", "shared.key": "from-provider"}
	})}, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if out.GetExtra()["tmdb.tagline"] != "Free your mind" {
		t.Fatal("catalogue extra lost")
	}
	if out.GetExtra()["jellyfin.path"] != "/library/movies/matrix" {
		t.Fatal("provider extra not added")
	}
	if out.GetExtra()["shared.key"] != "from-catalogue" {
		t.Fatalf("provider stole contested extra key: %q", out.GetExtra()["shared.key"])
	}
}

func TestSeriesPathsStayIndependentlyOwnable(t *testing.T) {
	current, err := Merge(nil, Contribution{Slot: "guide-a", Kind: SourceCatalogue, Metadata: md(func(m *corev1.TitleMetadata) {
		m.KindSpecific = &corev1.TitleMetadata_Series{Series: &corev1.SeriesSpecific{TotalSeasons: 3}}
	})}, now)
	if err != nil {
		t.Fatalf("seasons seed: %v", err)
	}
	out, err := Merge(current, Contribution{Slot: "guide-b", Kind: SourceCatalogue, Metadata: md(func(m *corev1.TitleMetadata) {
		m.KindSpecific = &corev1.TitleMetadata_Series{Series: &corev1.SeriesSpecific{TotalEpisodes: 24}}
	})}, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("episodes merge: %v", err)
	}
	s := out.GetSeries()
	if s.GetTotalSeasons() != 3 || s.GetTotalEpisodes() != 24 {
		t.Fatalf("series halves merged wrong: %+v", s)
	}
	if ownerOf(t, out, "series.total_seasons") != "catalogue:guide-a" ||
		ownerOf(t, out, "series.total_episodes") != "catalogue:guide-b" {
		t.Fatalf("series provenance wrong: %+v", out.Source)
	}

	// A seasons-only refresh from guide-a must preserve guide-b's episodes.
	out2, err := Merge(out, Contribution{Slot: "guide-a", Kind: SourceCatalogue, Metadata: md(func(m *corev1.TitleMetadata) {
		m.KindSpecific = &corev1.TitleMetadata_Series{Series: &corev1.SeriesSpecific{TotalSeasons: 4}}
	})}, now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if out2.GetSeries().GetTotalSeasons() != 4 || out2.GetSeries().GetTotalEpisodes() != 24 {
		t.Fatalf("refresh clobbered sibling half: %+v", out2.GetSeries())
	}
}

func TestMergeLeavesInputsUnchanged(t *testing.T) {
	current := md(func(m *corev1.TitleMetadata) { m.Title = "Original"; m.Rating = 1 })
	contrib := md(func(m *corev1.TitleMetadata) { m.Title = "Incoming"; m.Cast = []string{"X"} })
	before := proto.Clone(current).(*corev1.TitleMetadata)
	inBefore := proto.Clone(contrib).(*corev1.TitleMetadata)

	if _, err := Merge(current, Contribution{Slot: "s", Kind: SourceProvider, Metadata: contrib}, now); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if !proto.Equal(before, current) || !proto.Equal(inBefore, contrib) {
		t.Fatal("Merge mutated an input")
	}
}

func TestInvalidContributionsRejected(t *testing.T) {
	if _, err := Merge(nil, Contribution{Slot: "", Kind: SourceCatalogue, Metadata: md(nil)}, now); err == nil {
		t.Fatal("empty slot id accepted")
	}
	if _, err := Merge(nil, Contribution{Slot: "tmdb", Kind: SourceCatalogue, Metadata: nil}, now); err == nil {
		t.Fatal("nil metadata accepted")
	}
}
