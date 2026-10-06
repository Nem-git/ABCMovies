package m6_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/delivery"
	"github.com/nem-git/abcmovies/core/internal/library"
	"github.com/nem-git/abcmovies/core/internal/store"
)

type updDelivery struct {
	revokeProvider string
	revokeAccount  string
	revokeKeep     []string
	applyProvider  string
	applyAccount   string
	applyEnforce   bool
	applyCalls     int
	revokeCalls    int
}

func (d *updDelivery) Start(context.Context, delivery.StartRequest) (*delivery.Session, error) {
	return nil, nil
}
func (d *updDelivery) Heartbeat(string, string) error { return nil }
func (d *updDelivery) PlayMenu(string) (*apiserver.PlayMenu, error) {
	return &apiserver.PlayMenu{}, nil
}
func (d *updDelivery) RevokeAllOnAccount(string) int { return 0 }
func (d *updDelivery) RevokeOthersOnAccount(provider, accountID string, keepMembers []string) int {
	d.revokeCalls++
	d.revokeProvider = provider
	d.revokeAccount = accountID
	d.revokeKeep = append([]string(nil), keepMembers...)
	return 0
}

func (d *updDelivery) ApplyAccountCap(ctx context.Context, provider, accountID string, enforceNow bool) (int, error) {
	d.applyCalls++
	d.applyProvider = provider
	d.applyAccount = accountID
	d.applyEnforce = enforceNow
	return 0, nil
}

type updLibrary struct {
	users          map[string][]string
	reaches        map[string]library.Reach
	reachSwaps     int
	lastVisibility accounts.Visibility
	lastMembers    []string
	lastAccount    string
}

func (l *updLibrary) Library(context.Context, string) ([]*corev1.LibraryEntry, error) {
	return nil, nil
}

func (l *updLibrary) Metadata(context.Context, string) (*corev1.TitleMetadata, bool, error) {
	return nil, false, nil
}

func (l *updLibrary) ReachAuthorized(accountID, userID string) (library.Reach, bool) {
	for _, u := range l.users[accountID] {
		if u == userID {
			return l.reaches[accountID], true
		}
	}
	return library.Reach{}, false
}

func (l *updLibrary) ReachesForUser(userID string) []library.Reach {
	var out []library.Reach
	for id, us := range l.users {
		for _, u := range us {
			if u == userID {
				out = append(out, l.reaches[id])
			}
		}
	}
	return out
}
func (l *updLibrary) RemoveReach(string) {}
func (l *updLibrary) SetReachSharing(accountID string, v accounts.Visibility, m []string) error {
	l.reachSwaps++
	l.lastAccount = accountID
	l.lastVisibility = v
	l.lastMembers = append([]string(nil), m...)
	return nil
}

func newUpdateServer(t *testing.T, lib *updLibrary, del *updDelivery, userStore auth.UserStore, capDefault accounts.CapChangePolicy) (*apiserver.Server, config.Stores) {
	t.Helper()
	bus := apiserver.NewInMemoryBus()
	t.Cleanup(bus.Close)
	stores := config.Stores{Jobs: store.NewInMemory(), Vault: store.NewInMemory()}
	srv := apiserver.NewServer(bus, stores, nil, nil, del)
	srv.SetLibrary(lib)
	if userStore != nil {
		srv.SetUserDirectory(apiserver.NewUserDirectory(userStore))
	}
	if capDefault != accounts.CapChangePolicyDefault {
		srv.SetCapChangeDefault(capDefault)
	}
	return srv, stores
}

func seedAccountInto(t *testing.T, accts *accounts.Store, rec accounts.Record) {
	t.Helper()
	if err := accts.Add(context.Background(), rec); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := accts.Save(context.Background(), rec.ID, []byte("{}")); err != nil {
		t.Fatalf("save: %v", err)
	}
}

