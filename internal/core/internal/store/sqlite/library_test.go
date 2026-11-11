package sqlite

import (
	"context"
	"testing"
	"time"
)

func mustCreateSource(t *testing.T, s *Store, verifiedID string) *Source {
	t.Helper()
	ctx := context.Background()
	src, err := s.CreateSource(ctx, Source{
		ConnectorID:        "jellyfin",
		VerifiedExternalID: verifiedID,
		Endpoint:           "https://media.example.test",
		DisplayName:        "Media Server",
		Status:             1, // SOURCE_STATUS_ACTIVE in core's proto-free int32 shape
		CreatedBy:          "usr_admin",
	})
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	return src
}

func TestReplaceSourceSnapshotUpsertsAndRemovesByUnavailable(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	src := mustCreateSource(t, s, "jf-test-1")
	now := time.Now().UTC()

	syncState, err := s.ReplaceSourceSnapshot(ctx, src.SourceID, LibraryOutcomeSuccess, &now, "", []LibraryTitleInput{
		{ExternalID: "jf-1", Name: "Blade Runner", Kind: LibraryTitleKindMovie, Year: 1982, Overview: "Neon noir."},
		{ExternalID: "jf-2", Name: "Severance", Kind: LibraryTitleKindSeries, Year: 2022, Overview: "Work-life separation."},
	})
	if err != nil {
		t.Fatalf("ReplaceSourceSnapshot: %v", err)
	}
	if syncState.TitleCount != 2 {
		t.Fatalf("title_count = %d, want 2", syncState.TitleCount)
	}

	list, err := s.ListTitles(ctx, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Titles) != 2 || list.TotalCount != 2 {
		t.Fatalf("list = %+v, want 2 titles", list)
	}
	if list.Titles[0].Name != "Blade Runner" && list.Titles[1].Name != "Blade Runner" {
		t.Fatalf("titles not sorted by name: %+v", list.Titles)
	}

	// Re-sync the same library with Severance removed -> unavailable, not deleted.
	later := now.Add(time.Hour)
	syncState, err = s.ReplaceSourceSnapshot(ctx, src.SourceID, LibraryOutcomeSuccess, &later, "", []LibraryTitleInput{
		{ExternalID: "jf-1", Name: "Blade Runner", Kind: LibraryTitleKindMovie, Year: 1982, Overview: "Neon noir."},
	})
	if err != nil {
		t.Fatalf("re-sync: %v", err)
	}
	if syncState.TitleCount != 1 {
		t.Fatalf("title_count = %d, want 1", syncState.TitleCount)
	}
	removed, err := s.GetTitleByExternal(ctx, src.SourceID, "jf-2")
	if err != nil {
		t.Fatalf("GetTitleByExternal: %v", err)
	}
	if removed.Availability != LibraryAvailabilityUnavailable {
		t.Fatalf("removed title availability = %d, want unavailable", removed.Availability)
	}
	if removed.AvailabilityDetail == "" {
		t.Fatal("removed title availability_detail must tell the user why")
	}

	// Gone titles stay out of browse/search; by-id resolution is unchanged.
	list, err = s.ListTitles(ctx, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Titles) != 1 || list.TotalCount != 1 || list.Titles[0].Name != "Blade Runner" {
		t.Fatalf("list after re-sync = %+v, want only Blade Runner", list)
	}
	gone, err := s.SearchTitles(ctx, "Severance", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(gone.Titles) != 0 || gone.TotalCount != 0 {
		t.Fatalf("search for gone title = %+v, want no results", gone)
	}
	byID, err := s.GetTitle(ctx, removed.TitleID)
	if err != nil {
		t.Fatalf("GetTitle by id after sync: %v", err)
	}
	if byID.Availability != LibraryAvailabilityUnavailable {
		t.Fatalf("GetTitle by id availability = %d, want unavailable", byID.Availability)
	}
}

func TestReplaceSourceSnapshotPartialAndFailedRetainSnapshot(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	src := mustCreateSource(t, s, "jf-test-2")
	now := time.Now().UTC()

	if _, err := s.ReplaceSourceSnapshot(ctx, src.SourceID, LibraryOutcomeSuccess, &now, "", []LibraryTitleInput{
		{ExternalID: "jf-1", Name: "Blade Runner", Kind: LibraryTitleKindMovie, Year: 1982},
	}); err != nil {
		t.Fatal(err)
	}

	// A partial sync sees a subset; absent titles must NOT be marked unavailable.
	part := now.Add(time.Minute)
	syncState, err := s.ReplaceSourceSnapshot(ctx, src.SourceID, LibraryOutcomePartial, &part, "some pages failed; partial results", nil)
	if err != nil {
		t.Fatalf("partial sync: %v", err)
	}
	if syncState.LastOutcome != LibraryOutcomePartial {
		t.Fatalf("outcome = %d, want partial", syncState.LastOutcome)
	}
	if syncState.Detail == "" {
		t.Fatal("partial sync detail must describe the partial state")
	}
	detail, err := s.GetTitleByExternal(ctx, src.SourceID, "jf-1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Availability != LibraryAvailabilityAvailable {
		t.Fatal("partial sync changed availability of an absent title")
	}

	// A failed sync keeps the snapshot and reports the failure.
	fail := now.Add(2 * time.Minute)
	syncState, err = s.ReplaceSourceSnapshot(ctx, src.SourceID, LibraryOutcomeFailed, &fail, "source was unreachable; prior snapshot retained", nil)
	if err != nil {
		t.Fatalf("failed sync: %v", err)
	}
	if syncState.LastOutcome != LibraryOutcomeFailed {
		t.Fatalf("outcome = %d, want failed", syncState.LastOutcome)
	}
	detail, err = s.GetTitleByExternal(ctx, src.SourceID, "jf-1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Availability != LibraryAvailabilityAvailable {
		t.Fatal("failed sync changed availability")
	}
}

func TestSearchTitles(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	src := mustCreateSource(t, s, "jf-test-3")
	now := time.Now().UTC()

	if _, err := s.ReplaceSourceSnapshot(ctx, src.SourceID, LibraryOutcomeSuccess, &now, "", []LibraryTitleInput{
		{ExternalID: "jf-1", Name: "Blade Runner", Kind: LibraryTitleKindMovie, Year: 1982},
		{ExternalID: "jf-2", Name: "Blade Runner 2049", Kind: LibraryTitleKindMovie, Year: 2017},
		{ExternalID: "jf-3", Name: "The Matrix", Kind: LibraryTitleKindMovie, Year: 1999},
	}); err != nil {
		t.Fatal(err)
	}

	found, err := s.SearchTitles(ctx, "blade runner", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(found.Titles) != 2 || found.TotalCount != 2 {
		t.Fatalf("search found %d titles (total %d), want 2", len(found.Titles), found.TotalCount)
	}

	// LIKE metacharacters are literal, not wildcards.
	found, err = s.SearchTitles(ctx, "%", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if found.TotalCount != 0 {
		t.Fatalf("search for %% matched %d, want 0", found.TotalCount)
	}

	// Pagination
	page, err := s.SearchTitles(ctx, "blade runner", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Titles) != 1 || page.Titles[0].Name != "Blade Runner 2049" {
		t.Fatalf("page 2 = %+v", page.Titles)
	}

	found, err = s.SearchTitles(ctx, "The Matrix", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if found.TotalCount != 1 || found.Titles[0].Name != "The Matrix" {
		t.Fatalf("search Matrix = %+v", found)
	}
}
