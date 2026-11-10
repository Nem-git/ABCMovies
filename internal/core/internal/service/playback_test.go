package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	libraryv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/library/v1"
	"github.com/Nem-git/ABCMovies/internal/core/internal/connector/source/jellyfin"
)

const testAuthHeader = `MediaBrowser Client="abcmovies", Device="abcmovies-server", Token="tok-test"`

// fakeAuthorizer authenticates like a server would: any password containing
// "wrong" is rejected; everything else succeeds. It counts calls.
type fakeAuthorizer struct {
	header string
	calls  int
}

func (f *fakeAuthorizer) Authorize(_ context.Context, _ string, creds jellyfin.Credentials) (string, *jellyfin.Error) {
	f.calls++
	if creds.Password != nil && strings.Contains(creds.Password.Password, "wrong") {
		return "", &jellyfin.Error{Kind: jellyfin.ErrKindUnauthorized, Message: "invalid username or password"}
	}
	return f.header, nil
}

func TestCreatePlayTicketUsesSyncPrimeCache(t *testing.T) {
	svc, st := newTestLibraryService(t)
	a := &fakeAuthorizer{header: testAuthHeader}
	svc.authorizer = a
	src := mustCreateSource(t, st, "jf-play-1")

	svc.fetcher = fakeFetcher{snapshot: &jellyfin.LibrarySnapshot{
		Items: []jellyfin.LibraryItem{{ExternalID: "jf-1", Name: "Playable", Kind: "movie", Year: 2000}},
	}}
	ctx := context.Background()
	if _, err := svc.SyncSource(ctx, &libraryv1.SyncSourceRequest{
		ActorUserId: "usr_admin",
		SourceId:    src.SourceID,
		Credentials: &libraryv1.SyncSourceRequest_Password{Password: &libraryv1.PasswordCredentials{Username: "admin", Password: "correct horse"}},
	}); err != nil {
		t.Fatalf("SyncSource: %v", err)
	}

	list, err := svc.ListTitles(ctx, &libraryv1.ListTitlesRequest{ActorUserId: "usr_admin"})
	if err != nil {
		t.Fatalf("ListTitles: %v", err)
	}
	if len(list.GetTitles()) != 1 {
		t.Fatalf("expected 1 title, got %d", len(list.GetTitles()))
	}
	titleID := list.GetTitles()[0].GetTitleId()

	resp, err := svc.CreatePlayTicket(ctx, &libraryv1.CreatePlayTicketRequest{ActorUserId: "usr_admin", TitleId: titleID})
	if err != nil {
		t.Fatalf("CreatePlayTicket with cached creds: %v", err)
	}
	if resp.GetTicket() == "" {
		t.Fatal("empty ticket")
	}
	if a.calls != 1 {
		t.Fatalf("authorizer calls = %d, want 1 (sync primes, then play authorizes)", a.calls)
	}

	resolved, err := svc.ResolvePlayTicket(ctx, &libraryv1.ResolvePlayTicketRequest{ActorUserId: "usr_admin", Ticket: resp.GetTicket()})
	if err != nil {
		t.Fatalf("ResolvePlayTicket: %v", err)
	}
	if resolved.GetTitleId() != titleID {
		t.Fatalf("resolved title = %q, want %q", resolved.GetTitleId(), titleID)
	}
	if !strings.Contains(resolved.GetAuthorization(), "Token=") {
		t.Fatalf("authorization header missing token: %q", resolved.GetAuthorization())
	}
	if !strings.Contains(resolved.GetStreamUrl(), "/Videos/jf-1/stream") {
		t.Fatalf("stream url = %q", resolved.GetStreamUrl())
	}
}

func TestCreatePlayTicketColdCacheFails(t *testing.T) {
	svc, st := newTestLibraryService(t)
	a := &fakeAuthorizer{header: testAuthHeader}
	svc.authorizer = a
	src := mustCreateSource(t, st, "jf-play-2")

	svc.fetcher = fakeFetcher{snapshot: &jellyfin.LibrarySnapshot{
		Items: []jellyfin.LibraryItem{{ExternalID: "jf-9", Name: "Cold", Kind: "movie"}},
	}}
	ctx := context.Background()
	if _, err := svc.SyncSource(ctx, &libraryv1.SyncSourceRequest{
		ActorUserId: "usr_admin",
		SourceId:    src.SourceID,
		Credentials: &libraryv1.SyncSourceRequest_Password{Password: &libraryv1.PasswordCredentials{Username: "admin", Password: "correct horse"}},
	}); err != nil {
		t.Fatalf("SyncSource: %v", err)
	}
	// Simulate a fresh core process: the store persists, the credential cache does not.
	svc.session.creds = newSessionCredStore()

	list, err := svc.ListTitles(ctx, &libraryv1.ListTitlesRequest{ActorUserId: "usr_admin"})
	if err != nil {
		t.Fatalf("ListTitles: %v", err)
	}
	titleID := list.GetTitles()[0].GetTitleId()

	_, err = svc.CreatePlayTicket(ctx, &libraryv1.CreatePlayTicketRequest{ActorUserId: "usr_admin", TitleId: titleID})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("cold-cache CreatePlayTicket code = %v, want FailedPrecondition; err=%v", status.Code(err), err)
	}
	if a.calls != 0 {
		t.Fatalf("authorizer called %d times on cold cache, want 0", a.calls)
	}
}

