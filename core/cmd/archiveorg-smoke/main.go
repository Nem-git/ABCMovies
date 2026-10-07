// Command archiveorg-smoke runs a manual, operator-side verification of the
// generic Stremio adapter against the public Archive.org addon manifest —
// the real lazy-provider check M7 asks for (PLAN.md §5.4). It is never wired
// into CI: the fixture suite (adapters/stremio) is the CI gate; this script
// is the same pattern as the documented manual verification paths for
// DRM after M8.
//
// Usage:
//
//	go run ./core/cmd/archiveorg-smoke \
//	  --manifest-url https://stremio-archive-org-addon.fly.dev/manifest.json
//
// It browses one catalogue page, searches "bunny", refreshes the first item
// it found, and reports the stream manifest for the first search hit.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/nem-git/abcmovies/adapters/stremio"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

func main() {
	manifestURL := flag.String("manifest-url", "https://stremio-archive-org-addon.fly.dev/manifest.json", "addon manifest URL")
	flag.Parse()
	ctx := context.Background()
	slot, err := stremio.New([]string{"archiveorg"}, singleAccount{url: *manifestURL})
	if err != nil {
		log.Fatalf("build slot: %v", err)
	}
	browse, err := slot.BrowseCatalog(ctx, &slotsv1.BrowseCatalogRequest{AccountId: "archiveorg"})
	if err != nil {
		log.Fatalf("browse: %v", err)
	}
	fmt.Printf("browse: %d items\n", len(browse.GetItems()))
	search, err := slot.SearchCatalog(ctx, &slotsv1.SearchCatalogRequest{AccountId: "archiveorg", Query: "bunny"})
	if err != nil {
		log.Fatalf("search: %v", err)
	}
	fmt.Printf("search 'bunny': %d items\n", len(search.GetItems()))
	if len(search.GetItems()) == 0 {
		fmt.Fprintln(os.Stderr, "no search hits; synergy check passed up to search")
		os.Exit(0)
	}
	first := search.GetItems()[0]
	fmt.Printf("first hit: %s (%s)\n", first.GetMetadata().GetTitle(), first.GetNativeId())
	refresh, err := slot.RefreshAvailability(ctx, &slotsv1.RefreshAvailabilityRequest{
		AccountId: "archiveorg", NativeIds: []string{first.GetNativeId()},
	})
	if err != nil {
		log.Fatalf("refresh: %v", err)
	}
	fmt.Printf("refresh: %d present\n", len(refresh.GetItems()))
	sources, err := slot.ProduceSources(ctx, &slotsv1.ProduceSourcesRequest{AccountId: "archiveorg", NativeId: first.GetNativeId()})
	if err != nil {
		log.Fatalf("produce-sources: %v", err)
	}
	tracks := sources.GetSource().GetTracks()
	fmt.Printf("produce-sources: %d track(s); first url: %s\n", len(tracks), tracks[0].GetDelivery().GetLocations()[0])
}

type singleAccount struct{ url string }

func (s singleAccount) Lookup(_ context.Context, _ string) (stremio.Account, error) {
	return stremio.Account{ID: "archiveorg", ManifestURL: s.url}, nil
}
