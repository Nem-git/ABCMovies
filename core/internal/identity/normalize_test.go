package identity

import "testing"

func TestNormalizeTitle(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"whitespace only", "   \t ", ""},
		{"collapsed whitespace", "  Up   \t Close  ", "up close"},
		{"lowercased", "CITIZEN KANE", "citizen kane"},
		{"diacritics stripped", "Amélie", "amelie"},
		{"nfkd compatibility forms", "ﬁlm №1", "film no1"},
		{"leading the dropped", "The Matrix", "matrix"},
		{"leading a dropped", "A Bug's Life", "bug's life"},
		{"leading an case-insensitive", "AN AMERICAN TAIL", "american tail"},
		{"bare article kept", "The", "the"},
		{"interior article kept", "The Good, the Bad and the Ugly", "good, the bad and the ugly"},
		{"non-article first word kept", "There Will Be Blood", "there will be blood"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := NormalizeTitle(tc.in); got != tc.want {
				t.Fatalf("NormalizeTitle(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeTitleConfigurableArticles(t *testing.T) {
	t.Parallel()
	opts := Options{Articles: []string{"les"}}
	if got := opts.NormalizeTitle("Les Misérables"); got != "miserables" {
		t.Fatalf("custom articles: got %q, want %q", got, "miserables")
	}
	// English defaults stay in place alongside configured ones.
	opts.Articles = []string{"les", DefaultArticles[0]}
	if got := opts.NormalizeTitle("The Matrix"); got != "matrix" {
		t.Fatalf("mixed articles: got %q, want %q", got, "matrix")
	}
}

func TestNormalizeTitleIsIdempotent(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"The Matrix", "Amélie", "", "ﬁlm"} {
		once := NormalizeTitle(in)
		if twice := NormalizeTitle(once); twice != once {
			t.Fatalf("not idempotent for %q: %q -> %q", in, once, twice)
		}
	}
}