func TestCreatePlayTicketRejectsSeriesAndUnavailable(t *testing.T) {
	svc, st := newTestLibraryService(t)
	a := &fakeAuthorizer{header: testAuthHeader}
	svc.authorizer = a
	src := mustCreateSource(t, st, "jf-play-3")

	svc.fetcher = fakeFetcher{snapshot: &jellyfin.LibrarySnapshot{Items: []jellyfin.LibraryItem{
		{ExternalID: "jf-movie", Name: "Movie", Kind: "movie"},
		{ExternalID: "jf-series", Name: "Series", Kind: "series"},
	}}}
	ctx := context.Background()
	sync := func() {
		if _, err := svc.SyncSource(ctx, &libraryv1.SyncSourceRequest{
			ActorUserId: "usr_admin",
			SourceId:    src.SourceID,
			Credentials: &libraryv1.SyncSourceRequest_Password{Password: &libraryv1.PasswordCredentials{Username: "admin", Password: "correct horse"}},
		}); err != nil {
			t.Fatalf("SyncSource: %v", err)
		}
	}
	sync()

	list, err := svc.ListTitles(ctx, &libraryv1.ListTitlesRequest{ActorUserId: "usr_admin"})
	if err != nil {
		t.Fatalf("ListTitles: %v", err)
	}
	var seriesID, movieID string
	for _, ti := range list.GetTitles() {
		switch ti.GetKind() {
		case libraryv1.TitleKind_TITLE_KIND_SERIES:
			seriesID = ti.GetTitleId()
		case libraryv1.TitleKind_TITLE_KIND_MOVIE:
			movieID = ti.GetTitleId()
		}
	}

	_, err = svc.CreatePlayTicket(ctx, &libraryv1.CreatePlayTicketRequest{ActorUserId: "usr_admin", TitleId: seriesID})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("series play code = %v, want FailedPrecondition", status.Code(err))
	}

	// Remove everything upstream; the next sync marks titles unavailable.
	svc.fetcher = fakeFetcher{snapshot: &jellyfin.LibrarySnapshot{}}
	sync()
	_, err = svc.CreatePlayTicket(ctx, &libraryv1.CreatePlayTicketRequest{ActorUserId: "usr_admin", TitleId: movieID})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("unavailable play code = %v, want FailedPrecondition", status.Code(err))
	}
}

func TestResolvePlayTicketSlidesExpiry(t *testing.T) {
	svc, _ := newTestLibraryService(t)
	base := time.Now()
	fake := base
	svc.session.tickets.now = func() time.Time { return fake }

	p := playTicket{actorUserID: "usr_admin", titleID: "ttl_1", sourceID: "src_1", streamURL: "u", authorization: "a", expiresAt: fake.Add(ticketIdleTTL)}
	id, err := svc.session.tickets.mint(p)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	resolve := func() error {
		_, err := svc.ResolvePlayTicket(context.Background(), &libraryv1.ResolvePlayTicketRequest{ActorUserId: "usr_admin", Ticket: id})
		return err
	}

	// 25 minutes in with an active fetch: resolve refreshes the idle lifetime.
	fake = fake.Add(25 * time.Minute)
	if err := resolve(); err != nil {
		t.Fatalf("resolve at +25m: %v", err)
	}
	if got := svc.session.tickets.m[id].expiresAt; !got.Equal(fake.Add(ticketIdleTTL)) {
		t.Fatalf("expiry after resolve = %v, want %v (sliding)", got, fake.Add(ticketIdleTTL))
	}

	// +54m: still valid (would have died at +30m without sliding).
	fake = fake.Add(29 * time.Minute)
	if err := resolve(); err != nil {
		t.Fatalf("resolve at +54m: %v (sliding refresh lost)", err)
	}

	// 31 minutes without a fetch: the ticket expires.
	fake = fake.Add(31 * time.Minute)
	if err := resolve(); status.Code(err) != codes.NotFound {
		t.Fatalf("resolve after idle %v, want NotFound", err)
	}
}

