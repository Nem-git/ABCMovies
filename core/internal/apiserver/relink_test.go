package apiserver_test

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/config"
)

// linkBob builds the natural key every re-link fixture shares: the caller's
// own jellyfin/bob@https://jf.example link.
func linkBob(password string) *apiv1.LinkAccountRequest {
	return &apiv1.LinkAccountRequest{
		Provider:   "jellyfin",
		BaseUrl:    "https://jf.example/",
		AuthMethod: &apiv1.LinkAccountRequest_Password{Password: &apiv1.AccountPassword{Username: "bob", Password: []byte(password)}},
	}
}

// newRelinkServer wires a server with one accepting prober and, when withAlice
// is set, a user directory that knows "alice" so a shared roster validates.
func newRelinkServer(t *testing.T, withAlice bool) (*apiserver.Server, *stubProber, config.Stores, auth.Session) {
	t.Helper()
	bus := apiserver.NewInMemoryBus()
	t.Cleanup(bus.Close)
	authenticator, session := testAuth(t)
	stores := testStores(t)
	probe := &stubProber{accept: true, blob: []byte(`{"AccessToken":"first"}`)}
	srv := apiserver.NewServer(apiserver.Settings{Bus: bus, Stores: stores, Authenticator: authenticator, Session: session})
	srv.SetProber("jellyfin", probe)
	if withAlice {
		users := auth.NewMemoryUserStore()
		if err := users.PutUser("alice", &auth.UserData{Salt: []byte{1}}); err != nil {
			t.Fatalf("PutUser: %v", err)
		}
		srv.SetUserDirectory(apiserver.NewUserDirectory(users))
	}
	return srv, probe, stores, session
}

// A re-link on the caller's natural key refreshes the session in place: the
// same account id comes back, the vaulted session is the freshly probed one,
// and no second record appears (PLAN.md §7.5: idempotent on the natural key).
func TestLinkAccount_RelinkRefreshesSessionInPlace(t *testing.T) {
	srv, probe, stores, session := newRelinkServer(t, false)
	ctx := ctxAs(session, "user-1")

	first, err := srv.LinkAccount(ctx, linkBob("sekret"))
	if err != nil {
		t.Fatalf("first link: %v", err)
	}

	probe.blob = []byte(`{"AccessToken":"second"}`)
	second, err := srv.LinkAccount(ctx, linkBob("rotated"))
	if err != nil {
		t.Fatalf("re-link: %v", err)
	}
	if first.GetAccountId() != second.GetAccountId() {
		t.Fatalf("re-link minted a new id: %q then %q", first.GetAccountId(), second.GetAccountId())
	}

	st := accounts.NewStore(stores.Vault, nil)
	blob, err := st.Load(ctx, first.GetAccountId())
	if err != nil {
		t.Fatalf("vaulted session: %v", err)
	}
	if string(blob) != `{"AccessToken":"second"}` {
		t.Fatalf("vaulted session = %q, want the freshly probed one", blob)
	}
	recs, err := st.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("records = %d, want the one link, not a duplicate", len(recs))
	}
}

