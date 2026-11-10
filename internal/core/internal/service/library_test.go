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
