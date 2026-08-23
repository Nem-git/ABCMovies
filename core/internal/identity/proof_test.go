package identity

import (
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

func TestProofOfExtractsIdentityMaterialOnly(t *testing.T) {
	t.Parallel()
	md := &corev1.TitleMetadata{Title: "Heat", Year: 1995, Rating: 8.3, Genres: []string{"Crime"}}
	p := ProofOf(slotsv1.ItemKind_ITEM_KIND_MOVIE, md, []*slotsv1.ExternalId{{Namespace: "imdb", Value: "tt0113243"}})
	if p.Kind != slotsv1.ItemKind_ITEM_KIND_MOVIE || p.Title != "Heat" || p.Year != 1995 {
		t.Fatalf("proof = %+v", p)
	}
	if len(p.ExternalIDs) != 1 || p.ExternalIDs[0].GetNamespace() != "imdb" {
		t.Fatalf("external ids = %v", p.ExternalIDs)
	}
}

func TestSameProof(t *testing.T) {
	t.Parallel()
	base := Proof{
		Kind:        slotsv1.ItemKind_ITEM_KIND_MOVIE,
		Title:       "The Matrix",
		Year:        1999,
		ExternalIDs: []*slotsv1.ExternalId{{Namespace: "imdb", Value: "tt0133093"}},
	}
	mutate := func(f func(*Proof)) Proof {
		p := base
		f(&p)
		return p
	}

	cases := []struct {
		name string
		a, b Proof
		want bool
	}{
		{"identical", base, base, true},
		{"raw title casing differs", base, mutate(func(p *Proof) { p.Title = "the matrix" }), true},
		{"article dropped", base, mutate(func(p *Proof) { p.Title = "Matrix" }), true},
		{
			"id set equal modulo order",
			mutate(func(p *Proof) {
				p.ExternalIDs = []*slotsv1.ExternalId{{Namespace: "imdb", Value: "tt0133093"}, {Namespace: "tmdb", Value: "603"}}
			}),
			mutate(func(p *Proof) {
				p.ExternalIDs = []*slotsv1.ExternalId{{Namespace: "tmdb", Value: "603"}, {Namespace: "imdb", Value: "tt0133093"}}
			}),
			true,
		},
		{"title changed", base, mutate(func(p *Proof) { p.Title = "Matrix Reloaded" }), false},
		{"year changed", base, mutate(func(p *Proof) { p.Year = 2003 }), false},
		{"kind changed", base, mutate(func(p *Proof) { p.Kind = slotsv1.ItemKind_ITEM_KIND_SERIES }), false},
		{
			"id added",
			mutate(func(p *Proof) {
				p.ExternalIDs = []*slotsv1.ExternalId{{Namespace: "imdb", Value: "tt0133093"}}
			}),
			mutate(func(p *Proof) {
				p.ExternalIDs = []*slotsv1.ExternalId{{Namespace: "imdb", Value: "tt0133093"}, {Namespace: "tmdb", Value: "603"}}
			}),
			false,
		},
		{
			"id value changed",
			base,
			mutate(func(p *Proof) { p.ExternalIDs = []*slotsv1.ExternalId{{Namespace: "imdb", Value: "tt0234215"}} }),
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := SameProof(tc.a, tc.b); got != tc.want {
				t.Fatalf("SameProof = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSameProofIgnoresEmptyAssertions(t *testing.T) {
	t.Parallel()
	a := Proof{ExternalIDs: []*slotsv1.ExternalId{nil, {Namespace: "", Value: ""}}}
	b := Proof{}
	if !SameProof(a, b) {
		t.Fatal("empty assertions must not count as identity material")
	}
	if !SameProof(ProofOf(slotsv1.ItemKind_ITEM_KIND_MOVIE, nil, nil), Proof{Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE}) {
		t.Fatal("nil metadata proof equals kind-only proof")
	}
}
