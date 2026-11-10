package service

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	libraryv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/library/v1"
	"github.com/Nem-git/ABCMovies/internal/core/internal/connector/source/jellyfin"
	"github.com/Nem-git/ABCMovies/internal/core/internal/store"
)

// fakeFetcher implements libraryFetcher for deterministic tests without a live server.
type fakeFetcher struct {
	snapshot *jellyfin.LibrarySnapshot
	err      *jellyfin.Error
}

func (f fakeFetcher) FetchLibrary(_ context.Context, _ string, _ jellyfin.Credentials) (*jellyfin.LibrarySnapshot, *jellyfin.Error) {
	return f.snapshot, f.err
}

func newTestLibraryService(t *testing.T) (*LibraryService, *store.Store) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return NewLibraryServiceWithFetcher(st, fakeFetcher{}, NewSessionState()), st
}

func mustCreateSource(t *testing.T, st *store.Store, verifiedID string) *store.Source {
	t.Helper()
	src, err := st.CreateSource(context.Background(), store.Source{
		ConnectorID:        "jellyfin",
		VerifiedExternalID: verifiedID,
		Endpoint:           "https://media.example.test",
		DisplayName:        "Media Server",
		Status:             1,
		CreatedBy:          "usr_admin",
	})
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	return src
}

func mustConnectForSync(t *testing.T, st *store.Store) *libraryv1.SyncSourceRequest {
	t.Helper()
	src := mustCreateSource(t, st, "jf-test-library")
	return &libraryv1.SyncSourceRequest{
		ActorUserId: "usr_admin",
		SourceId:    src.SourceID,
		Credentials: &libraryv1.SyncSourceRequest_Password{Password: &libraryv1.PasswordCredentials{Username: "admin", Password: "correct horse"}},
	}
}

