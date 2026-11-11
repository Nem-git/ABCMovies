package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	libraryv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/library/v1"
	sourcev1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/source/v1"
	"github.com/Nem-git/ABCMovies/internal/core/internal/audit"
	"github.com/Nem-git/ABCMovies/internal/core/internal/connector/source/jellyfin"
	"github.com/Nem-git/ABCMovies/internal/core/internal/store"
)

func newRemoveTestStack(t *testing.T, v sourceVerifier) (*SourceService, *LibraryService, *store.Store, *SessionState, string) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	auditPath := filepath.Join(dir, "audit.log")
	aud, err := audit.Open(auditPath)
	if err != nil {
		t.Fatalf("audit.Open: %v", err)
	}
	t.Cleanup(func() { _ = aud.Close() })
	sv := NewSessionState()
	svc := NewSourceServiceWithVerifier(st, v, sv, aud)
	lib := NewLibraryServiceWithFetcher(st, fakeFetcher{snapshot: &jellyfin.LibrarySnapshot{
		Items: []jellyfin.LibraryItem{{ExternalID: "mv-1", Name: "Killable Movie", Kind: "movie"}},
	}}, sv)
	lib.authorizer = &fakeAuthorizer{header: testAuthHeader}
	return svc, lib, st, sv, auditPath
}

func connectAndSync(t *testing.T, svc *SourceService, lib *LibraryService, endpoint string) string {
	t.Helper()
	ctx := context.Background()
	conn, err := svc.ConnectSource(ctx, &sourcev1.ConnectSourceRequest{
		ActorUserId: "usr_admin",
		ConnectorId: "jellyfin",
		Endpoint:    endpoint,
		Credentials: &sourcev1.ConnectSourceRequest_Password{
			Password: &sourcev1.PasswordCredentials{Username: "admin", Password: "sentinel-remove-pass"},
		},
	})
	if err != nil {
		t.Fatalf("ConnectSource: %v", err)
	}
	sourceID := conn.GetSource().GetSourceId()
	if _, err := lib.SyncSource(ctx, &libraryv1.SyncSourceRequest{
		ActorUserId: "usr_admin",
		SourceId:    sourceID,
		Credentials: &libraryv1.SyncSourceRequest_Password{
			Password: &libraryv1.PasswordCredentials{Username: "admin", Password: "sentinel-remove-pass"},
		},
	}); err != nil {
		t.Fatalf("SyncSource: %v", err)
	}
	return sourceID
}

func TestRemoveSourceRevokesTicketsAndFlushesCreds(t *testing.T) {
	svc, lib, st, sv, auditPath := newRemoveTestStack(t, &fakeVerifier{})
	ctx := context.Background()
	sourceID := connectAndSync(t, svc, lib, "https://media.example.test")

	// One title, playable, with a ticket and cached credentials.
	list, err := lib.ListTitles(ctx, &libraryv1.ListTitlesRequest{ActorUserId: "usr_admin"})
	if err != nil {
		t.Fatalf("ListTitles: %v", err)
	}
	if len(list.GetTitles()) != 1 {
		t.Fatalf("titles = %d, want 1", len(list.GetTitles()))
	}
	titleID := list.GetTitles()[0].GetTitleId()
	play, err := lib.CreatePlayTicket(ctx, &libraryv1.CreatePlayTicketRequest{ActorUserId: "usr_admin", TitleId: titleID})
	if err != nil {
		t.Fatalf("CreatePlayTicket: %v", err)
	}
	if _, err := lib.ResolvePlayTicket(ctx, &libraryv1.ResolvePlayTicketRequest{ActorUserId: "usr_admin", Ticket: play.GetTicket()}); err != nil {
		t.Fatalf("ticket should resolve before remove: %v", err)
	}

	if _, err := svc.RemoveSource(ctx, &sourcev1.RemoveSourceRequest{ActorUserId: "usr_admin", SourceId: sourceID}); err != nil {
		t.Fatalf("RemoveSource: %v", err)
	}

	// Ticket revoked.
	if _, err := lib.ResolvePlayTicket(ctx, &libraryv1.ResolvePlayTicketRequest{ActorUserId: "usr_admin", Ticket: play.GetTicket()}); status.Code(err) != codes.NotFound {
		t.Fatalf("resolve after remove code = %v, want NotFound", status.Code(err))
	}
	// Cached credentials evicted.
	if _, ok := sv.creds.Get("usr_admin"); ok {
		t.Fatal("cached credentials survived remove")
	}

	// Source tombstoned and hidden from the list.
	got, err := svc.GetSource(ctx, &sourcev1.GetSourceRequest{ActorUserId: "usr_admin", SourceId: sourceID})
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}
	if got.GetSource().GetStatus() != sourcev1.SourceStatus_SOURCE_STATUS_DISCONNECTED {
		t.Fatalf("status = %v, want DISCONNECTED", got.GetSource().GetStatus())
	}
	listed, err := svc.ListSources(ctx, &sourcev1.ListSourcesRequest{ActorUserId: "usr_admin"})
	if err != nil {
		t.Fatalf("ListSources: %v", err)
	}
	if len(listed.GetSources()) != 0 {
		t.Fatalf("ListSources after remove = %+v, want empty", listed.GetSources())
	}

	// Titles are marked unavailable, not deleted.
	check, err := st.GetTitle(ctx, titleID)
	if err != nil {
		t.Fatalf("GetTitle: %v", err)
	}
	if check.Availability != store.LibraryAvailabilityUnavailable {
		t.Fatalf("title availability = %d, want unavailable", check.Availability)
	}

	// Audit recorded the removal with no credentials in it.
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	entry := string(data)
	if !strings.Contains(entry, "source_removed") || !strings.Contains(entry, sourceID) {
		t.Fatalf("audit log missing source_removed entry for %q: %s", sourceID, entry)
	}
	if strings.Contains(entry, "sentinel-remove-pass") {
		t.Fatal("audit log leaked the password")
	}
}

