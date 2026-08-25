// Package enrichment realizes PLAN.md §5.2: best-effort per-title metadata
// from optional catalogue slots (preferred) and providers (fallback), merged
// field by field with per-field provenance, cached globally per title.
package enrichment

import (
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// SourceKind ranks a contributor the way PLAN.md §5.2 does: catalogue data
// is preferred (globally authoritative); provider data fills the rest.
type SourceKind int

const (
	SourceProvider SourceKind = iota
	SourceCatalogue
)

// String renders the provenance prefix the TitleMetadata.source contract
// documents ("catalogue:tmdb", "provider:jellyfin").
func (k SourceKind) String() string {
	switch k {
	case SourceCatalogue:
		return "catalogue"
	default:
		return "provider"
	}
}

// Contribution is one slot's offered metadata for one title.
type Contribution struct {
	// Slot is the contributor's slot instance id (TECHNICAL-DECISIONS §1.25).
	Slot string
	Kind SourceKind
	// Metadata is what the slot asserts; absent fields claim nothing and
	// clear nothing.
	Metadata *corev1.TitleMetadata
}

// owner renders the provenance identity stored in TitleMetadata.source:
// "<kind>:<slot-instance-id>", per the contract's documented examples.
func (c Contribution) owner() string {
	return c.Kind.String() + ":" + c.Slot
}

// Merge folds one contribution into the current record field by field
// (PLAN.md §5.2) and returns the result; neither input is mutated. Rules per
// field path:
//
//   - absent contributions claim nothing and never clear;
//   - a slot always refreshes the fields it already owns;
//   - a catalogue field takes over a provider-owned field;
//   - otherwise the first slot to claim a field at its tier keeps it — two
//     catalogues never fight over one field, so no single catalogue owns an
//     entry.
//
// Provenance is rewritten for every claimed or refreshed path; fetched_at is
// stamped with now so tests stay deterministic.
func Merge(current *corev1.TitleMetadata, c Contribution, now time.Time) (*corev1.TitleMetadata, error) {
	if strings.TrimSpace(c.Slot) == "" {
		return nil, fmt.Errorf("enrichment: contribution slot id is required")
	}
	if c.Metadata == nil {
		return nil, fmt.Errorf("enrichment: contribution from %q carries no metadata", c.Slot)
	}

	src := c.Metadata
	if current == nil {
		out := proto.Clone(src).(*corev1.TitleMetadata)
		out.Source = map[string]*corev1.EnrichmentSource{}
		for _, f := range typedFields(src) {
			if f.present() {
				stamp(out, f.path, c, now)
			}
		}
		for _, key := range sortedKeys(src.GetExtra()) {
			stamp(out, extraPath(key), c, now)
		}
		return out, nil
	}

	out := proto.Clone(current).(*corev1.TitleMetadata)
	for _, f := range typedFields(src) {
		f.mergeInto(out, c, now)
	}
	for _, key := range sortedKeys(src.GetExtra()) {
		mergeExtra(out, key, src.GetExtra()[key], c, now)
	}
	return out, nil
}

// typedField is one metadata path of the incoming contribution.
type typedField struct {
	path string
	// present reports whether the contribution populates this path.
	present func() bool
	// write copies the contribution's value into dst.
	write func(dst *corev1.TitleMetadata)
}

func typedFields(src *corev1.TitleMetadata) []typedField {
	return []typedField{
		{
			path:    "base.title",
			present: func() bool { return src.GetTitle() != "" },
			write:   func(dst *corev1.TitleMetadata) { dst.Title = src.GetTitle() },
		},
		{
			path:    "base.year",
			present: func() bool { return src.GetYear() != 0 },
			write:   func(dst *corev1.TitleMetadata) { dst.Year = src.GetYear() },
		},
		{
			path:    "base.description",
			present: func() bool { return src.GetDescription() != "" },
			write:   func(dst *corev1.TitleMetadata) { dst.Description = src.GetDescription() },
		},
		{
			path:    "base.poster_url",
			present: func() bool { return src.GetPosterUrl() != "" },
			write:   func(dst *corev1.TitleMetadata) { dst.PosterUrl = src.GetPosterUrl() },
		},
		{
			path:    "base.rating",
			present: func() bool { return src.GetRating() != 0 },
			write:   func(dst *corev1.TitleMetadata) { dst.Rating = src.GetRating() },
		},
		{
			path:    "base.content_rating",
			present: func() bool { return src.GetContentRating() != "" },
			write:   func(dst *corev1.TitleMetadata) { dst.ContentRating = src.GetContentRating() },
		},
		{
			path:    "base.genres",
			present: func() bool { return len(src.GetGenres()) > 0 },
			write:   func(dst *corev1.TitleMetadata) { dst.Genres = cloneStrings(src.GetGenres()) },
		},
		{
			path:    "base.original_language",
			present: func() bool { return src.GetOriginalLanguage() != "" },
			write:   func(dst *corev1.TitleMetadata) { dst.OriginalLanguage = src.GetOriginalLanguage() },
		},
		{
			path:    "base.cast",
			present: func() bool { return len(src.GetCast()) > 0 },
			write:   func(dst *corev1.TitleMetadata) { dst.Cast = cloneStrings(src.GetCast()) },
		},
		{
			path:    "base.directors",
			present: func() bool { return len(src.GetDirectors()) > 0 },
			write:   func(dst *corev1.TitleMetadata) { dst.Directors = cloneStrings(src.GetDirectors()) },
		},
		{
			path:    "movie.runtime_minutes",
			present: func() bool { return src.GetMovie().GetRuntimeMinutes() != 0 },
			write:   func(dst *corev1.TitleMetadata) { patchMovie(dst, src.GetMovie().GetRuntimeMinutes()) },
		},
		{
			path:    "series.total_seasons",
			present: func() bool { return src.GetSeries().GetTotalSeasons() != 0 },
			write: func(dst *corev1.TitleMetadata) {
				patchSeries(dst, src.GetSeries().GetTotalSeasons(), dst.GetSeries().GetTotalEpisodes())
			},
		},
		{
			path:    "series.total_episodes",
			present: func() bool { return src.GetSeries().GetTotalEpisodes() != 0 },
			write: func(dst *corev1.TitleMetadata) {
				patchSeries(dst, dst.GetSeries().GetTotalSeasons(), src.GetSeries().GetTotalEpisodes())
			},
		},
	}
}

// mergeInto applies this field's ownership rules to dst.
func (f typedField) mergeInto(dst *corev1.TitleMetadata, c Contribution, now time.Time) {
	if !f.present() {
		return // absent claims nothing, clears nothing
	}
	switch takeOver(dst.Source[f.path], c) {
	case true:
		f.write(dst)
		stamp(dst, f.path, c, now)
	default:
		// keep the current owner's value
	}
}

// patchSeries writes one series half (seasons or episodes) while preserving
// the other, so the two paths stay independently ownable inside the shared
// oneof wrapper.
func patchSeries(dst *corev1.TitleMetadata, totalSeasons, totalEpisodes uint32) {
	var s *corev1.SeriesSpecific
	if cur := dst.GetSeries(); cur != nil {
		s = proto.Clone(cur).(*corev1.SeriesSpecific)
	} else {
		s = &corev1.SeriesSpecific{}
	}
	s.TotalSeasons = totalSeasons
	s.TotalEpisodes = totalEpisodes
	dst.KindSpecific = &corev1.TitleMetadata_Series{Series: s}
}

func patchMovie(dst *corev1.TitleMetadata, runtime uint32) {
	dst.KindSpecific = &corev1.TitleMetadata_Movie{Movie: &corev1.MovieSpecific{RuntimeMinutes: runtime}}
}

// takeOver decides whether an incoming contribution may write a path given
// its current provenance entry (possibly nil): unclaimed paths are claimed,
// owners refresh their own claims, catalogue takes over provider, and any
// other incumbent wins.
func takeOver(owner *corev1.EnrichmentSource, c Contribution) bool {
	if owner == nil || owner.GetSlot() == "" {
		return true
	}
	if owner.GetSlot() == c.owner() {
		return true
	}
	return c.Kind == SourceCatalogue && strings.HasPrefix(owner.GetSlot(), SourceProvider.String()+":")
}

func extraPath(key string) string { return "extra." + key }

// mergeExtra applies the ownership rules to one extra key.
func mergeExtra(dst *corev1.TitleMetadata, key, val string, c Contribution, now time.Time) {
	if !takeOver(dst.Source[extraPath(key)], c) {
		return
	}
	if dst.Extra == nil {
		dst.Extra = map[string]string{}
	}
	dst.Extra[key] = val
	stamp(dst, extraPath(key), c, now)
}

// stamp attributes one path to the contributor.
func stamp(m *corev1.TitleMetadata, path string, c Contribution, now time.Time) {
	if m.Source == nil {
		m.Source = map[string]*corev1.EnrichmentSource{}
	}
	m.Source[path] = &corev1.EnrichmentSource{Slot: c.owner(), FetchedAt: timestamppb.New(now)}
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