// The "re-link-doesn't-clobber-sharing" fixture: absent visibility and an
// absent cap on a re-link leave the account's audience and budget exactly as
// the owner set them (PLAN.md §7.5 presence semantics).
func TestLinkAccount_RelinkKeepsSharingAndCapWhenAbsent(t *testing.T) {
	srv, _, _, session := newRelinkServer(t, true)
	ownerCtx := ctxAs(session, "user-1")

	shared := apiv1.AccountVisibility_ACCOUNT_VISIBILITY_SHARED
	first := linkBob("sekret")
	first.Visibility = &shared
	first.SharedWith = []string{"user:alice"}
	first.MaxConcurrentStreams = proto.Uint32(2)
	linked, err := srv.LinkAccount(ownerCtx, first)
	if err != nil {
		t.Fatalf("first link: %v", err)
	}

	if _, err := srv.LinkAccount(ownerCtx, linkBob("rotated")); err != nil {
		t.Fatalf("re-link: %v", err)
	}

	resp, err := srv.ListAccounts(ownerCtx, &apiv1.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if got := len(resp.GetAccounts()); got != 1 {
		t.Fatalf("accounts after re-link = %d, want exactly the one re-linked account, not a duplicate", got)
	}
	var found *apiv1.Account
	for _, a := range resp.GetAccounts() {
		if a.GetAccountId() == linked.GetAccountId() {
			found = a
		}
	}
	if found == nil {
		t.Fatal("the re-linked account vanished from the owner's list")
	}
	if found.GetVisibility() != apiv1.AccountVisibility_ACCOUNT_VISIBILITY_SHARED {
		t.Fatalf("visibility after re-link = %v, want SHARED untouched", found.GetVisibility())
	}
	if len(found.GetSharedWith()) != 1 || found.GetSharedWith()[0] != "user:alice" {
		t.Fatalf("roster after re-link = %v, want [user:alice] untouched", found.GetSharedWith())
	}
	if found.GetMaxConcurrentStreams() != 2 {
		t.Fatalf("cap after re-link = %d, want 2 untouched", found.GetMaxConcurrentStreams())
	}
}

// Presence means applied: a present visibility replaces the member list
// wholesale, a present cap replaces the budget (zero = unlimited), exactly
// as the contract's field comments promise.
func TestLinkAccount_RelinkAppliesPresentSharingAndCap(t *testing.T) {
	srv, _, _, session := newRelinkServer(t, true)
	ownerCtx := ctxAs(session, "user-1")

	shared := apiv1.AccountVisibility_ACCOUNT_VISIBILITY_SHARED
	first := linkBob("sekret")
	first.Visibility = &shared
	first.SharedWith = []string{"user:alice"}
	first.MaxConcurrentStreams = proto.Uint32(2)
	linked, err := srv.LinkAccount(ownerCtx, first)
	if err != nil {
		t.Fatalf("first link: %v", err)
	}

	private := apiv1.AccountVisibility_ACCOUNT_VISIBILITY_PRIVATE
	rel := linkBob("rotated")
	rel.Visibility = &private
	rel.MaxConcurrentStreams = proto.Uint32(0)
	if _, err := srv.LinkAccount(ownerCtx, rel); err != nil {
		t.Fatalf("re-link: %v", err)
	}

	resp, err := srv.ListAccounts(ownerCtx, &apiv1.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	var found *apiv1.Account
	for _, a := range resp.GetAccounts() {
		if a.GetAccountId() == linked.GetAccountId() {
			found = a
		}
	}
	if found == nil {
		t.Fatal("the re-linked account vanished from the owner's list")
	}
	if found.GetVisibility() != apiv1.AccountVisibility_ACCOUNT_VISIBILITY_PRIVATE {
		t.Fatalf("visibility after re-link = %v, want the applied PRIVATE", found.GetVisibility())
	}
	if len(found.GetSharedWith()) != 0 {
		t.Fatalf("roster after re-link = %v, want the wholesale replacement to be empty", found.GetSharedWith())
	}
	if found.GetMaxConcurrentStreams() != 0 {
		t.Fatalf("cap after re-link = %d, want the applied 0 (unlimited)", found.GetMaxConcurrentStreams())
	}
}

// The "no cross-user hijack" fixture: a second user re-linking the same
// provider username gets their own separate record — the natural key is
// owner-scoped, so nobody's link can reach another user's account.
func TestLinkAccount_RelinkCannotHijackAcrossUsers(t *testing.T) {
	srv, probe, stores, session := newRelinkServer(t, false)

	u1 := ctxAs(session, "user-1")
	probe.blob = []byte(`{"AccessToken":"session-of-user-1"}`)
	by1, err := srv.LinkAccount(u1, linkBob("sekret"))
	if err != nil {
		t.Fatalf("user-1 link: %v", err)
	}

	u2 := ctxAs(session, "user-2")
	probe.blob = []byte(`{"AccessToken":"session-of-user-2"}`)
	by2, err := srv.LinkAccount(u2, linkBob("their-own-password"))
	if err != nil {
		t.Fatalf("user-2 link: %v", err)
	}
	if by1.GetAccountId() == by2.GetAccountId() {
		t.Fatalf("user-2 hijacked user-1's account: both links returned %q", by1.GetAccountId())
	}

	st := accounts.NewStore(stores.Vault, nil)
	recs, err := st.List(u1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("records = %d, want one per user", len(recs))
	}
	blob, err := st.Load(u1, by1.GetAccountId())
	if err != nil {
		t.Fatalf("user-1 vault: %v", err)
	}
	if string(blob) != `{"AccessToken":"session-of-user-1"}` {
		t.Fatalf("user-1's session = %q; a stranger's link overwrote it", blob)
	}
	rec, err := st.Get(u1, by1.GetAccountId())
	if err != nil {
		t.Fatalf("user-1 record: %v", err)
	}
	if rec.OwnerUserID != "user-1" || rec.Status != accounts.StatusLinked {
		t.Fatalf("user-1's record after user-2's link = %+v, want theirs and still LINKED", rec)
	}
}

// The expiry-flip half of the re-auth flow: an EXPIRED account that re-links
// returns to LINKED under its own id.
func TestLinkAccount_RelinkAfterExpiryReturnsLinked(t *testing.T) {
	srv, _, stores, session := newRelinkServer(t, false)
	ctx := ctxAs(session, "user-1")

	linked, err := srv.LinkAccount(ctx, linkBob("sekret"))
	if err != nil {
		t.Fatalf("first link: %v", err)
	}
	st := accounts.NewStore(stores.Vault, nil)
	if err := st.SetStatus(ctx, linked.GetAccountId(), accounts.StatusExpired); err != nil {
		t.Fatalf("flip to EXPIRED: %v", err)
	}

	again, err := srv.LinkAccount(ctx, linkBob("rotated"))
	if err != nil {
		t.Fatalf("re-link: %v", err)
	}
	if again.GetAccountId() != linked.GetAccountId() {
		t.Fatalf("re-link after expiry returned %q, want the same account %q", again.GetAccountId(), linked.GetAccountId())
	}
	rec, err := st.Get(ctx, linked.GetAccountId())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if rec.Status != accounts.StatusLinked {
		t.Fatalf("status after re-link = %v, want LINKED", rec.Status)
	}
}

// A present UNSPECIFIED is a client error, never read as absence: the
// request dies at validation and the provider is never probed.
func TestLinkAccount_UnspecifiedVisibilityPresentRejected(t *testing.T) {
	srv, probe, _, session := newRelinkServer(t, false)
	ctx := ctxAs(session, "user-1")

	unspec := apiv1.AccountVisibility_ACCOUNT_VISIBILITY_UNSPECIFIED
	req := linkBob("sekret")
	req.Visibility = &unspec
	if _, err := srv.LinkAccount(ctx, req); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("explicit UNSPECIFIED code = %v, want InvalidArgument", status.Code(err))
	}
	if probe.calls != 0 {
		t.Fatalf("probe ran %d times for a request that must die at validation", probe.calls)
	}
}

// A roster on a present non-shared visibility is ambiguous — the same
// discipline as UpdateAccount's wholesale sharing — and dies before the
// probe, leaving the account untouched.
func TestLinkAccount_RelinkOrphanRosterRejected(t *testing.T) {
	srv, probe, _, session := newRelinkServer(t, true)
	ctx := ctxAs(session, "user-1")

	if _, err := srv.LinkAccount(ctx, linkBob("sekret")); err != nil {
		t.Fatalf("first link: %v", err)
	}
	private := apiv1.AccountVisibility_ACCOUNT_VISIBILITY_PRIVATE
	req := linkBob("rotated")
	req.Visibility = &private
	req.SharedWith = []string{"user:alice"}
	if _, err := srv.LinkAccount(ctx, req); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("orphan roster code = %v, want InvalidArgument", status.Code(err))
	}
	if probe.calls != 1 {
		t.Fatalf("probe calls = %d, want only the first link's", probe.calls)
	}
}

