package slotwiring

import (
	"context"
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/itemregistry"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// TestRegistryResolverNotifiesOnIdentityWork pins the T2 trigger semantics:
// exactly the resolves that did identity work enqueue their entry; pure
// lookups enqueue nothing (TECHNICAL-DECISIONS.md §1.28).
func TestRegistryResolverNotifiesOnIdentityWork(t *testing.T) {
	reg, err := itemregistry.New(store.NewInMemory(), "")
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	var marked []string
	rr := registryResolver{r: reg, notify: func(id string) { marked = append(marked, id) }}
	item := &slotsv1.CatalogueItem{
		NativeId: "m1",
		Kind:     slotsv1.ItemKind_ITEM_KIND_MOVIE,
		Metadata: &corev1.TitleMetadata{Title: "Up", Year: 2009},
	}

	// First resolve: first-seen, an entry is minted — notify once.
	if err := rr.Resolve(context.Background(), "prov", item); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	if len(marked) != 1 {
		t.Fatalf("identity work did not notify; marked=%v", marked)
	}

	// Same proof again: pure lookup — no notification.
	if err := rr.Resolve(context.Background(), "prov", item); err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if len(marked) != 1 {
		t.Fatalf("unchanged mapping notified; marked=%v", marked)
	}

	// Proof evolves on the same entry — that is identity work too.
	evolved := &slotsv1.CatalogueItem{
		NativeId:    "m1",
		Kind:        slotsv1.ItemKind_ITEM_KIND_MOVIE,
		Metadata:    &corev1.TitleMetadata{Title: "Up", Year: 2009},
		ExternalIds: []*slotsv1.ExternalId{{Namespace: "imdb", Value: "tt0382932"}},
	}
	if err := rr.Resolve(context.Background(), "prov", evolved); err != nil {
		t.Fatalf("third resolve: %v", err)
	}
	if len(marked) != 2 {
		t.Fatalf("proof evolution did not notify; marked=%v", marked)
	}
}
