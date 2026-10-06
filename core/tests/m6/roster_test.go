package m6_test

import (
	"context"
	"testing"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/library"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// The member roster of an account is owner-only. A member of a shared
// account learns that the account is shared — Visibility tells them that —
// but never whom with. That is the member-boundary invariant (PLAN.md §2.2)
// applied to the account view.
type rosterLibrary struct {
	users   map[string][]string // account id -> who may derive it
	reaches map[string]library.Reach
}

func (l *rosterLibrary) Library(context.Context, string) ([]*corev1.LibraryEntry, error) {
	return nil, nil
}

func (l *rosterLibrary) Metadata(context.Context, string) (*corev1.TitleMetadata, bool, error) {
	return nil, false, nil
}

func (l *rosterLibrary) ReachAuthorized(accountID, userID string) (library.Reach, bool) {
	for _, u := range l.users[accountID] {
		if u == userID {
			return l.reaches[accountID], true
		}
	}
	return library.Reach{}, false
}

func (l *rosterLibrary) ReachesForUser(userID string) []library.Reach {
	var out []library.Reach
	for id, users := range l.users {
		for _, u := range users {
			if u == userID {
				out = append(out, l.reaches[id])
			}
		}
	}
	return out
}

func (l *rosterLibrary) RemoveReach(string) {}
func (l *rosterLibrary) SetReachSharing(string, accounts.Visibility, []string) error {
	return nil
}

func newRosterServer(t *testing.T, lib *rosterLibrary) *apiserver.Server {
	t.Helper()
	bus := apiserver.NewInMemoryBus()
	t.Cleanup(bus.Close)
	stores := config.Stores{Jobs: store.NewInMemory(), Vault: store.NewInMemory()}
	srv := apiserver.NewServer(bus, stores, nil, nil, &stubDelivery{})
	srv.SetLibrary(lib)
	accts := accounts.NewStore(stores.Vault, nil)
	rec := accounts.Record{
		ID: "acc-shared", Provider: "jellyfin", BaseURL: "http://x", Username: "alice.homeserver.user",
		OwnerUserID: "alice", Status: accounts.StatusLinked,
		Visibility: accounts.VisibilityShared, SharedWith: []string{"bob"},
	}
	if err := accts.Add(context.Background(), rec); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if err := accts.Save(context.Background(), rec.ID, []byte("{}")); err != nil {
		t.Fatalf("seed vault blob: %v", err)
	}
	return srv
}

func TestM6AccountRosterIsOwnerOnly(t *testing.T) {
	lib := &rosterLibrary{
		users: map[string][]string{
			"acc-shared":      {"alice", "bob"},
			"acc-public-host": {"alice", "bob", "carol"},
		},
		reaches: map[string]library.Reach{
			"acc-shared":      {AccountID: "acc-shared", Visibility: accounts.VisibilityShared, Owner: "alice", Members: []string{"bob"}},
			"acc-public-host": {AccountID: "acc-public-host", Visibility: accounts.VisibilityPublic, Members: []string{"alice", "bob"}},
		},
	}
	srv := newRosterServer(t, lib)

	alice, err := srv.ListAccounts(apiserver.WithUserID(context.Background(), "alice"), &apiv1.ListAccountsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var shared, publicHost *apiv1.Account
	for _, a := range alice.GetAccounts() {
		switch a.GetAccountId() {
		case "acc-shared":
			shared = a
		case "acc-public-host":
			publicHost = a
		}
	}
	if shared == nil || publicHost == nil {
		t.Fatalf("alice should see both accounts: %#v", alice.GetAccounts())
	}
	if len(shared.GetSharedWith()) != 1 || shared.GetSharedWith()[0] != "bob" {
		t.Errorf("owner sees roster %v, want [bob]", shared.GetSharedWith())
	}
	if len(publicHost.GetSharedWith()) != 0 {
		t.Errorf("host-provided account carries roster %v, want none", publicHost.GetSharedWith())
	}

	bob, err := srv.ListAccounts(apiserver.WithUserID(context.Background(), "bob"), &apiv1.ListAccountsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range bob.GetAccounts() {
		if len(a.GetSharedWith()) != 0 {
			t.Errorf("member bob sees roster %v on %q, want none", a.GetSharedWith(), a.GetAccountId())
		}
	}

	carol, err := srv.ListAccounts(apiserver.WithUserID(context.Background(), "carol"), &apiv1.ListAccountsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(carol.GetAccounts()) != 1 || carol.GetAccounts()[0].GetAccountId() != "acc-public-host" {
		t.Fatalf("carol sees %d accounts, want only the public host one", len(carol.GetAccounts()))
	}
	if len(carol.GetAccounts()[0].GetSharedWith()) != 0 {
		t.Errorf("host-provided account carries roster %v on carol's view, want none", carol.GetAccounts()[0].GetSharedWith())
	}
}
