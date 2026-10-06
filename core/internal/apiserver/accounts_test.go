package apiserver_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/library"
)

func TestLinkAccount_ProbesBeforeVaulting(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	stores := testStores(t)
	probe := &stubProber{accept: true, blob: []byte(`{"AccessToken":"t"}`)}
	srv := apiserver.NewServer(bus, stores, authenticator, session)
	srv.SetProber("jellyfin", probe)
	ctx := ctxAs(session, "user-1")

	ch := bus.Subscribe("evt-watch", "user-1")
	defer bus.Unsubscribe("evt-watch")

	resp, err := srv.LinkAccount(ctx, &apiv1.LinkAccountRequest{
		Provider: "jellyfin",
		BaseUrl:  "https://jf.example/",
		AuthMethod: &apiv1.LinkAccountRequest_Password{
			Password: &apiv1.AccountPassword{Username: "bob", Password: []byte("sekret")},
		},
	})
	if err != nil {
		t.Fatalf("LinkAccount: %v", err)
	}
	if got := resp.GetAccountId(); !strings.HasPrefix(got, "lnk_") || len(got) != 36 {
		t.Fatalf("account id = %q, want an lnk_ id", got)
	}
	if probe.calls != 1 || probe.user != "bob" || string(probe.pass) != "sekret" {
		t.Fatalf("probe calls=%d user=%q, want one probe of bob", probe.calls, probe.user)
	}
	if probe.baseURL != "https://jf.example" {
		t.Fatalf("probe base_url = %q, want trailing slash trimmed", probe.baseURL)
	}

	// The record is persisted with the probing metadata and defaults to
	// private, and the vaulted session blob is readable under the account id.
	linked := accounts.NewStore(stores.Vault, nil)
	rec, err := linked.Get(context.Background(), resp.GetAccountId())
	if err != nil {
		t.Fatalf("record not persisted: %v", err)
	}
	if rec.Provider != "jellyfin" || rec.BaseURL != "https://jf.example" || rec.Username != "bob" || rec.OwnerUserID != "user-1" {
		t.Fatalf("record = %+v", rec)
	}
	if rec.Visibility != accounts.VisibilityPrivate {
		t.Fatalf("default visibility = %q, want private", rec.Visibility)
	}
	blob, err := linked.Load(context.Background(), resp.GetAccountId())
	if err != nil || string(blob) != `{"AccessToken":"t"}` {
		t.Fatalf("vaulted session blob = %q err=%v", blob, err)
	}

	// A link event was announced to the owner.
	if ev := <-ch; ev.GetType() != corev1.EventType_EVENT_TYPE_ACCOUNT_SESSION_LINKED {
		t.Fatalf("event type = %v, want ACCOUNT_SESSION_LINKED", ev.GetType())
	}
}

func TestLinkAccount_VisibilityAndValidation(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	stores := testStores(t)
	probe := &stubProber{accept: true, blob: []byte{'{'}}
	srv := apiserver.NewServer(bus, stores, authenticator, session)
	srv.SetProber("jellyfin", probe)
	ctx := ctxAs(session, "user-1")

	// Shared with named users maps to the shared visibility, canonicalized by
	// the accounts store.
	resp, err := srv.LinkAccount(ctx, &apiv1.LinkAccountRequest{
		Provider: "jellyfin",
		BaseUrl:  "https://jf.example",
		AuthMethod: &apiv1.LinkAccountRequest_Password{
			Password: &apiv1.AccountPassword{Username: "bob", Password: []byte("sekret")},
		},
		Visibility: apiv1.AccountVisibility_ACCOUNT_VISIBILITY_SHARED,
		SharedWith: []string{"alice", "alice"},
	})
	if err != nil {
		t.Fatalf("shared link: %v", err)
	}
	linked := accounts.NewStore(stores.Vault, nil)
	rec, _ := linked.Get(context.Background(), resp.GetAccountId())
	if rec.Visibility != accounts.VisibilityShared || len(rec.SharedWith) != 1 || rec.SharedWith[0] != "alice" {
		t.Fatalf("shared record = %+v", rec)
	}

	// Rejected probes must not leave anything in the vault.
	rejecting := &stubProber{}
	rejectingServer := apiserver.NewServer(bus, testStores(t), authenticator, session)
	rejectingServer.SetProber("jellyfin", rejecting)
	if _, err := rejectingServer.LinkAccount(ctx, &apiv1.LinkAccountRequest{
		Provider: "jellyfin", BaseUrl: "https://jf.example",
		AuthMethod: &apiv1.LinkAccountRequest_Password{
			Password: &apiv1.AccountPassword{Username: "bob", Password: []byte("nope")},
		},
	}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("rejected probe code = %v, want Unauthenticated", status.Code(err))
	}

	// No prober armed for the provider is Unavailable.
	if _, err := srv.LinkAccount(ctx, &apiv1.LinkAccountRequest{
		Provider: "jellyfin2", BaseUrl: "https://x",
		AuthMethod: &apiv1.LinkAccountRequest_Password{
			Password: &apiv1.AccountPassword{Username: "bob", Password: []byte("s")},
		},
	}); status.Code(err) != codes.Unavailable {
		t.Fatalf("unarmed provider code = %v, want Unavailable", status.Code(err))
	}

	// Shape validation rejects malformed requests before any probe.
	if _, err := srv.LinkAccount(ctx, &apiv1.LinkAccountRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("empty link code = %v, want InvalidArgument", status.Code(err))
	}
	if _, err := srv.LinkAccount(ctx, &apiv1.LinkAccountRequest{
		Provider:   "jellyfin",
		BaseUrl:    "https://x",
		Visibility: apiv1.AccountVisibility_ACCOUNT_VISIBILITY_SHARED,
		AuthMethod: &apiv1.LinkAccountRequest_Password{
			Password: &apiv1.AccountPassword{Username: "bob", Password: []byte("s")},
		},
	}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("shared-without-users code = %v, want InvalidArgument", status.Code(err))
	}
}

