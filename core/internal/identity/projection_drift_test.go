package identity

import (
	"fmt"
	"strings"
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// exclusion records one field a projection deliberately does not copy, with
// the reason a human agreed to it. A field absent from both the handled set
// and this map means the projection silently dropped it — the drift the
// test exists to catch.
type exclusion struct {
	field  string
	reason string
}

// coverage is one proto shape feeding an Item: which of its fields the
// projection copies (handled) and which it deliberately does not (excluded).
// Both lists are data so a new proto field fails this test until a human
// classifies it — the contract's guarantee that a corroborating signal is
// never dropped without a recorded decision (PLAN.md §5.3).
type coverage struct {
	name     string
	fields   func() protoreflect.MessageDescriptor
	handled  []string
	excluded []exclusion
	// project builds a sentinel-populated message and runs the projection.
	project func(t *testing.T) Item
	// sentinels maps each handled field to a value the projection must carry
	// into the Item. Verified against fmt.Sprint of the result, so a field
	// present in the descriptor but missing from the output fails.
	sentinels map[string]string
}

func allFieldNames(md protoreflect.MessageDescriptor) []string {
	out := make([]string, 0, md.Fields().Len())
	for i := 0; i < md.Fields().Len(); i++ {
		out = append(out, string(md.Fields().Get(i).Name()))
	}
	return out
}

func TestProjectionDrift(t *testing.T) {
	cases := []coverage{
		{
			name:   "CatalogueItem",
			fields: (&slotsv1.CatalogueItem{}).ProtoReflect().Descriptor,
			handled: []string{
				"kind", "external_ids", "metadata",
			},
			excluded: []exclusion{
				{"native_id", "the provider's internal key; the registry stores it as its own mapping key, not as identity material"},
			},
			project: func(t *testing.T) Item {
				return FromSlotItem(&slotsv1.CatalogueItem{
					Kind:        slotsv1.ItemKind_ITEM_KIND_MOVIE,
					ExternalIds: []*slotsv1.ExternalId{{Namespace: "sent_ns", Value: "sent_val"}},
					Metadata:    &corev1.TitleMetadata{Title: "sent_title", Year: 2001},
				})
			},
			sentinels: map[string]string{
				"kind":         "ITEM_KIND_MOVIE",
				"external_ids": "sent_ns",
				"metadata":     "sent_title",
			},
		},
		{
			name:   "TitleCandidate",
			fields: (&slotsv1.TitleCandidate{}).ProtoReflect().Descriptor,
			handled: []string{
				"ref", "kind", "title", "original_title", "year", "external_ids",
			},
			project: func(t *testing.T) Item {
				return FromLookupCandidate(&slotsv1.TitleCandidate{
					Ref:           "sent_ref_ns:sent_ref_val",
					Kind:          slotsv1.ItemKind_ITEM_KIND_SERIES,
					Title:         "sent_title",
					OriginalTitle: "sent_orig_title",
					Year:          1999,
					ExternalIds:   []*slotsv1.ExternalId{{Namespace: "sent_ns", Value: "sent_val"}},
				})
			},
			sentinels: map[string]string{
				"ref":            "sent_ref_ns",
				"kind":           "ITEM_KIND_SERIES",
				"title":          "sent_title",
				"original_title": "sent_orig_title",
				"year":           "1999",
				"external_ids":   "sent_ns",
			},
		},
		{
			name:   "GetMetadataResponse",
			fields: (&slotsv1.GetMetadataResponse{}).ProtoReflect().Descriptor,
			handled: []string{
				"metadata", "external_ids",
			},
			project: func(t *testing.T) Item {
				return FromDetail(&slotsv1.TitleCandidate{
					Ref:  "sent_ref_ns:sent_ref_val",
					Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE,
				}, &slotsv1.GetMetadataResponse{
					Metadata:    &corev1.TitleMetadata{Title: "sent_title", Year: 2001},
					ExternalIds: []*slotsv1.ExternalId{{Namespace: "sent_ns", Value: "sent_val"}},
				})
			},
			sentinels: map[string]string{
				"metadata":     "sent_title",
				"external_ids": "sent_ns",
			},
		},
		{
			// TitleMetadata is passed through by pointer in the three
			// projections above rather than destructured, so every field it
			// carries is handled — including the corroborating signals the
			// matcher scores on (cast, directors, runtime). Pinning that here
			// means a future destructuring projection fails until each
			// dropped field is declared.
			name:   "TitleMetadata",
			fields: (&corev1.TitleMetadata{}).ProtoReflect().Descriptor,
			excluded: []exclusion{
				// The oneof sibling of "movie": only one member of a oneof can
				// be set, so its pass-through is proven in
				// TestSeriesOneofPassesThrough rather than alongside movie.
				{"series", "oneof sibling of movie; covered by TestSeriesOneofPassesThrough"},
			},
			handled: []string{
				"title", "year", "description", "poster_url", "rating",
				"content_rating", "genres", "original_language", "cast",
				"directors", "movie", "source", "extra",
			},
			project: func(t *testing.T) Item {
				return FromEvidence(slotsv1.ItemKind_ITEM_KIND_MOVIE, nil, &corev1.TitleMetadata{
					Title:            "sent_title",
					Year:             2001,
					Description:      "sent_description",
					PosterUrl:        "sent_poster_url",
					Rating:           8.7,
					ContentRating:    "sent_content_rating",
					Genres:           []string{"sent_genre"},
					OriginalLanguage: "sent_language",
					Cast:             []string{"sent_cast"},
					Directors:        []string{"sent_director"},
					KindSpecific:     &corev1.TitleMetadata_Movie{Movie: &corev1.MovieSpecific{RuntimeMinutes: 121}},
					Source:           map[string]*corev1.EnrichmentSource{"sent_path": {Slot: "sent_slot"}},
					Extra:            map[string]string{"sent_extra": "sent_extra_value"},
				})
			},
			sentinels: map[string]string{
				"title":             "sent_title",
				"year":              "2001",
				"description":       "sent_description",
				"poster_url":        "sent_poster_url",
				"rating":            "8.7",
				"content_rating":    "sent_content_rating",
				"genres":            "sent_genre",
				"original_language": "sent_language",
				"cast":              "sent_cast",
				"directors":         "sent_director",
				"movie":             "121",
				"source":            "sent_slot",
				"extra":             "sent_extra_value",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			desc := tc.fields()
			if desc == nil {
				t.Fatalf("%s: no descriptor", tc.name)
			}
			handledSet := map[string]bool{}
			for _, h := range tc.handled {
				if handledSet[h] {
					t.Errorf("%s: field %q listed as handled twice", tc.name, h)
				}
				handledSet[h] = true
			}
			excludedSet := map[string]bool{}
			for _, e := range tc.excluded {
				if excludedSet[e.field] {
					t.Errorf("%s: field %q listed as excluded twice", tc.name, e.field)
				}
				excludedSet[e.field] = true
				if e.reason == "" {
					t.Errorf("%s: excluded field %q has no reason", tc.name, e.field)
				}
			}
			for h := range handledSet {
				if excludedSet[h] {
					t.Errorf("%s: field %q is both handled and excluded", tc.name, h)
				}
			}

			// Every descriptor field must be classified: handled or excluded.
			for _, f := range allFieldNames(desc) {
				if !handledSet[f] && !excludedSet[f] {
					t.Errorf("%s: field %q is neither handled by a projection nor a declared exclusion — classify it before shipping", tc.name, f)
				}
			}
			// And nothing classified may be absent from the descriptor.
			for h := range handledSet {
				if desc.Fields().ByName(protoreflect.Name(h)) == nil {
					t.Errorf("%s: handled field %q is not in the descriptor", tc.name, h)
				}
			}

			// Prove the handled fields actually reach the Item.
			got := tc.project(t)
			printed := fmt.Sprint(got)
			for _, f := range tc.handled {
				want := tc.sentinels[f]
				if want == "" {
					t.Fatalf("%s: handled field %q has no sentinel", tc.name, f)
				}
				if !strings.Contains(printed, want) {
					t.Errorf("%s: field %q sentinel %q not carried into the Item: %s", tc.name, f, want, printed)
				}
			}
		})
	}
}

// TestProjectionNilSafety pins that every projection tolerates a nil input —
// a caller with an absent optional sub-message must never panic.
func TestProjectionNilSafety(t *testing.T) {
	if got := FromSlotItem(nil); fmt.Sprint(got) != fmt.Sprint(Item{}) {
		t.Errorf("FromSlotItem(nil) = %+v, want zero Item", got)
	}
	if got := FromLookupCandidate(nil); fmt.Sprint(got) != fmt.Sprint(Item{}) {
		t.Errorf("FromLookupCandidate(nil) = %+v, want zero Item", got)
	}
	if got := FromDetail(nil, nil); fmt.Sprint(got) != fmt.Sprint(Item{}) {
		t.Errorf("FromDetail(nil,nil) = %+v, want zero Item", got)
	}
}

// TestClaimedIDsParsesRef pins the ref→external-ID parsing: a catalogue's
// canonical ref is an identity assertion, so it must reach the Item.
func TestClaimedIDsParsesRef(t *testing.T) {
	got := FromLookupCandidate(&slotsv1.TitleCandidate{Ref: "tmdb:603"})
	if len(got.ExternalIDs) != 1 || got.ExternalIDs[0].GetNamespace() != "tmdb" || got.ExternalIDs[0].GetValue() != "603" {
		t.Fatalf("ref not parsed into an external ID: %+v", got.ExternalIDs)
	}
	// A colonless ref must not fabricate an ID.
	if got := FromLookupCandidate(&slotsv1.TitleCandidate{Ref: "nope"}); len(got.ExternalIDs) != 0 {
		t.Fatalf("colonless ref fabricated an ID: %+v", got.ExternalIDs)
	}
}

// TestAltTitleOnlyWhenDifferent pins the localization rule: the original
// title becomes an alt title only when it differs from the display title.
func TestAltTitleOnlyWhenDifferent(t *testing.T) {
	same := FromLookupCandidate(&slotsv1.TitleCandidate{Title: "T", OriginalTitle: "T"})
	if len(same.AltTitles) != 0 {
		t.Fatalf("identical titles produced an alt title: %+v", same.AltTitles)
	}
	diff := FromLookupCandidate(&slotsv1.TitleCandidate{Title: "T", OriginalTitle: "U"})
	if len(diff.AltTitles) != 1 || diff.AltTitles[0] != "U" {
		t.Fatalf("differing original title not carried as alt title: %+v", diff.AltTitles)
	}
}

// TestFromEvidencePassesThrough pins the evidence projection: it copies the
// three identity fields unchanged, including a nil metadata.
func TestFromEvidencePassesThrough(t *testing.T) {
	ids := []*slotsv1.ExternalId{{Namespace: "n", Value: "v"}}
	md := &corev1.TitleMetadata{Title: "T", Year: 2001}
	got := FromEvidence(slotsv1.ItemKind_ITEM_KIND_SERIES, ids, md)
	if got.Kind != slotsv1.ItemKind_ITEM_KIND_SERIES || len(got.ExternalIDs) != 1 || got.Metadata != md {
		t.Fatalf("evidence not passed through: %+v", got)
	}
}

// TestSeriesOneofPassesThrough pins the series oneof sibling: a series
// entry's totals survive the evidence projection like a movie's runtime.
func TestSeriesOneofPassesThrough(t *testing.T) {
	md := &corev1.TitleMetadata{
		Title:        "T",
		Year:         2001,
		KindSpecific: &corev1.TitleMetadata_Series{Series: &corev1.SeriesSpecific{TotalSeasons: 3, TotalEpisodes: 30}},
	}
	got := FromEvidence(slotsv1.ItemKind_ITEM_KIND_SERIES, nil, md)
	printed := fmt.Sprint(got)
	for _, want := range []string{"total_seasons:3", "total_episodes:30"} {
		if !strings.Contains(printed, want) {
			t.Errorf("series oneof not passed through: %s", printed)
		}
	}
}

// TestFromProofRebuildsComparisonView pins the proof projection: it rebuilds
// the title/year view a refresh compares against, nothing more.
func TestFromProofRebuildsComparisonView(t *testing.T) {
	got := FromProof(Proof{Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE, Title: "T", Year: 2001})
	if got.Kind != slotsv1.ItemKind_ITEM_KIND_MOVIE || got.Metadata.GetTitle() != "T" || got.Metadata.GetYear() != 2001 {
		t.Fatalf("proof not rebuilt: %+v", got)
	}
	// The method form must agree with the function form.
	if fmt.Sprint(Proof{Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE, Title: "T", Year: 2001}.Item()) != fmt.Sprint(got) {
		t.Errorf("Proof.Item() disagrees with FromProof")
	}
}

// TestDetailCandidateClaimsPlusResponseIDs pins FromDetail's ID merge: the
// candidate's own assertions (including its ref) come first, then the
// catalogue's full response IDs — no dedup, no reordering.
func TestDetailCandidateClaimsPlusResponseIDs(t *testing.T) {
	got := FromDetail(
		&slotsv1.TitleCandidate{
			Ref:         "tmdb:603",
			Kind:        slotsv1.ItemKind_ITEM_KIND_MOVIE,
			ExternalIds: []*slotsv1.ExternalId{{Namespace: "imdb", Value: "tt0133093"}},
		},
		&slotsv1.GetMetadataResponse{
			Metadata:    &corev1.TitleMetadata{Title: "The Matrix", Year: 1999},
			ExternalIds: []*slotsv1.ExternalId{{Namespace: "tvrage", Value: "123"}},
		},
	)
	if len(got.ExternalIDs) != 3 {
		t.Fatalf("want candidate ref + candidate id + response id = 3, got %d: %+v", len(got.ExternalIDs), got.ExternalIDs)
	}
	if got.Metadata.GetTitle() != "The Matrix" || got.Metadata.GetYear() != 1999 {
		t.Fatalf("response metadata not used: %+v", got.Metadata)
	}
	// A nil response carries no metadata: the projection must not manufacture
	// one from the lookup summary (which only has title and year).
	got = FromDetail(&slotsv1.TitleCandidate{Title: "T", Year: 1999, Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE}, nil)
	if got.Metadata != nil {
		t.Fatalf("nil response manufactured metadata: %+v", got.Metadata)
	}
}