// A rejected probe must not clobber anything: the EXPIRED account keeps its
// status and its earlier vaulted session.
func TestLinkAccount_RelinkBadCredentialsLeaveAccountUntouched(t *testing.T) {
	srv, probe, stores, session := newRelinkServer(t, false)
	ctx := ctxAs(session, "user-1")

	linked, err := srv.LinkAccount(ctx, linkBob("sekret"))
	if err != nil {
		t.Fatalf("first link: %v", err)
	}
	st := accounts.NewStore(stores.Vault, nil)
	if err := st.SetStatus(ctx, linked.GetAccountId(), accounts.StatusExpired); err != nil {
		t.Fatalf("flip to EXPIRED: %v", err)
	}

	probe.accept = false
	if _, err := srv.LinkAccount(ctx, linkBob("wrong")); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("bad-credential re-link code = %v, want Unauthenticated", status.Code(err))
	}
	rec, err := st.Get(ctx, linked.GetAccountId())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if rec.Status != accounts.StatusExpired {
		t.Fatalf("status after a refused re-link = %v, want EXPIRED untouched", rec.Status)
	}
	blob, err := st.Load(ctx, linked.GetAccountId())
	if err != nil {
		t.Fatalf("vaulted session: %v", err)
	}
	if string(blob) != `{"AccessToken":"first"}` {
		t.Fatalf("vaulted session = %q, want the earlier one untouched", blob)
	}
}