func TestListAccounts_VisibilityGate(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	stores := testStores(t)
	op := operatorReach("op-1")

	lib := &stubLibrary{
		reachable: map[string][]string{"op-1": {"user-1", "user-2"}},
		reaches:   []library.Reach{op},
	}
	srv := apiserver.NewServer(bus, stores, authenticator, session)
	srv.SetLibrary(lib)

	// user-1 linked a private account; it must be visible only to them.
	linked := accounts.NewStore(stores.Vault, nil)
	if err := linked.Add(context.Background(), accounts.Record{
		ID: "lnk_aaa", Provider: "jellyfin", BaseURL: "https://jf.example",
		Username: "bob", OwnerUserID: "user-1", Visibility: accounts.VisibilityPrivate,
		Status: accounts.StatusExpired, CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	ownerView, err := srv.ListAccounts(ctxAs(session, "user-1"), &apiv1.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("owner ListAccounts: %v", err)
	}
	byID := map[string]*apiv1.Account{}
	for _, a := range ownerView.GetAccounts() {
		byID[a.GetAccountId()] = a
	}
	linkedAcct, ok := byID["lnk_aaa"]
	if !ok {
		t.Fatalf("owner view lacks the linked account: %+v", byID)
	}
	if !linkedAcct.GetCallerLinked() {
		t.Fatal("linked account not marked caller_linked for its owner")
	}
	if linkedAcct.GetStatus() != apiv1.AccountStatus_ACCOUNT_STATUS_EXPIRED {
		t.Fatalf("status = %v, want EXPIRED", linkedAcct.GetStatus())
	}
	if byID["op-1"] == nil || byID["op-1"].GetCallerLinked() {
		t.Fatal("operator account missing or misflagged")
	}
	if p := byID["op-1"].GetProvider(); p != "jellyfin" {
		t.Fatalf("operator provider = %q, want jellyfin", p)
	}

	// A different user sees the operator account but not the private link.
	other, err := srv.ListAccounts(ctxAs(session, "user-2"), &apiv1.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("other ListAccounts: %v", err)
	}
	for _, a := range other.GetAccounts() {
		if a.GetAccountId() == "lnk_aaa" {
			t.Fatal("other user sees the owner's private account")
		}
	}
	if len(other.GetAccounts()) != 1 || other.GetAccounts()[0].GetAccountId() != "op-1" {
		t.Fatalf("other view = %+v, want only op-1", other.GetAccounts())
	}
}

func TestRemoveAccount_OwnerOnlyAndOperatorProtected(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	stores := testStores(t)
	op := operatorReach("op-1")
	lib := &stubLibrary{reachable: map[string][]string{"op-1": {"user-1"}}, reaches: []library.Reach{op}}
	srv := apiserver.NewServer(bus, stores, authenticator, session)
	srv.SetLibrary(lib)

	linked := accounts.NewStore(stores.Vault, nil)
	if err := linked.Add(context.Background(), accounts.Record{
		ID: "lnk_aaa", Provider: "jellyfin", BaseURL: "https://jf.example",
		Username: "bob", OwnerUserID: "user-1", Visibility: accounts.VisibilityPrivate,
		Status: accounts.StatusLinked, CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	// Operator-declared accounts are read-only.
	if _, err := srv.RemoveAccount(ctxAs(session, "user-1"), &apiv1.RemoveAccountRequest{AccountId: "op-1"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("removing operator account code = %v, want PermissionDenied", status.Code(err))
	}

	// Only the owner may remove a linked account.
	if _, err := srv.RemoveAccount(ctxAs(session, "user-2"), &apiv1.RemoveAccountRequest{AccountId: "lnk_aaa"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("non-owner removal code = %v, want PermissionDenied", status.Code(err))
	}

	// The owner's removal works, drops the live reach, and revokes the session.
	if _, err := srv.RemoveAccount(ctxAs(session, "user-1"), &apiv1.RemoveAccountRequest{AccountId: "lnk_aaa"}); err != nil {
		t.Fatalf("owner removal: %v", err)
	}
	if _, err := linked.Get(context.Background(), "lnk_aaa"); !errors.Is(err, accounts.ErrNotFound) {
		t.Fatalf("record survives removal: %v", err)
	}
	if len(lib.removed) != 1 || lib.removed[0] != "lnk_aaa" {
		t.Fatalf("removed reaches = %v", lib.removed)
	}

	// An unknown, unreachable account is NotFound.
	if _, err := srv.RemoveAccount(ctxAs(session, "user-1"), &apiv1.RemoveAccountRequest{AccountId: "lnk_zzz"}); status.Code(err) != codes.NotFound {
		t.Fatalf("unknown account code = %v, want NotFound", status.Code(err))
	}
}

// The member-existence seam must distinguish a real user from a mistyped id,
// a bare username (missing the id prefix), and the empty case — so a typo in
// a share roster is refused with the user's name, never silently granted to
// nobody.
func TestUserDirectoryAcceptsOnlyRealUserIDs(t *testing.T) {
	users := auth.NewMemoryUserStore()
	if err := users.PutUser("alice", &auth.UserData{Salt: []byte{1, 2, 3}}); err != nil {
		t.Fatalf("PutUser: %v", err)
	}
	dir := apiserver.NewUserDirectory(users)
	if dir == nil {
		t.Fatal("directory built over a real store is nil")
	}
	for id, want := range map[string]bool{
		"user:alice":      true,
		"user:bob":        false,
		"alice":           false,
		"user:":           false,
		"":                false,
		"user:user:alice": false,
	} {
		got, err := dir.HasUser(id)
		if err != nil {
			t.Fatalf("HasUser(%q): %v", id, err)
		}
		if got != want {
			t.Errorf("HasUser(%q) = %v, want %v", id, got, want)
		}
	}
	if dir := apiserver.NewUserDirectory(nil); dir != nil {
		t.Fatal("directory over a nil store should be nil")
	}
}

// A roster naming someone who does not exist is refused at link time, by
// name, and the provider is never probed — sharing cannot be granted to a
// ghost.
func TestLinkAccount_SharedRosterNamesRealUsers(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	stores := testStores(t)
	probe := &stubProber{accept: true, blob: []byte(`{"AccessToken":"t"}`)}
	srv := apiserver.NewServer(bus, stores, authenticator, session)
	srv.SetProber("jellyfin", probe)

	users := auth.NewMemoryUserStore()
	if err := users.PutUser("alice", &auth.UserData{Salt: []byte{1}}); err != nil {
		t.Fatalf("PutUser: %v", err)
	}
	srv.SetUserDirectory(apiserver.NewUserDirectory(users))

	ctx := ctxAs(session, "user-1")
	pw := &apiv1.LinkAccountRequest_Password{Password: &apiv1.AccountPassword{Username: "bob", Password: []byte("sekret")}}
	if _, err := srv.LinkAccount(ctx, &apiv1.LinkAccountRequest{
		Provider: "jellyfin", BaseUrl: "https://jf.example/", AuthMethod: pw,
		Visibility: apiv1.AccountVisibility_ACCOUNT_VISIBILITY_SHARED,
		SharedWith: []string{"user:does-not-exist"},
	}); status.Code(err) != codes.InvalidArgument || !strings.Contains(err.Error(), "user:does-not-exist") {
		t.Fatalf("unknown member: err = %v, want InvalidArgument naming the user", err)
	}
	if probe.calls != 0 {
		t.Fatalf("probe ran %d times for a rejected roster", probe.calls)
	}
	if _, err := srv.LinkAccount(ctx, &apiv1.LinkAccountRequest{
		Provider: "jellyfin", BaseUrl: "https://jf.example/", AuthMethod: pw,
		Visibility: apiv1.AccountVisibility_ACCOUNT_VISIBILITY_SHARED,
		SharedWith: []string{"user:alice"},
	}); err != nil {
		t.Fatalf("real member should link: %v", err)
	}
}

// The owner can update a linked account's settings live: the record
// carries the new values, the response view shows them, and the delivery
// engine is told to end sessions who lost access (narrower than public)
// and to enforce a lowered cap the same way admission would. Nothing about
// the sharing change leaks to members of the account — the member path
// through ListAccounts shows no cap or policy fields.
func TestUpdateAccount_OwnerChangesSharingAndCap(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	stores := testStores(t)
	probe := &stubProber{accept: true, blob: []byte(`{"AccessToken":"t"}`)}
	srv := apiserver.NewServer(bus, stores, authenticator, session)
	srv.SetProber("jellyfin", probe)
	users := auth.NewMemoryUserStore()
	for _, u := range []string{"alice", "carol"} {
		if err := users.PutUser(u, &auth.UserData{Salt: []byte{1}}); err != nil {
			t.Fatalf("PutUser: %v", err)
		}
	}
	srv.SetUserDirectory(apiserver.NewUserDirectory(users))
	dev := &stubDelivery{}
	srv.SetDelivery(dev)

	ownerCtx := ctxAs(session, "user-1")
	link, err := srv.LinkAccount(ownerCtx, &apiv1.LinkAccountRequest{
		Provider: "jellyfin", BaseUrl: "https://jf.example/",
		AuthMethod: &apiv1.LinkAccountRequest_Password{Password: &apiv1.AccountPassword{Username: "bob", Password: []byte("x")}},
		Visibility: apiv1.AccountVisibility_ACCOUNT_VISIBILITY_PUBLIC,
	})
	if err != nil {
		t.Fatalf("LinkAccount: %v", err)
	}

	res, err := srv.UpdateAccount(ownerCtx, &apiv1.UpdateAccountRequest{
		AccountId: link.GetAccountId(),
		Sharing: &apiv1.AccountSharing{
			Visibility: apiv1.AccountVisibility_ACCOUNT_VISIBILITY_SHARED,
			SharedWith: []string{"user:alice"},
		},
		MaxConcurrentStreams: proto.Uint32(2),
		CapChangePolicy:      apiv1.CapChangePolicy_CAP_CHANGE_POLICY_ENFORCE_NOW,
	})
	if err != nil {
		t.Fatalf("UpdateAccount: %v", err)
	}
	got := res.GetAccount()
	if got.GetVisibility() != apiv1.AccountVisibility_ACCOUNT_VISIBILITY_SHARED || len(got.GetSharedWith()) != 1 || got.GetSharedWith()[0] != "user:alice" {
		t.Fatalf("sharing = %v %v, want shared [user:alice]", got.GetVisibility(), got.GetSharedWith())
	}
	if got.GetMaxConcurrentStreams() != 2 || got.GetCapChangePolicy() != apiv1.CapChangePolicy_CAP_CHANGE_POLICY_ENFORCE_NOW || got.GetEffectiveCapChangePolicy() != apiv1.CapChangePolicy_CAP_CHANGE_POLICY_ENFORCE_NOW {
		t.Fatalf("cap/policy = %d/%v/%v, want 2/enforce-now/enforce-now", got.GetMaxConcurrentStreams(), got.GetCapChangePolicy(), got.GetEffectiveCapChangePolicy())
	}
	// The live effects: exactly one revoke sweep keeping the owner and the
	// newly named member, and one cap-enforcement call in enforce-now mode.
	if len(dev.revokes) != 1 || dev.revokes[0].accountID != link.GetAccountId() || len(dev.revokes[0].keep) != 2 || dev.revokes[0].keep[0] != "user-1" || dev.revokes[0].keep[1] != "user:alice" {
		t.Fatalf("revokes = %+v, want one with keep=[user-1 user:alice]", dev.revokes)
	}
	if len(dev.applies) != 1 || !dev.applies[0].enforceNow {
		t.Fatalf("applies = %+v, want one enforceNow", dev.applies)
	}
	// The store carries the same values.
	rec, err := accounts.NewStore(stores.Vault, nil).Get(context.Background(), link.GetAccountId())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if rec.Visibility != accounts.VisibilityShared || len(rec.SharedWith) != 1 || rec.MaxConcurrentStreams != 2 || rec.CapChangePolicy != accounts.CapChangePolicyEnforceNow {
		t.Fatalf("record = %+v", rec)
	}

	// The member-visible view hides the owner's cap and policy fields — the
	// roster tells a member the account is shared, never the owner's budget.
	// The reach itself still carries the link-time sharing until the live
	// swap lands with the library change; arming the stub view as the member
	// sees it pins what the API leaks across that boundary now.
	srv.SetLibrary(&stubLibrary{
		reachable: map[string][]string{link.GetAccountId(): {"user-1", "user:alice"}},
		reaches: []library.Reach{{
			AccountID:  link.GetAccountId(),
			Owner:      "user-1",
			Visibility: accounts.VisibilityShared,
			Members:    []string{"user:alice"},
		}},
	})
	aliCtx := ctxAs(session, "user:alice")
	list, err := srv.ListAccounts(aliCtx, &apiv1.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	var member *apiv1.Account
	for _, a := range list.GetAccounts() {
		if a.GetAccountId() == link.GetAccountId() {
			member = a
		}
	}
	if member == nil {
		t.Fatal("member does not see the shared account")
	}
	if member.GetVisibility() != apiv1.AccountVisibility_ACCOUNT_VISIBILITY_SHARED || len(member.GetSharedWith()) != 0 ||
		member.GetMaxConcurrentStreams() != 0 || member.GetCapChangePolicy() != apiv1.CapChangePolicy_CAP_CHANGE_POLICY_UNSPECIFIED ||
		member.GetEffectiveCapChangePolicy() != apiv1.CapChangePolicy_CAP_CHANGE_POLICY_UNSPECIFIED {
		t.Fatalf("member view = %+v, want shared, no roster, no owner settings", member)
	}
}

// The update gate is strict: only the owner, and only caller-linked
// accounts. An unknown member is rejected and changes nothing, and a
// request that changes nothing at all is rejected rather than treated as
// a no-op success.
func TestUpdateAccount_GatesAndNoOps(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	stores := testStores(t)
	probe := &stubProber{accept: true, blob: []byte(`{"AccessToken":"t"}`)}
	srv := apiserver.NewServer(bus, stores, authenticator, session)
	srv.SetProber("jellyfin", probe)
	srv.SetUserDirectory(apiserver.NewUserDirectory(auth.NewMemoryUserStore()))
	srv.SetDelivery(&stubDelivery{})

	ownerCtx := ctxAs(session, "user-1")
	link, err := srv.LinkAccount(ownerCtx, &apiv1.LinkAccountRequest{
		Provider: "jellyfin", BaseUrl: "https://jf.example/",
		AuthMethod: &apiv1.LinkAccountRequest_Password{Password: &apiv1.AccountPassword{Username: "bob", Password: []byte("x")}},
	})
	if err != nil {
		t.Fatalf("LinkAccount: %v", err)
	}
	// Not the owner.
	if _, err := srv.UpdateAccount(ctxAs(session, "user-2"), &apiv1.UpdateAccountRequest{
		AccountId:            link.GetAccountId(),
		MaxConcurrentStreams: proto.Uint32(1),
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("non-owner: err=%v, want PermissionDenied", err)
	}
	// Unknown id.
	if _, err := srv.UpdateAccount(ownerCtx, &apiv1.UpdateAccountRequest{
		AccountId:            "lnk_missing",
		MaxConcurrentStreams: proto.Uint32(1),
	}); status.Code(err) != codes.NotFound {
		t.Fatalf("unknown: err=%v, want NotFound", err)
	}
	// Nothing to change.
	if _, err := srv.UpdateAccount(ownerCtx, &apiv1.UpdateAccountRequest{AccountId: link.GetAccountId()}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("no-op: err=%v, want InvalidArgument", err)
	}
	// An unknown member name in the roster is refused, and the record is untouched.
	if _, err := srv.UpdateAccount(ownerCtx, &apiv1.UpdateAccountRequest{
		AccountId: link.GetAccountId(),
		Sharing: &apiv1.AccountSharing{
			Visibility: apiv1.AccountVisibility_ACCOUNT_VISIBILITY_SHARED,
			SharedWith: []string{"user:nobody"},
		},
	}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("unknown member: err=%v, want InvalidArgument", err)
	}
	rec, _ := accounts.NewStore(stores.Vault, nil).Get(context.Background(), link.GetAccountId())
	if rec.Visibility != accounts.VisibilityPrivate || rec.MaxConcurrentStreams != 0 {
		t.Fatalf("rejected update leaked into the record: %+v", rec)
	}
}
