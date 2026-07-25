package search_test

import (
	"testing"

	"github.com/nem-git/abcmovies/internal/oas"
	"github.com/nem-git/abcmovies/internal/search"
)

func movie(name string) oas.SearchResultItem {
	return oas.SearchResultItem{
		Resource: oas.SearchResultItemResource{
			Type:  oas.MovieSearchResultItemResource,
			Movie: oas.Movie{ID: "m1", Name: name},
		},
	}
}

func series(name string) oas.SearchResultItem {
	return oas.SearchResultItem{
		Resource: oas.SearchResultItemResource{
			Type:   oas.SeriesSearchResultItemResource,
			Series: oas.Series{ID: "s1", Name: name},
		},
	}
}

func service(name string) oas.SearchResultItem {
	return oas.SearchResultItem{
		Resource: oas.SearchResultItemResource{
			Type:    oas.ServiceSearchResultItemResource,
			Service: oas.Service{Tag: "svc", Name: name},
		},
	}
}

func item(tag string, item oas.SearchResultItem) search.Result {
	return search.Result{Tag: tag, Item: item}
}

func TestScoreAndSort_EmptyQuery(t *testing.T) {
	items := []search.Result{
		item("a", movie("Batman")),
		item("a", movie("Superman")),
	}
	out := search.ScoreAndSort("", items)
	if len(out) != 2 {
		t.Fatalf("expected 2 results, got %d", len(out))
	}
}

func TestScoreAndSort_EmptyResults(t *testing.T) {
	out := search.ScoreAndSort("batman", nil)
	if len(out) != 0 {
		t.Fatalf("expected 0 results, got %d", len(out))
	}
}

func TestScoreAndSort_ExactMatch(t *testing.T) {
	items := []search.Result{
		item("a", movie("Superman")),
		item("a", movie("Batman")),
	}
	out := search.ScoreAndSort("Batman", items)
	if len(out) != 2 {
		t.Fatalf("expected 2 results, got %d", len(out))
	}
	if out[0].Item.Resource.Movie.Name != "Batman" {
		t.Errorf("expected Batman first, got %s", out[0].Item.Resource.Movie.Name)
	}
	if out[0].Item.Score != 1.0 {
		t.Errorf("exact match score = %f, want 1.0", out[0].Item.Score)
	}
}

func TestScoreAndSort_CaseInsensitive(t *testing.T) {
	items := []search.Result{
		item("a", movie("batman")),
	}
	out := search.ScoreAndSort("BATMAN", items)
	if out[0].Item.Score != 1.0 {
		t.Errorf("case-insensitive exact score = %f, want 1.0", out[0].Item.Score)
	}
}

func TestScoreAndSort_PrefixMatch(t *testing.T) {
	items := []search.Result{
		item("a", movie("Superman Returns")),
		item("a", movie("Batman Begins")),
	}
	out := search.ScoreAndSort("bat", items)
	if out[0].Item.Resource.Movie.Name != "Batman Begins" {
		t.Errorf("expected Batman Begins first, got %s", out[0].Item.Resource.Movie.Name)
	}
	if out[0].Item.Score != 0.9 {
		t.Errorf("prefix match score = %f, want 0.9", out[0].Item.Score)
	}
}

func TestScoreAndSort_ContainsMatch(t *testing.T) {
	items := []search.Result{
		item("a", movie("The Dark Knight")),
		item("a", movie("Superman")),
	}
	out := search.ScoreAndSort("knight", items)
	if out[0].Item.Resource.Movie.Name != "The Dark Knight" {
		t.Errorf("expected The Dark Knight first, got %s", out[0].Item.Resource.Movie.Name)
	}
	if out[0].Item.Score != 0.7 {
		t.Errorf("contains match score = %f, want 0.7", out[0].Item.Score)
	}
}

func TestScoreAndSort_FuzzyMatch(t *testing.T) {
	items := []search.Result{
		item("a", movie("Superman")),
		item("a", movie("Spider-Man")),
	}
	out := search.ScoreAndSort("Batm", items)
	if out[0].Item.Score <= 0 {
		t.Errorf("fuzzy match score = %f, expected > 0", out[0].Item.Score)
	}
}

func TestScoreAndSort_MixedResourceTypes(t *testing.T) {
	items := []search.Result{
		item("a", series("Batman")),
		item("a", movie("Batman")),
		item("a", service("Batman")),
	}
	out := search.ScoreAndSort("Batman", items)
	if len(out) != 3 {
		t.Fatalf("expected 3 results, got %d", len(out))
	}
	for _, r := range out {
		if r.Item.Score != 1.0 {
			t.Errorf("expected 1.0 for exact match, got %f for %s", r.Item.Score, r.Tag)
		}
	}
}

func TestScoreAndSort_SortOrder(t *testing.T) {
	items := []search.Result{
		item("a", movie("Inception")),
		item("a", movie("Batman Returns")),
		item("a", movie("The Batman")),
	}
	out := search.ScoreAndSort("Batman", items)
	if out[0].Item.Score < out[1].Item.Score {
		t.Errorf("expected higher score first: %f < %f", out[0].Item.Score, out[1].Item.Score)
	}
}

func TestResourceName(t *testing.T) {
	tests := []struct {
		name string
		item oas.SearchResultItem
		want string
	}{
		{"movie", movie("Batman"), "Batman"},
		{"series", series("The Office"), "The Office"},
		{"service", service("Netflix"), "Netflix"},
		{"unknown", oas.SearchResultItem{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := search.ResourceName(tt.item)
			if got != tt.want {
				t.Errorf("ResourceName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterByType(t *testing.T) {
	items := []search.Result{
		item("a", movie("Batman")),
		item("a", series("The Office")),
		item("a", service("Netflix")),
	}

	t.Run("empty types returns all", func(t *testing.T) {
		out := search.FilterByType(items, nil)
		if len(out) != 3 {
			t.Errorf("expected 3 results, got %d", len(out))
		}
	})

	t.Run("filter movie only", func(t *testing.T) {
		out := search.FilterByType(items, []string{"movie"})
		if len(out) != 1 {
			t.Fatalf("expected 1 result, got %d", len(out))
		}
		if !out[0].Item.Resource.IsMovie() {
			t.Errorf("expected movie, got %v", out[0].Item.Resource.Type)
		}
	})

	t.Run("filter movie and series", func(t *testing.T) {
		out := search.FilterByType(items, []string{"movie", "series"})
		if len(out) != 2 {
			t.Fatalf("expected 2 results, got %d", len(out))
		}
	})

	t.Run("filter service only", func(t *testing.T) {
		out := search.FilterByType(items, []string{"service"})
		if len(out) != 1 {
			t.Fatalf("expected 1 result, got %d", len(out))
		}
		if !out[0].Item.Resource.IsService() {
			t.Errorf("expected service, got %v", out[0].Item.Resource.Type)
		}
	})

	t.Run("no matching types returns empty", func(t *testing.T) {
		out := search.FilterByType(items, []string{"episode"})
		if len(out) != 0 {
			t.Errorf("expected 0 results, got %d", len(out))
		}
	})
}