func TestResolvePlayTicketRejectsForeignActorAndExpired(t *testing.T) {
	svc, _ := newTestLibraryService(t)
	p := playTicket{
		actorUserID:   "usr_admin",
		titleID:       "ttl_1",
		sourceID:      "src_1",
		streamURL:     "https://media.example.test/Videos/jf-1/stream?static=true",
		authorization: testAuthHeader,
		expiresAt:     time.Now().Add(time.Minute),
	}
	id, err := svc.session.tickets.mint(p)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	// Different actor: uniform NotFound.
	_, err = svc.ResolvePlayTicket(context.Background(), &libraryv1.ResolvePlayTicketRequest{ActorUserId: "usr_other", Ticket: id})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("foreign-actor resolve code = %v, want NotFound", status.Code(err))
	}

	// Expired ticket: NotFound.
	expiredID, err := svc.session.tickets.mint(playTicket{actorUserID: "usr_admin", titleID: "ttl_1", sourceID: "src_1", streamURL: "u", authorization: "a", expiresAt: time.Now().Add(-time.Minute)})
	if err != nil {
		t.Fatalf("mint expired: %v", err)
	}
	_, err = svc.ResolvePlayTicket(context.Background(), &libraryv1.ResolvePlayTicketRequest{ActorUserId: "usr_admin", Ticket: expiredID})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expired resolve code = %v, want NotFound", status.Code(err))
	}

	// Unknown ticket: NotFound.
	_, err = svc.ResolvePlayTicket(context.Background(), &libraryv1.ResolvePlayTicketRequest{ActorUserId: "usr_admin", Ticket: "doesnotexist"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("unknown resolve code = %v, want NotFound", status.Code(err))
	}
}

func TestCreatePlayTicketAuthFailureMapsAndDoesNotCache(t *testing.T) {
	svc, st := newTestLibraryService(t)
	a := &fakeAuthorizer{header: testAuthHeader}
	svc.authorizer = a
	src := mustCreateSource(t, st, "jf-play-4")

	svc.fetcher = fakeFetcher{snapshot: &jellyfin.LibrarySnapshot{
		Items: []jellyfin.LibraryItem{{ExternalID: "jf-7", Name: "Movie", Kind: "movie"}},
	}}
	ctx := context.Background()
	if _, err := svc.SyncSource(ctx, &libraryv1.SyncSourceRequest{
		ActorUserId: "usr_admin",
		SourceId:    src.SourceID,
		Credentials: &libraryv1.SyncSourceRequest_Password{Password: &libraryv1.PasswordCredentials{Username: "admin", Password: "correct horse"}},
	}); err != nil {
		t.Fatalf("SyncSource: %v", err)
	}
	// Sync primes the cache; now supply deliberately wrong credentials.
	_, err := svc.CreatePlayTicket(ctx, &libraryv1.CreatePlayTicketRequest{
		ActorUserId: "usr_admin",
		TitleId:     mustOnlyTitleID(t, svc),
		Credentials: &libraryv1.CreatePlayTicketRequest_Password{Password: &libraryv1.PasswordCredentials{Username: "admin", Password: "wrong password"}},
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("bad-creds CreatePlayTicket code = %v, want Unauthenticated", status.Code(err))
	}
	// The failed attempt must not replace the cached credentials: a blank
	// request still works.
	_, err = svc.CreatePlayTicket(ctx, &libraryv1.CreatePlayTicketRequest{ActorUserId: "usr_admin", TitleId: mustOnlyTitleID(t, svc)})
	if err != nil {
		t.Fatalf("blank play after failed attempt: %v", err)
	}
}

func mustOnlyTitleID(t *testing.T, svc *LibraryService) string {
	t.Helper()
	list, err := svc.ListTitles(context.Background(), &libraryv1.ListTitlesRequest{ActorUserId: "usr_admin"})
	if err != nil {
		t.Fatalf("ListTitles: %v", err)
	}
	if len(list.GetTitles()) != 1 {
		t.Fatalf("expected 1 title, got %d", len(list.GetTitles()))
	}
	return list.GetTitles()[0].GetTitleId()
}

func TestSyncSourceRequiresCredsOrCache(t *testing.T) {
	svc, st := newTestLibraryService(t)
	src := mustCreateSource(t, st, "jf-play-5")
	ctx := context.Background()

	// No credentials and nothing cached: FailedPrecondition, no fetch.
	_, err := svc.SyncSource(ctx, &libraryv1.SyncSourceRequest{ActorUserId: "usr_admin", SourceId: src.SourceID})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("sync without creds/cache code = %v, want FailedPrecondition", status.Code(err))
	}
	if !strings.Contains(err.Error(), "credentials are required") {
		t.Fatalf("sync error = %v, want credentials-required message", err)
	}
}
