package identity

import (
	"sort"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// Proof is the evidence that fixed a provider item's identity at resolution
// time (PLAN.md §5.3): its kind, title, year, and provider-supplied external
// IDs. The registry stores one proof per mapping; a refresh compares the live
// item against it and re-resolves only when the proof no longer matches.
type Proof struct {
	Kind        slotsv1.ItemKind
	Title       string
	Year        uint32
	ExternalIDs []*slotsv1.ExternalId
}

// ProofOf extracts the proof from a provider item's identity material.
func ProofOf(kind slotsv1.ItemKind, md *corev1.TitleMetadata, externalIDs []*slotsv1.ExternalId) Proof {
	p := Proof{Kind: kind, ExternalIDs: append([]*slotsv1.ExternalId(nil), externalIDs...)}
	if md != nil {
		p.Title = md.GetTitle()
		p.Year = md.GetYear()
	}
	return p
}

// SameProof reports whether two proofs describe the same identity material:
// equal kinds, equal normalized titles (a pure casing or article change is
// not an identity change), exactly equal years, and identical external-ID
// sets — order and duplicates ignored.
func SameProof(a, b Proof) bool {
	if a.Kind != b.Kind || NormalizeTitle(a.Title) != NormalizeTitle(b.Title) || a.Year != b.Year {
		return false
	}
	return sameIDSet(a.ExternalIDs, b.ExternalIDs)
}

func sameIDSet(as, bs []*slotsv1.ExternalId) bool {
	keys := func(ids []*slotsv1.ExternalId) []string {
		out := make([]string, 0, len(ids))
		for _, id := range ids {
			if id == nil || id.GetNamespace() == "" || id.GetValue() == "" {
				continue
			}
			out = append(out, id.GetNamespace()+"\x00"+id.GetValue())
		}
		sort.Strings(out)
		return out
	}
	aKeys, bKeys := keys(as), keys(bs)
	if len(aKeys) != len(bKeys) {
		return false
	}
	for i := range aKeys {
		if aKeys[i] != bKeys[i] {
			return false
		}
	}
	return true
}

// ItemFromProof rebuilds the comparison view of an item from stored proof,
// letting the registry match against entries without keeping full metadata.
func (p Proof) Item() Item {
	return Item{
		Kind:        p.Kind,
		ExternalIDs: p.ExternalIDs,
		Metadata:    &corev1.TitleMetadata{Title: p.Title, Year: p.Year},
	}
}