func TestRemoveSourceUnknownAndBadInput(t *testing.T) {
	svc, _, _, _, _ := newRemoveTestStack(t, &fakeVerifier{})
	ctx := context.Background()

	if _, err := svc.RemoveSource(ctx, &sourcev1.RemoveSourceRequest{SourceId: "src_x"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("empty actor code = %v, want InvalidArgument", status.Code(err))
	}
	if _, err := svc.RemoveSource(ctx, &sourcev1.RemoveSourceRequest{ActorUserId: "usr_admin"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("empty source id code = %v, want InvalidArgument", status.Code(err))
	}
	if _, err := svc.RemoveSource(ctx, &sourcev1.RemoveSourceRequest{ActorUserId: "usr_admin", SourceId: "src_missing"}); status.Code(err) != codes.NotFound {
		t.Fatalf("unknown source code = %v, want NotFound", status.Code(err))
	}
}

func TestConnectRevivesDisconnectedSource(t *testing.T) {
	svc, lib, st, _, _ := newRemoveTestStack(t, &fakeVerifier{})
	ctx := context.Background()
	sourceID := connectAndSync(t, svc, lib, "https://revive.example.test")

	if _, err := svc.RemoveSource(ctx, &sourcev1.RemoveSourceRequest{ActorUserId: "usr_admin", SourceId: sourceID}); err != nil {
		t.Fatalf("RemoveSource: %v", err)
	}

	// Re-connecting the same verified server returns CONNECTED (not
	// DUPLICATE) with the same source_id.
	conn, err := svc.ConnectSource(ctx, &sourcev1.ConnectSourceRequest{
		ActorUserId: "usr_admin",
		ConnectorId: "jellyfin",
		Endpoint:    "https://revive.example.test",
		Credentials: &sourcev1.ConnectSourceRequest_Password{
			Password: &sourcev1.PasswordCredentials{Username: "admin", Password: "sentinel-remove-pass"},
		},
	})
	if err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	if conn.GetOutcome() != sourcev1.ConnectSourceOutcome_CONNECT_SOURCE_OUTCOME_CONNECTED {
		t.Fatalf("reconnect outcome = %v, want CONNECTED", conn.GetOutcome())
	}
	if conn.GetSource().GetSourceId() != sourceID {
		t.Fatalf("reconnect source_id = %q, want the tombstoned %q", conn.GetSource().GetSourceId(), sourceID)
	}
	if conn.GetSource().GetStatus() != sourcev1.SourceStatus_SOURCE_STATUS_ACTIVE {
		t.Fatalf("reconnect status = %v, want ACTIVE", conn.GetSource().GetStatus())
	}

	// Until the next sync, titles stay unavailable (tombstone semantics)...
	list, err := lib.ListTitles(ctx, &libraryv1.ListTitlesRequest{ActorUserId: "usr_admin"})
	if err != nil {
		t.Fatalf("ListTitles: %v", err)
	}
	if list.GetTitles()[0].GetAvailability() != libraryv1.AvailabilityState_AVAILABILITY_STATE_UNAVAILABLE {
		t.Fatalf("availability before sync = %v, want unavailable", list.GetTitles()[0].GetAvailability())
	}

	// ...and sync restores them (the tombstone never needed re-creating).
	if _, err := lib.SyncSource(ctx, &libraryv1.SyncSourceRequest{
		ActorUserId: "usr_admin",
		SourceId:    sourceID,
		Credentials: &libraryv1.SyncSourceRequest_Password{
			Password: &libraryv1.PasswordCredentials{Username: "admin", Password: "sentinel-remove-pass"},
		},
	}); err != nil {
		t.Fatalf("re-sync: %v", err)
	}
	list, err = lib.ListTitles(ctx, &libraryv1.ListTitlesRequest{ActorUserId: "usr_admin"})
	if err != nil {
		t.Fatalf("ListTitles after sync: %v", err)
	}
	if list.GetTitles()[0].GetAvailability() != libraryv1.AvailabilityState_AVAILABILITY_STATE_AVAILABLE {
		t.Fatalf("availability after re-sync = %v, want available", list.GetTitles()[0].GetAvailability())
	}

	_ = st // store kept for symmetry with sibling tests
}
