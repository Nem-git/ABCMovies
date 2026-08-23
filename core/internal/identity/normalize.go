// Package identity implements title matching (PLAN.md §5.3): deciding
// whether two provider items are the same movie or series. The merge rule is
// deliberately conservative — a matching provider-supplied external ID alone
// is an identity assertion and merges; anything else must agree on the
// normalized title (plus year for movies) and at least one corroborating
// signal before two items merge, and otherwise they stay separate.
package identity

import (
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// DefaultArticles is the leading-article list used when Options.Articles is
// nil. The list is configurable by design (PLAN.md §5.3).
var DefaultArticles = []string{"the", "a", "an"}

// Options tunes title normalization.
type Options struct {
	// Articles are the lowercase leading articles dropped from titles before
	// comparison. An article is only dropped when something follows it, so a
	// bare article normalizes to itself. Nil selects DefaultArticles.
	Articles []string
}

func (o Options) articles() []string {
	if o.Articles == nil {
		return DefaultArticles
	}
	return o.Articles
}

// NormalizeTitle normalizes a title with default options.
func NormalizeTitle(title string) string {
	return Options{}.NormalizeTitle(title)
}

// NormalizeTitle reduces a title to its comparable form: Unicode NFKD
// normalization, diacritics stripped, lowercased, configured leading article
// dropped, whitespace collapsed. The result is compared exactly — there is no
// fuzzy scoring anywhere in this package.
func (o Options) NormalizeTitle(title string) string {
	decomposed := norm.NFKD.String(title)
	var b strings.Builder
	b.Grow(len(decomposed))
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) { // combining mark: the visible base letter stays
			continue
		}
		b.WriteRune(r)
	}
	fields := strings.Fields(strings.ToLower(b.String()))
	if len(fields) > 1 && slices.Contains(o.articles(), fields[0]) {
		fields = fields[1:]
	}
	return strings.Join(fields, " ")
}
