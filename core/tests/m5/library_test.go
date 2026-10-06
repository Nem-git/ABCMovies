package m5_test

import (
	"testing"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// libraryEntryKindCounts tallies a page's entries by kind, for assertions
// that the derived view merged the seeded provider's items correctly.
func libraryEntryKindCounts(items []*apiv1.LibraryItem) (movies, series int) {
	for _, it := range items {
		switch it.GetEntry().GetKind() {
		case corev1.EntryKind_ENTRY_KIND_MOVIE:
			movies++
		case corev1.EntryKind_ENTRY_KIND_SERIES:
			series++
		}
	}
	return movies, series
}

// TestM5LibraryBrowseMergedFromLinkedAccount proves the derived library over
// the wire (PLAN.md §5, §8.1): alice's linked-account reach yields the
// provider's three titles as a single merged view — two movies and one series
// — each carrying corroborated external identities and the cached display
// record; a full browse fits one page and the next_token stays empty.
func TestM5LibraryBrowseMergedFromLinkedAccount(t *testing.T) {
	stack := newM5Stack(t, fakeJellyfinServer(t))
	client := apiv1.NewCoreServiceClient(startWireServer(t, stack))
	aliceCtx := authedCtx(t.Context(), stack.aliceToken)

	resp, err := client.GetLibrary(aliceCtx, &apiv1.GetLibraryRequest{})
	if err != nil {
		t.Fatalf("GetLibrary: %v", err)
	}
	if len(resp.GetItems()) != 3 {
		t.Fatalf("alice's library has %d entries, want 3 (two movies, one series)", len(resp.GetItems()))
	}
	if resp.GetNextPageToken() != "" {
		t.Errorf("next_page_token = %q, want empty for a 3-item page at page bound %d", resp.GetNextPageToken(), libraryPageSize)
	}
	if movies, series := libraryEntryKindCounts(resp.GetItems()); movies != 2 || series != 1 {
		t.Errorf("library kinds = %d movies, %d series; want 2 and 1", movies, series)
	}

	sawGondwana := false
	for _, it := range resp.GetItems() {
		if it.GetEntry().GetId() == "" {
			t.Errorf("entry with empty id: %v", it)
		}
		if it.GetMetadata() == nil || it.GetMetadata().GetTitle() == "" {
			t.Errorf("entry %q has no cached display record", it.GetEntry().GetId())
		}
		for _, id := range it.GetEntry().GetExternalIdentities() {
			if id.GetNamespace() == "imdb" && id.GetValue() == "tt-gondwana" {
				sawGondwana = true
			}
		}
		if it.GetEntry().GetCoverage() == nil || len(it.GetEntry().GetCoverage()) == 0 {
			t.Errorf("entry %q has no coverage rows", it.GetEntry().GetId())
		}
	}
	if !sawGondwana {
		t.Error("no entry carries the corroborated imdb identity tt-gondwana")
	}

	// bob has no reachable account: his view of the same instance is empty.
	bob, err := client.GetLibrary(authedCtx(t.Context(), stack.bobToken), &apiv1.GetLibraryRequest{})
	if err != nil {
		t.Fatalf("bob GetLibrary: %v", err)
	}
	if len(bob.GetItems()) != 0 {
		t.Fatalf("bob's library has %d entries, want 0 (his own private link is not derivable to him)", len(bob.GetItems()))
	}
}

// TestM5LibraryQueryFiltersByMetadataAndIdentity proves the query surface of
// GetLibrary (PLAN.md §8.1): the free-text query matches the cached title and
// the corroborated identity values, and a miss filters everything out.
func TestM5LibraryQueryFiltersByMetadataAndIdentity(t *testing.T) {
	stack := newM5Stack(t, fakeJellyfinServer(t))
	client := apiv1.NewCoreServiceClient(startWireServer(t, stack))
	aliceCtx := authedCtx(t.Context(), stack.aliceToken)

	byTitle, err := client.GetLibrary(aliceCtx, &apiv1.GetLibraryRequest{Query: "gondwana"})
	if err != nil {
		t.Fatalf("GetLibrary(title): %v", err)
	}
	if len(byTitle.GetItems()) != 1 {
		t.Fatalf("title query matched %d entries, want 1", len(byTitle.GetItems()))
	}
	if got := byTitle.GetItems()[0].GetMetadata().GetTitle(); got != "The Last Gondwana Gardener" {
		t.Errorf("title query matched %q, want The Last Gondwana Gardener", got)
	}

	byIdentity, err := client.GetLibrary(aliceCtx, &apiv1.GetLibraryRequest{Query: "tt-gondwana"})
	if err != nil {
		t.Fatalf("GetLibrary(identity): %v", err)
	}
	if len(byIdentity.GetItems()) != 1 {
		t.Fatalf("identity query matched %d entries, want 1", len(byIdentity.GetItems()))
	}

	noMatch, err := client.GetLibrary(aliceCtx, &apiv1.GetLibraryRequest{Query: "zzz-no-such-title"})
	if err != nil {
		t.Fatalf("GetLibrary(no-match): %v", err)
	}
	if len(noMatch.GetItems()) != 0 {
		t.Fatalf("no-match query yielded %d entries, want 0", len(noMatch.GetItems()))
	}
}

// TestM5LibraryInvalidPageTokenRejected proves the page-token validation of
// GetLibrary: a token that is not a valid start index is rejected with
// InvalidArgument, never paged silently (negative/robustness half of §8.1).
func TestM5LibraryInvalidPageTokenRejected(t *testing.T) {
	stack := newM5Stack(t, fakeJellyfinServer(t))
	client := apiv1.NewCoreServiceClient(startWireServer(t, stack))
	aliceCtx := authedCtx(t.Context(), stack.aliceToken)

	for _, token := range []string{"abc", "4", "-1"} {
		_, err := client.GetLibrary(aliceCtx, &apiv1.GetLibraryRequest{PageToken: token})
		if got := status.Code(err); got != codes.InvalidArgument {
			t.Errorf("page token %q: got %v, want InvalidArgument", token, got)
		}
	}
}

// libraryPageSize mirrors the bounded page bound of the API service; asserted
// here so the browse test's token expectations stay coupled to it. The value's
// home is TECHNICAL-DECISIONS.md at P6.
const libraryPageSize = 100
