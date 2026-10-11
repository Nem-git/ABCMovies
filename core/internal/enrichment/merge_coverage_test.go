package enrichment

import (
	"strings"
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// The merge copies a contribution's typed fields by hand: each path in
// typedFields names one field it knows how to copy. That list is only as
// complete as whoever last remembered to extend it — a new corroborating or
// display field on TitleMetadata would silently never merge, with no error
// and no failing test, just mysteriously absent metadata.
//
// This test closes that hole: it walks the TitleMetadata descriptor and
// requires every field to be accounted for — either copied by typedFields or
// named in the deliberate exclusions below with a reason. Adding a field to
// the contract fails this test until a human classifies it.
func TestTypedFieldsCoversEveryMetadataField(t *testing.T) {
	// Fields the merge deliberately does not copy, each with the reason.
	// "source" and "extra" are the contract's own non-content maps, handled
	// by the merge's provenance logic rather than field-by-field copying.
	excluded := map[string]string{
		"source": "per-field provenance map; the merge writes provenance, never copies it as a value",
		"extra":  "opaque adapter-specific bag; copied wholesale by the merge, not per typed field",
	}

	fields := typedFields(&corev1.TitleMetadata{})
	covered := map[string]int{}  // base field name -> entries copying it
	prefixed := map[string]int{} // "movie"/"series" -> sub-field entries under it
	for _, f := range fields {
		if head, sub, ok := strings.Cut(f.path, "."); ok && head != "base" && sub != "" {
			prefixed[head]++
			continue
		}
		covered[baseName(f.path)]++
	}

	// Sub-field paths are checked exhaustively by the companion test below;
	// here each oneof member only needs to be covered at all.
	for head := range prefixed {
		if covered[head] != 0 {
			t.Errorf("kind-specific field %q is covered both directly (%d) and by sub-field paths", head, covered[head])
		}
	}

	allFields := (&corev1.TitleMetadata{}).ProtoReflect().Descriptor().Fields()
	for i := 0; i < allFields.Len(); i++ {
		fd := allFields.Get(i)
		name := string(fd.Name())

		if reason, ok := excluded[name]; ok {
			if reason == "" {
				t.Errorf("field %q is excluded with an empty reason; every exclusion must say why", name)
			}
			if covered[name] != 0 || prefixed[name] != 0 {
				t.Errorf("field %q is listed as excluded but typedFields also copies it", name)
			}
			continue
		}

		if fd.ContainingOneof() == nil {
			// A base field: exactly one typedFields entry must copy it.
			if n := covered[name]; n != 1 {
				t.Errorf("base field %q is covered by %d typedFields entries, want exactly 1 "+
					"(add it to typedFields, or to the exclusions with a reason)", name, n)
			}
			continue
		}
		// The kind_specific oneof: movie and series carry their own
		// sub-fields, covered as "movie.<sub>" paths — at least one must
		// exist, and each sub-field's own coverage is the companion test's
		// job. A oneof member with no sub-field paths at all would mean the
		// merge silently drops that whole kind.
		if prefixed[name] == 0 {
			t.Errorf("kind-specific field %q has no typedFields entries under the %q. prefix", name, name)
		}
	}

	// No path may name a field that doesn't exist: a typo'd path silently
	// never matches, so the merge quietly stops copying that field — the
	// same silent failure, one step later.
	for _, f := range fields {
		head, sub, ok := strings.Cut(f.path, ".")
		if !ok || head == "base" || sub == "" {
			// base.<field> (or a bare field): the field lives on TitleMetadata.
			if !hasField(&corev1.TitleMetadata{}, baseName(f.path)) {
				t.Errorf("typedFields path %q does not correspond to any TitleMetadata field", f.path)
			}
			continue
		}
		// movie.<sub> / series.<sub>: the head must be a kind-specific
		// oneof member and the tail one of its sub-message's fields.
		subMsg, okKind := kindSpecificMessages[head]
		if !okKind {
			t.Errorf("typedFields path %q prefixes fields under unknown kind %q", f.path, head)
			continue
		}
		if !hasField(subMsg, sub) {
			t.Errorf("typedFields path %q does not correspond to any %s field", f.path, head)
		}
	}
}

// kindSpecificMessages maps each kind_specific oneof member name to an
// instance of its sub-message, so a path prefix can be resolved to the
// descriptor its tail must exist on.
var kindSpecificMessages = map[string]proto.Message{
	"movie":  &corev1.MovieSpecific{},
	"series": &corev1.SeriesSpecific{},
}

// hasField reports whether the message carries a field of the given name.
func hasField(m proto.Message, name string) bool {
	fields := m.ProtoReflect().Descriptor().Fields()
	fd := fields.ByName(protoreflect.Name(name))
	// ByName returns an invalid (empty) descriptor for an unknown name.
	return fd != nil && fd.FullName() != ""
}

// The kind-specific sub-fields live under their parent's prefix, so walk
// those descriptors too: runtime_minutes, total_seasons, total_episodes are
// ordinary fields of their own messages and deserve the same guarantee.
func TestTypedFieldsCoversKindSpecificSubfields(t *testing.T) {
	covered := map[string]int{}
	for _, f := range typedFields(&corev1.TitleMetadata{}) {
		covered[f.path]++
	}

	movieFields := (&corev1.MovieSpecific{}).ProtoReflect().Descriptor().Fields()
	for i := 0; i < movieFields.Len(); i++ {
		path := "movie." + string(movieFields.Get(i).Name())
		if covered[path] != 1 {
			t.Errorf("movie sub-field %q is covered by %d typedFields entries, want exactly 1", path, covered[path])
		}
	}
	seriesFields := (&corev1.SeriesSpecific{}).ProtoReflect().Descriptor().Fields()
	for i := 0; i < seriesFields.Len(); i++ {
		path := "series." + string(seriesFields.Get(i).Name())
		if covered[path] != 1 {
			t.Errorf("series sub-field %q is covered by %d typedFields entries, want exactly 1", path, covered[path])
		}
	}
}

// baseName strips the "base." prefix the universal fields are listed under.
// A path without that prefix (the movie./series. sub-fields) is returned
// unchanged so it can be matched against its own message's fields.
func baseName(path string) string {
	return strings.TrimPrefix(path, "base.")
}
