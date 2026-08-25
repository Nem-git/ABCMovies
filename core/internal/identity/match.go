package identity

import (
	"fmt"
	"slices"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// Signal names which corroborating heuristic carried a heuristic merge.
// Ranked in declaration order: the first matching signal wins and is reported
// so merge provenance can name its weakest link (PLAN.md §5.3). Duration is
// the weakest signal — theatrical and director's cuts differ.
type Signal int

const (
	SignalNone Signal = iota
	SignalDirector
	SignalCastMember
	SignalOriginalLanguage
	SignalDuration
)

func (s Signal) String() string {
	switch s {
	case SignalNone:
		return "none"
	case SignalDirector:
		return "director"
	case SignalCastMember:
		return "cast-member"
	case SignalOriginalLanguage:
		return "original-language"
	case SignalDuration:
		return "duration"
	default:
		return fmt.Sprintf("Signal(%d)", int(s))
	}
}

// Verdict is what Decide concluded about two provider items.
type Verdict struct {
	// Merge is true when the two items are decided to be the same title.
	Merge bool
	// Corroborated is true when the merge rests on a matching
	// provider-supplied external ID — an identity assertion, not a heuristic
	// (PLAN.md §5.3).
	Corroborated bool
	// Signal names the corroborating heuristic that matched when Merge is
	// true and Corroborated is false; SignalNone otherwise.
	Signal Signal
}

// Item is one side of a comparison: a provider item's kind, its
// provider-supplied identity assertions, and its metadata. Matching reads
// only the corroborating fields below — poster, rating, genres, content
// rating and description are inert by construction and can never drive or
// block a merge (PLAN.md §5.3).
type Item struct {
	Kind        slotsv1.ItemKind
	ExternalIDs []*slotsv1.ExternalId
	Metadata    *corev1.TitleMetadata
}

// Decide applies the merge rule to two provider items. Kinds must always
// agree — a movie and a series never merge, even when a provider-supplied
// external ID matches (that situation is contradictory data, handled
// upstream as a conflict). Between same-kind items, a matching
// provider-supplied external ID merges outright; otherwise they merge only
// when normalized titles agree (plus exact year for movies) and at least one
// ranked signal matches exactly. Everything else stays separate —
// conservative matching favors unlinked duplicates over wrong merges
// (PLAN.md §5.3). Merging is never destructive: callers record the verdict
// as proof in the provider item registry.
func Decide(a, b Item) Verdict {
	if !sameKind(a.Kind, b.Kind) {
		return Verdict{}
	}
	if corroborationMatch(a.ExternalIDs, b.ExternalIDs) {
		return Verdict{Merge: true, Corroborated: true}
	}
	if !titlesMatch(a.Metadata, b.Metadata) {
		return Verdict{}
	}
	// Year gates movies only: compared exactly, unknown years fail. Series
	// ignore years entirely (PLAN.md §5.3).
	if a.Kind == slotsv1.ItemKind_ITEM_KIND_MOVIE && !yearsMatch(a.Metadata.GetYear(), b.Metadata.GetYear()) {
		return Verdict{}
	}
	signal := firstSignal(a.Metadata, b.Metadata)
	if signal == SignalNone {
		return Verdict{}
	}
	return Verdict{Merge: true, Signal: signal}
}

// corroborationMatch reports whether any namespace+value pair matches
// exactly. Empty namespaces/values never match, even against each other.
func corroborationMatch(as, bs []*slotsv1.ExternalId) bool {
	for _, x := range as {
		if x == nil || x.GetNamespace() == "" || x.GetValue() == "" {
			continue
		}
		for _, y := range bs {
			if y == nil || y.GetNamespace() != x.GetNamespace() || y.GetValue() != x.GetValue() {
				continue
			}
			return true
		}
	}
	return false
}

// sameKind requires equal, specified kinds: a movie and a series never merge,
// and unspecified kinds (rejected upstream) stay separate here too.
func sameKind(a, b slotsv1.ItemKind) bool {
	if a == slotsv1.ItemKind_ITEM_KIND_UNSPECIFIED || b == slotsv1.ItemKind_ITEM_KIND_UNSPECIFIED {
		return false
	}
	return a == b
}

// titlesMatch compares normalized titles exactly. Missing or empty titles
// never match.
func titlesMatch(a, b *corev1.TitleMetadata) bool {
	if a == nil || b == nil {
		return false
	}
	na, nb := NormalizeTitle(a.GetTitle()), NormalizeTitle(b.GetTitle())
	return na != "" && nb != "" && na == nb
}

// yearsMatch requires both years known (non-zero) and equal.
func yearsMatch(a, b uint32) bool {
	return a > 0 && b > 0 && a == b
}

// firstSignal walks the ranked signals director → shared cast member →
// original language → duration and returns the first that matches exactly.
// Person and language comparisons are exact strings; unknown values (empty
// language, zero runtime) cannot match.
func firstSignal(a, b *corev1.TitleMetadata) Signal {
	if sharesAny(a.GetDirectors(), b.GetDirectors()) {
		return SignalDirector
	}
	if sharesAny(a.GetCast(), b.GetCast()) {
		return SignalCastMember
	}
	if la := a.GetOriginalLanguage(); la != "" && la == b.GetOriginalLanguage() {
		return SignalOriginalLanguage
	}
	if ra := a.GetMovie().GetRuntimeMinutes(); ra > 0 && ra == b.GetMovie().GetRuntimeMinutes() {
		return SignalDuration
	}
	return SignalNone
}

// sharesAny reports whether the two lists share an exact member.
func sharesAny(xs, ys []string) bool {
	if len(xs) == 0 || len(ys) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(xs))
	for _, x := range xs {
		set[x] = struct{}{}
	}
	return slices.ContainsFunc(ys, func(y string) bool { _, ok := set[y]; return ok })
}