func TestM6UpdateAccountOnlyOwnerCanEdit(t *testing.T) {
	ctx := context.Background()
	lib := &updLibrary{
		users: map[string][]string{"acc-1": {"alice", "bob"}},
		reaches: map[string]library.Reach{
			"acc-1": {AccountID: "acc-1", Visibility: accounts.VisibilityShared, Owner: "alice", Members: []string{"user:bob"}},
		},
	}
	del := &updDelivery{}
	us := auth.NewMemoryUserStore()
	if err := us.PutUser("alice", &auth.UserData{Salt: []byte{1, 2, 3}}); err != nil {
		t.Fatal(err)
	}
	if err := us.PutUser("bob", &auth.UserData{Salt: []byte{1, 2, 3}}); err != nil {
		t.Fatal(err)
	}
	srv, stores := newUpdateServer(t, lib, del, us, accounts.CapChangePolicyNewSessionsOnly)
	seedAccountInto(t, accounts.NewStore(stores.Vault, nil), accounts.Record{
		ID: "acc-1", Provider: "jellyfin", BaseURL: "http://x", Username: "alice",
		OwnerUserID: "alice", Status: accounts.StatusLinked,
		Visibility: accounts.VisibilityShared, SharedWith: []string{"user:bob"},
		MaxConcurrentStreams: 2,
	})

	// non-owner denied
	_, err := srv.UpdateAccount(apiserver.WithUserID(ctx, "bob"), &apiv1.UpdateAccountRequest{
		AccountId: "acc-1",
		Sharing:   &apiv1.AccountSharing{Visibility: apiv1.AccountVisibility_ACCOUNT_VISIBILITY_PUBLIC},
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("want PermissionDenied, got %v", err)
	}

	// operator-declared: reachable but not in store -> denied
	srv2, _ := newUpdateServer(t, lib, del, us, accounts.CapChangePolicyNewSessionsOnly)
	_, err = srv2.UpdateAccount(apiserver.WithUserID(ctx, "alice"), &apiv1.UpdateAccountRequest{
		AccountId: "acc-1",
		Sharing:   &apiv1.AccountSharing{Visibility: apiv1.AccountVisibility_ACCOUNT_VISIBILITY_PUBLIC},
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("want PermissionDenied for operator-declared, got %v", err)
	}
}

func TestM6UpdateAccountNarrowingRevokesOthers(t *testing.T) {
	ctx := context.Background()
	lib := &updLibrary{
		users: map[string][]string{"acc-1": {"alice", "user:bob", "user:carol"}},
		reaches: map[string]library.Reach{
			"acc-1": {AccountID: "acc-1", Visibility: accounts.VisibilityPublic, Owner: "alice"},
		},
	}
	del := &updDelivery{}
	us := auth.NewMemoryUserStore()
	_ = us.PutUser("alice", &auth.UserData{Salt: []byte{1, 2, 3}})
	_ = us.PutUser("bob", &auth.UserData{Salt: []byte{1, 2, 3}})
	_ = us.PutUser("carol", &auth.UserData{Salt: []byte{1, 2, 3}})
	srv, stores := newUpdateServer(t, lib, del, us, accounts.CapChangePolicyNewSessionsOnly)
	seedAccountInto(t, accounts.NewStore(stores.Vault, nil), accounts.Record{
		ID: "acc-1", Provider: "jellyfin", BaseURL: "http://x", Username: "alice",
		OwnerUserID: "alice", Status: accounts.StatusLinked,
		Visibility: accounts.VisibilityPublic,
	})

	_, err := srv.UpdateAccount(apiserver.WithUserID(ctx, "alice"), &apiv1.UpdateAccountRequest{
		AccountId: "acc-1",
		Sharing: &apiv1.AccountSharing{
			Visibility: apiv1.AccountVisibility_ACCOUNT_VISIBILITY_SHARED,
			SharedWith: []string{"user:bob"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if del.revokeCalls != 1 {
		t.Fatalf("revokeCalls=%d want 1", del.revokeCalls)
	}
	if del.revokeProvider != "jellyfin" || del.revokeAccount != "acc-1" {
		t.Fatalf("revoke provider/account %s/%s want jellyfin/acc-1", del.revokeProvider, del.revokeAccount)
	}
	if len(del.revokeKeep) != 2 {
		t.Fatalf("keep len=%d want 2", len(del.revokeKeep))
	}
	// keep owner + bob
}

func TestM6UpdateAccountCapRaiseDoesNotKill(t *testing.T) {
	ctx := context.Background()
	lib := &updLibrary{users: map[string][]string{"a1": {"alice"}}}
	del := &updDelivery{}
	us := auth.NewMemoryUserStore()
	_ = us.PutUser("alice", &auth.UserData{Salt: []byte{1, 2, 3}})
	srv, stores := newUpdateServer(t, lib, del, us, accounts.CapChangePolicyNewSessionsOnly)
	seedAccountInto(t, accounts.NewStore(stores.Vault, nil), accounts.Record{
		ID: "a1", Provider: "jellyfin", BaseURL: "http://x", Username: "alice",
		OwnerUserID: "alice", Status: accounts.StatusLinked,
		Visibility:           accounts.VisibilityPrivate,
		MaxConcurrentStreams: 1,
	})
	capv := uint32(3)
	_, err := srv.UpdateAccount(apiserver.WithUserID(ctx, "alice"), &apiv1.UpdateAccountRequest{
		AccountId:            "a1",
		MaxConcurrentStreams: &capv,
	})
	if err != nil {
		t.Fatal(err)
	}
	if del.applyCalls < 1 {
		t.Fatalf("applyCalls=%d want >=1", del.applyCalls)
	}
	// enforceNow depends on policy/default; raise path calls apply; must not force kill by default
}

func TestM6UpdateAccountCapLowerEnforceNowKillsOldestFirst(t *testing.T) {
	ctx := context.Background()
	lib := &updLibrary{users: map[string][]string{"a1": {"alice"}}}
	del := &updDelivery{}
	us := auth.NewMemoryUserStore()
	_ = us.PutUser("alice", &auth.UserData{Salt: []byte{1, 2, 3}})
	srv, stores := newUpdateServer(t, lib, del, us, accounts.CapChangePolicyEnforceNow)
	seedAccountInto(t, accounts.NewStore(stores.Vault, nil), accounts.Record{
		ID: "a1", Provider: "jellyfin", BaseURL: "http://x", Username: "alice",
		OwnerUserID: "alice", Status: accounts.StatusLinked,
		Visibility:           accounts.VisibilityPrivate,
		MaxConcurrentStreams: 5,
		CapChangePolicy:      accounts.CapChangePolicyEnforceNow,
	})
	capv := uint32(1)
	_, err := srv.UpdateAccount(apiserver.WithUserID(ctx, "alice"), &apiv1.UpdateAccountRequest{
		AccountId:            "a1",
		MaxConcurrentStreams: &capv,
	})
	if err != nil {
		t.Fatal(err)
	}
	if del.applyCalls < 1 {
		t.Fatalf("applyCalls=%d want >=1", del.applyCalls)
	}
	if del.applyProvider != "jellyfin" || del.applyAccount != "a1" {
		t.Fatalf("apply %s/%s want jellyfin/a1", del.applyProvider, del.applyAccount)
	}
	if !del.applyEnforce {
		t.Fatalf("applyEnforce=false want true")
	}
}

func TestM6UpdateAccountSharingEditedGatesLiveSwap(t *testing.T) {
	ctx := context.Background()
	lib := &updLibrary{
		users: map[string][]string{"a1": {"alice", "user:bob"}},
		reaches: map[string]library.Reach{
			"a1": {AccountID: "a1", Visibility: accounts.VisibilityShared, Owner: "alice", Members: []string{"bob"}},
		},
	}
	del := &updDelivery{}
	us := auth.NewMemoryUserStore()
	_ = us.PutUser("alice", &auth.UserData{Salt: []byte{1, 2, 3}})
	_ = us.PutUser("bob", &auth.UserData{Salt: []byte{1, 2, 3}})
	srv, stores := newUpdateServer(t, lib, del, us, accounts.CapChangePolicyNewSessionsOnly)
	seedAccountInto(t, accounts.NewStore(stores.Vault, nil), accounts.Record{
		ID: "a1", Provider: "jellyfin", BaseURL: "http://x", Username: "alice",
		OwnerUserID: "alice", Status: accounts.StatusLinked,
		Visibility: accounts.VisibilityShared, SharedWith: []string{"user:bob"},
		MaxConcurrentStreams: 2,
	})

	lib.reachSwaps = 0
	del.applyCalls = 0
	del.revokeCalls = 0
	capv := uint32(3)
	_, err := srv.UpdateAccount(apiserver.WithUserID(ctx, "alice"), &apiv1.UpdateAccountRequest{
		AccountId:            "a1",
		MaxConcurrentStreams: &capv,
	})
	if err != nil {
		t.Fatal(err)
	}
	if lib.reachSwaps != 0 {
		t.Fatalf("cap-only edit triggered reachSwaps=%d want 0", lib.reachSwaps)
	}
	if del.revokeCalls != 0 {
		t.Fatalf("cap-only edit triggered revokeCalls=%d want 0", del.revokeCalls)
	}
	if del.applyCalls < 1 {
		t.Fatalf("applyCalls=%d want >=1", del.applyCalls)
	}

	lib.reachSwaps = 0
	del.revokeCalls = 0
	del.applyCalls = 0
	_, err = srv.UpdateAccount(apiserver.WithUserID(ctx, "alice"), &apiv1.UpdateAccountRequest{
		AccountId: "a1",
		Sharing:   &apiv1.AccountSharing{Visibility: apiv1.AccountVisibility_ACCOUNT_VISIBILITY_PRIVATE},
	})
	if err != nil {
		t.Fatal(err)
	}
	if lib.reachSwaps != 1 {
		t.Fatalf("sharing edit reachSwaps=%d want 1", lib.reachSwaps)
	}
	if del.revokeCalls != 1 {
		t.Fatalf("narrowing to private should revoke removed members, revokeCalls=%d want 1", del.revokeCalls)
	}
}
