package identity

import (
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// Catalogue-candidate selection (PLAN.md §5.3 applied to enrichment): the
// fallback text lookup returns ranked candidates, and this file decides
// which one — if any — may enrich an entry. A wrongly adopted candidate
// propagates through the global metadata cache, so the bar mirrors the
// merge rule itself: reuse Decide unchanged rather than invent a second
// matching semantics.

// Screen applies only the hard elimination gates between an entry and
// catalogue lookup summaries: kind agreement, normalized title agreement,
// exact movie year when both sides know it, and no contradicting external
// IDs. It deliberately ignores corroborating signals — summaries carry
// none; callers fetch details for survivors and let Adopt decide on full
// records (TECHNICAL-DECISIONS §1.28).
func Screen(entry Item, candidates []Item) []int {
	out := []int{}
	for i, c := range candidates {
		if !sameKind(entry.Kind, c.Kind) {
			continue
		}
		if !titlesMatch(entry.Metadata, c.Metadata) {
			continue
		}
		// Year gates movies only, compared exactly, unknown failing closed
		// (PLAN.md §5.3) — same posture as Decide.
		if entry.Kind == slotsv1.ItemKind_ITEM_KIND_MOVIE &&
			!yearsMatch(entry.Metadata.GetYear(), c.Metadata.GetYear()) {
			continue
		}
		if contradicts(entry.ExternalIDs, c.ExternalIDs) {
			continue
		}
		out = append(out, i)
	}
	return out
}

// contradicts reports whether the two assertion sets disagree inside any
// shared namespace (both non-empty, unequal values). Absent values never
// contradict.
func contradicts(as, bs []*slotsv1.ExternalId) bool {
	for _, x := range as {
		if x == nil || x.GetNamespace() == "" || x.GetValue() == "" {
			continue
		}
		for _, y := range bs {
			if y == nil || y.GetNamespace() != x.GetNamespace() {
				continue
			}
			if y.GetValue() != "" && y.GetValue() != x.GetValue() {
				return true
			}
		}
	}
	return false
}

// Select scores every catalogue candidate against an entry's evidence using
// the same merge rule as provider items: kind agreement, matching
// provider-supplied external ID (corroborated), or normalized title +
// exact year (movies) + at least one ranked signal. The returned verdicts
// are parallel to candidates; non-merging verdicts are zero Verdicts.
//
// Candidates carrying summary-level metadata only (title, year, external
// IDs — what LookupTitle returns) can still pass on corroborated IDs;
// heuristic survival generally needs detail-fetched fields (directors,
// cast, language, runtime), which is why the engine fetches details for
// near-ties before calling Adopt.
func Select(entry Item, candidates []Item) []Verdict {
	verdicts := make([]Verdict, len(candidates))
	for i, c := range candidates {
		verdicts[i] = Decide(entry, c)
	}
	return verdicts
}

// Adopt picks the one candidate allowed to enrich an entry under
// TECHNICAL-DECISIONS §1.28's resolution protocol, or reports abstention.
//
//   - an ID-corroborated candidate wins outright — an identity assertion
//     beats any number of heuristic look-alikes;
//   - more than one ID-corroborated candidate is contradictory data and
//     abstains;
//   - otherwise exactly one heuristic survivor may win;
//   - anything else — zero survivors, ties after scoring — abstains: never
//     guess stays absolute (PLAN.md §5.3).
func Adopt(entry Item, candidates []Item) (picked int, ok bool) {
	verdicts := Select(entry, candidates)

	corroborated := -1
	for i, v := range verdicts {
		if v.Corroborated {
			if corroborated != -1 {
				return -1, false // two candidates assert the same ID
			}
			corroborated = i
		}
	}
	if corroborated != -1 {
		return corroborated, true
	}

	survivors := -1
	for i, v := range verdicts {
		if v.Merge {
			if survivors != -1 {
				return -1, false // genuine tie — abstain, log upstream
			}
			survivors = i
		}
	}
	if survivors == -1 {
		return -1, false
	}
	return survivors, true
}
