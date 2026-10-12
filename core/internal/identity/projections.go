package identity

import (
	"strings"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// This file holds the ONLY places that project a producer's shape into an
// Item: one conversion per producer shape, so a field the matcher cares
// about (kind, external IDs, title, year, corroborating metadata) can never
// be dropped in one caller and kept in another. The drift test walks each
// feeding proto descriptor and fails when a new corroborating field is not
// copied here — and when a deliberate exclusion is not declared.

// FromSlotItem projects a provider catalogue item into the identity view
// (the shape the registry matches on at resolve time).
func FromSlotItem(item *slotsv1.CatalogueItem) Item {
	if item == nil {
		return Item{}
	}
	return Item{
		Kind:        item.GetKind(),
		ExternalIDs: item.GetExternalIds(),
		Metadata:    item.GetMetadata(),
	}
}

// FromEvidence projects an entry's accumulated identity material (the core's
// own record of what an entry claims) into the identity view. Both the
// enrichment entry evidence and the registry's stored entry record use it:
// they are the same producer shape — kind, external IDs, metadata.
func FromEvidence(kind slotsv1.ItemKind, externalIDs []*slotsv1.ExternalId, md *corev1.TitleMetadata) Item {
	return Item{
		Kind:        kind,
		ExternalIDs: externalIDs,
		Metadata:    md,
	}
}

// FromLookupCandidate projects a lookup hit into the identity view. The
// candidate's canonical ref is parsed back into namespace:value form so a
// ref the catalogue asserts counts as an identity assertion too, and the
// original title stands in as an alt title when it differs from the display
// title — that is exactly the localization case the field exists for
// (contract comment on TitleCandidate.original_title; matching semantics in
// TECHNICAL-DECISIONS.md §1.29).
func FromLookupCandidate(c *slotsv1.TitleCandidate) Item {
	if c == nil {
		return Item{}
	}
	title := c.GetTitle()
	alt := c.GetOriginalTitle()
	item := Item{
		Kind:        c.GetKind(),
		ExternalIDs: claimedIDs(c),
		Metadata:    &corev1.TitleMetadata{Title: title, Year: c.GetYear()},
	}
	if alt != "" && alt != title {
		item.AltTitles = []string{alt}
	}
	return item
}

// FromDetail projects a detail-fetched survivor (the candidate plus the
// catalogue's full metadata response) into the identity view. The response
// owns the metadata: it is the whole point of the second fetch. A nil
// response yields no metadata — the projection never manufactures one from
// the lookup summary, which holds only title and year.
func FromDetail(c *slotsv1.TitleCandidate, resp *slotsv1.GetMetadataResponse) Item {
	if c == nil {
		if resp == nil {
			return Item{}
		}
		return Item{
			ExternalIDs: resp.GetExternalIds(),
			Metadata:    resp.GetMetadata(),
		}
	}
	return Item{
		Kind:        c.GetKind(),
		ExternalIDs: append(claimedIDs(c), resp.GetExternalIds()...),
		Metadata:    resp.GetMetadata(),
	}
}

// FromProof rebuilds the comparison view of an item from stored proof,
// letting the registry match against entries without keeping full metadata.
func FromProof(p Proof) Item {
	return Item{
		Kind:        p.Kind,
		ExternalIDs: p.ExternalIDs,
		Metadata:    &corev1.TitleMetadata{Title: p.Title, Year: p.Year},
	}
}

// claimedIDs renders a candidate's own assertions, including its canonical
// ref parsed back into namespace:value form.
func claimedIDs(c *slotsv1.TitleCandidate) []*slotsv1.ExternalId {
	if c == nil {
		return nil
	}
	ids := append([]*slotsv1.ExternalId{}, c.GetExternalIds()...)
	if ns, val, ok := strings.Cut(c.GetRef(), ":"); ok && ns != "" && val != "" {
		ids = append(ids, &slotsv1.ExternalId{Namespace: ns, Value: val})
	}
	return ids
}
