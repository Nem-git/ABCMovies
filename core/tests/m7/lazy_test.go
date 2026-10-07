// Package m7 holds the milestone acceptance tests for M7's first slice: the
// lazy streaming-service provider contract, proven on the built-in mock
// adapter (docs/IMPLEMENTATION.md §3, PLAN.md §5.4). The real Archive.org
// adapter exercises the same contract over HTTP in the smoke script, never in
// CI.
package m7_test

import (
	"context"
	"strings"
	"testing"

	"github.com/nem-git/abcmovies/adapters/mocklazy"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

func catalogue() []mocklazy.Item {
	return []mocklazy.Item{
		{NativeID: "m1", Title: "Big Buck Bunny", Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE, Year: 2008, Available: true},
		{NativeID: "m2", Title: "Sintel", Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE, Year: 2010, Available: true},
		{NativeID: "s1", Title: "Tears of Steel", Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE, Year: 2012, Available: false},
	}
}

// A lazy provider must never answer a whole-catalogue sync: it has no index.
// Any core that requests one has misclassified the provider, and the adapter
// rejects loudly rather than degrading to an empty catalogue (§2.5).
func TestCatalogueSyncIsRefused(t *testing.T) {
	s := mocklazy.New(catalogue())
	_, err := s.CatalogueSync(context.Background(), &slotsv1.CatalogueSyncRequest{AccountId: "acct"})
	if err == nil || !strings.Contains(err.Error(), "not a lazy-provider capability") {
		t.Fatalf("CatalogueSync = %v, want lazy refusal", err)
	}
}

// Usage is the only refresh: browsing and searching the provider drive the
// diff against the registry. Nothing probes the provider in the background
// (§5.4): the only calls this milestone issues are the ones a user made.
func TestUsageIsTheRefresh(t *testing.T) {
	s := mocklazy.New(catalogue())
	for name := range map[string]bool{"CatalogueSync": true, "SearchCatalog": true, "BrowseCatalog": true, "RefreshAvailability": true} {
		if n := s.Calls(name); n != 0 {
			t.Fatalf("precondition: %s called %d times before any user action", name, n)
		}
	}
	resp, err := s.SearchCatalog(context.Background(), &slotsv1.SearchCatalogRequest{AccountId: "acct", Query: "sintel"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetItems()) != 1 || resp.GetItems()[0].GetNativeId() != "m2" {
		t.Fatalf("search = %v, want m2 only", resp.GetItems())
	}
	if got := s.Calls("SearchCatalog"); got != 1 {
		t.Fatalf("SearchCatalog calls = %d, want 1", got)
	}
	if got := s.Calls("CatalogueSync"); got != 0 {
		t.Fatalf("background-class CatalogueSync calls = %d, want 0", got)
	}
}

// RefreshAvailability is the explicit, user-triggered pure lookup: it reports
// which requested items are still present, and while the provider is marked
// degraded it fails rather than probing (§5.4 scheduler rules).
func TestRefreshAvailabilityIsManualAndPausesWhenDegraded(t *testing.T) {
	s := mocklazy.New(catalogue())
	s.RemoveItem("m2")
	res, err := s.RefreshAvailability(context.Background(), &slotsv1.RefreshAvailabilityRequest{
		AccountId: "acct", NativeIds: []string{"m1", "m2", "m3"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.GetItems()) != 1 || res.GetItems()[0].GetNativeId() != "m1" {
		t.Fatalf("refresh = %v, want m1 only", res.GetItems())
	}
	s.SetDegraded(true)
	if _, err := s.RefreshAvailability(context.Background(), &slotsv1.RefreshAvailabilityRequest{
		AccountId: "acct", NativeIds: []string{"m1"},
	}); err == nil {
		t.Fatal("degraded provider must refuse refreshes, not probe")
	}
}

// produce-sources is the click/play confirmation: playing an item confirms
// its availability and returns the manifest.
func TestProduceSourcesConfirmsAtClick(t *testing.T) {
	s := mocklazy.New(catalogue())
	res, err := s.ProduceSources(context.Background(), &slotsv1.ProduceSourcesRequest{AccountId: "acct", NativeId: "m1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.GetSource() == nil || len(res.GetSource().GetTracks()) == 0 {
		t.Fatal("expected a manifest with tracks")
	}
	if _, err := s.ProduceSources(context.Background(), &slotsv1.ProduceSourcesRequest{AccountId: "acct", NativeId: "s1"}); err == nil {
		t.Fatal("delisted item must not resolve")
	}
}

// Negative: requests without account_id are rejected outright (§5.4: an
// unknown account is a runtime error, never an implicit page).
func TestMissingAccountRejected(t *testing.T) {
	s := mocklazy.New(catalogue())
	if _, err := s.SearchCatalog(context.Background(), &slotsv1.SearchCatalogRequest{Query: "x"}); err == nil {
		t.Fatal("search without account_id must fail")
	}
	if _, err := s.RefreshAvailability(context.Background(), &slotsv1.RefreshAvailabilityRequest{}); err == nil {
		t.Fatal("refresh without account_id must fail")
	}
}