func TestSyncSourceHappyPath(t *testing.T) {
	svc, st := newTestLibraryService(t)
	req := mustConnectForSync(t, st)
	svc.fetcher = fakeFetcher{
		snapshot: &jellyfin.LibrarySnapshot{
			Items: []jellyfin.LibraryItem{
				{ExternalID: "jf-1", Name: "Blade Runner", Kind: "movie", Year: 1982, Overview: "Neon noir."},
				{ExternalID: "jf-2", Name: "Severance", Kind: "series", Year: 2022},
			},
		},
	}
	ctx := context.Background()

	resp, err := svc.SyncSource(ctx, req)
	if err != nil {
		t.Fatalf("SyncSource: %v", err)
	}
	if resp.GetOutcome() != libraryv1.SyncOutcome_SYNC_OUTCOME_SUCCESS {
		t.Fatalf("outcome = %v, want success", resp.GetOutcome())
	}
	state := resp.GetState()
	if state.GetTitleCount() != 2 {
		t.Fatalf("title_count = %d, want 2", state.GetTitleCount())
	}
	if state.GetLastOutcome() != libraryv1.SyncOutcome_SYNC_OUTCOME_SUCCESS {
		t.Fatalf("state outcome = %v", state.GetLastOutcome())
	}
	if state.GetLastSyncedAt() == nil {
		t.Fatal("last_synced_at is unset after sync")
	}

	// Titles are listed with sync state disclosure.
	list, err := svc.ListTitles(ctx, &libraryv1.ListTitlesRequest{ActorUserId: "usr_admin", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.GetTitles()) != 2 || list.GetTotalCount() != 2 {
		t.Fatalf("list = %+v", list)
	}
	if list.GetSyncStates() == nil || len(list.GetSyncStates()) == 0 {
		t.Fatal("sync_states missing from ListTitles response")
	}
	if !strings.Contains(list.GetSyncStates()[0].SourceId, req.GetSourceId()) {
		t.Fatalf("sync_state source_id = %v", list.GetSyncStates()[0].GetSourceId())
	}
	// Availability present.
	for _, title := range list.GetTitles() {
		if title.GetAvailability() != libraryv1.AvailabilityState_AVAILABILITY_STATE_AVAILABLE {
			t.Fatalf("title %q availability = %v", title.GetTitleId(), title.GetAvailability())
		}
	}
}

func TestSyncSourcePartialSkippedItems(t *testing.T) {
	svc, st := newTestLibraryService(t)
	req := mustConnectForSync(t, st)
	svc.fetcher = fakeFetcher{
		snapshot: &jellyfin.LibrarySnapshot{
			Items:   []jellyfin.LibraryItem{{ExternalID: "jf-1", Name: "Blade Runner", Kind: "movie", Year: 1982}},
			Skipped: 2,
		},
	}

	resp, err := svc.SyncSource(context.Background(), req)
	if err != nil {
		t.Fatalf("SyncSource: %v", err)
	}
	if resp.GetOutcome() != libraryv1.SyncOutcome_SYNC_OUTCOME_PARTIAL {
		t.Fatalf("outcome = %v, want partial", resp.GetOutcome())
	}
	if !strings.Contains(resp.GetState().GetDetail(), "skipped 2 titles") {
		t.Fatalf("detail = %q", resp.GetState().GetDetail())
	}
	// Snapshot applied anyway.
	got, err := st.ListTitles(context.Background(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Titles) != 1 {
		t.Fatalf("titles after partial = %d", len(got.Titles))
	}
}

func TestSyncSourceFetchFailureRecordsFailedState(t *testing.T) {
	svc, st := newTestLibraryService(t)
	req := mustConnectForSync(t, st)
	svc.fetcher = fakeFetcher{err: &jellyfin.Error{Kind: jellyfin.ErrKindUnreachable, Message: "the server could not be reached", Retryable: true}}

	_, err := svc.SyncSource(context.Background(), req)
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("code = %v, want Unavailable", status.Code(err))
	}
	// Failed state is recorded; no titles are applied.
	list, err := svc.ListTitles(context.Background(), &libraryv1.ListTitlesRequest{ActorUserId: "usr_admin", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.GetSyncStates()) != 1 || list.GetSyncStates()[0].GetLastOutcome() != libraryv1.SyncOutcome_SYNC_OUTCOME_FAILED {
		t.Fatalf("sync_states = %+v", list.GetSyncStates())
	}
	if len(list.GetTitles()) != 0 {
		t.Fatalf("titles after failed sync = %d", len(list.GetTitles()))
	}
	if !strings.Contains(list.GetSyncStates()[0].GetDetail(), "could not be reached") {
		t.Fatalf("detail = %q", list.GetSyncStates()[0].GetDetail())
	}
}

func TestSyncSourceBadCredentials(t *testing.T) {
	svc, st := newTestLibraryService(t)
	src := mustCreateSource(t, st, "jf-test-library-2")
	svc.fetcher = fakeFetcher{err: &jellyfin.Error{Kind: jellyfin.ErrKindUnauthorized, Message: "invalid username or password"}}

	req := &libraryv1.SyncSourceRequest{
		ActorUserId: "usr_admin",
		SourceId:    src.SourceID,
		Credentials: &libraryv1.SyncSourceRequest_Password{Password: &libraryv1.PasswordCredentials{Username: "admin", Password: "wrong"}},
	}
	_, err := svc.SyncSource(context.Background(), req)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("code = %v, want Unauthenticated", status.Code(err))
	}
	// No secrets in the error message.
	if strings.Contains(err.Error(), "wrong") {
		t.Fatalf("password leaked into error: %v", err)
	}
}

func TestSearchAndGetTitle(t *testing.T) {
	svc, st := newTestLibraryService(t)
	req := mustConnectForSync(t, st)
	svc.fetcher = fakeFetcher{
		snapshot: &jellyfin.LibrarySnapshot{
			Items: []jellyfin.LibraryItem{{ExternalID: "jf-1", Name: "Blade Runner", Kind: "movie", Year: 1982, Overview: "Neon noir."}},
		},
	}
	if _, err := svc.SyncSource(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	found, err := svc.SearchTitles(context.Background(), &libraryv1.SearchTitlesRequest{ActorUserId: "usr_admin", Query: "blade runner", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(found.GetTitles()) != 1 || found.GetTotalCount() != 1 {
		t.Fatalf("search = %+v", found)
	}
	got := found.GetTitles()[0]
	detail, err := svc.GetTitle(context.Background(), &libraryv1.GetTitleRequest{ActorUserId: "usr_admin", TitleId: got.GetTitleId()})
	if err != nil {
		t.Fatal(err)
	}
	if detail.GetTitle().GetOverview() != "Neon noir." || detail.GetTitle().GetExternalId() != "jf-1" {
		t.Fatalf("detail = %+v", detail.GetTitle())
	}

	// Unknown title id is NotFound.
	_, err = svc.GetTitle(context.Background(), &libraryv1.GetTitleRequest{ActorUserId: "usr_admin", TitleId: "ttl_nope"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %v, want NotFound", status.Code(err))
	}
}

func TestSyncSourceEmptyActor(t *testing.T) {
	svc, _ := newTestLibraryService(t)
	_, err := svc.SyncSource(context.Background(), &libraryv1.SyncSourceRequest{ActorUserId: "  ", SourceId: "src_x"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
	}
}

func TestLibraryHidesDisconnectedSourceTitles(t *testing.T) {
	svc, st := newTestLibraryService(t)
	ctx := context.Background()
	a := mustCreateSource(t, st, "jf-lib-a")
	b := mustCreateSource(t, st, "jf-lib-b")

	// Distinct snapshots per source (fakeFetcher is a single snapshot).
	svc.fetcher = fakeFetcher{snapshot: &jellyfin.LibrarySnapshot{
		Items: []jellyfin.LibraryItem{{ExternalID: "a-1", Name: "A Movie", Kind: "movie"}},
	}}
	if _, err := svc.SyncSource(ctx, &libraryv1.SyncSourceRequest{
		ActorUserId: "usr_admin",
		SourceId:    a.SourceID,
		Credentials: &libraryv1.SyncSourceRequest_Password{
			Password: &libraryv1.PasswordCredentials{Username: "admin", Password: "correct horse"},
		},
	}); err != nil {
		t.Fatalf("SyncSource a: %v", err)
	}
	svc.fetcher = fakeFetcher{snapshot: &jellyfin.LibrarySnapshot{
		Items: []jellyfin.LibraryItem{{ExternalID: "b-1", Name: "B Movie", Kind: "movie"}},
	}}
	if _, err := svc.SyncSource(ctx, &libraryv1.SyncSourceRequest{
		ActorUserId: "usr_admin",
		SourceId:    b.SourceID,
		Credentials: &libraryv1.SyncSourceRequest_Password{
			Password: &libraryv1.PasswordCredentials{Username: "admin", Password: "correct horse"},
		},
	}); err != nil {
		t.Fatalf("SyncSource b: %v", err)
	}

	// Tombstone A at the store layer (the RPC does this; here we exercise
	// the library's respect for it).
	if _, err := st.RemoveSource(ctx, a.SourceID); err != nil {
		t.Fatalf("RemoveSource: %v", err)
	}

	// A's titles are hidden from browse, B's are not; total count follows.
	list, err := svc.ListTitles(ctx, &libraryv1.ListTitlesRequest{ActorUserId: "usr_admin"})
	if err != nil {
		t.Fatalf("ListTitles: %v", err)
	}
	if list.GetTotalCount() != 1 || len(list.GetTitles()) != 1 || list.GetTitles()[0].GetName() != "B Movie" {
		t.Fatalf("list after disconnect = (count %d, titles %+v), want only B Movie", list.GetTotalCount(), list.GetTitles())
	}

	// A's titles are hidden from search, too.
	search, err := svc.SearchTitles(ctx, &libraryv1.SearchTitlesRequest{ActorUserId: "usr_admin", Query: "A Movie"})
	if err != nil {
		t.Fatalf("SearchTitles: %v", err)
	}
	if search.GetTotalCount() != 0 || len(search.GetTitles()) != 0 {
		t.Fatalf("search after disconnect = (count %d, titles %+v), want 0", search.GetTotalCount(), search.GetTitles())
	}

	// A disconnected source's freshness state is hidden as well.
	for _, s := range list.GetSyncStates() {
		if s.GetSourceId() == a.SourceID {
			t.Fatalf("sync states after disconnect include the tombstone: %+v", s)
		}
	}

	// Detail pages still resolve for a tombstoned title (unavailable).
	titleA := findTitleByExternalID(t, st, a.SourceID, "a-1")
	got, err := svc.GetTitle(ctx, &libraryv1.GetTitleRequest{ActorUserId: "usr_admin", TitleId: titleA.TitleID})
	if err != nil {
		t.Fatalf("GetTitle after disconnect: %v", err)
	}
	if got.GetTitle().GetAvailability() != libraryv1.AvailabilityState_AVAILABILITY_STATE_UNAVAILABLE {
		t.Fatalf("availability = %v, want AVAILABILITY_STATE_UNAVAILABLE", got.GetTitle().GetAvailability())
	}
	if !strings.Contains(got.GetTitle().GetAvailabilityDetail(), "source was disconnected") {
		t.Fatalf("availability_detail = %q, want the tombstone detail", got.GetTitle().GetAvailabilityDetail())
	}

	// Sync on a disconnected source fails honestly (422-class).
	if _, err := svc.SyncSource(ctx, &libraryv1.SyncSourceRequest{
		ActorUserId: "usr_admin",
		SourceId:    a.SourceID,
		Credentials: &libraryv1.SyncSourceRequest_Password{
			Password: &libraryv1.PasswordCredentials{Username: "admin", Password: "correct horse"},
		},
	}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("sync on disconnected source code = %v, want FailedPrecondition", status.Code(err))
	}
}

func findTitleByExternalID(t *testing.T, st *store.Store, sourceID, externalID string) *store.LibraryTitle {
	t.Helper()
	ttl, err := st.GetTitleByExternal(context.Background(), sourceID, externalID)
	if err != nil {
		t.Fatalf("GetTitleByExternal %s/%s: %v", sourceID, externalID, err)
	}
	return ttl
}
