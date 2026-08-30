package m5_test

import (
	"strings"
	"testing"
	"time"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestM5SeedLinkVisibleToOwnerAndInvisibleToOthers proves the linked-account
// read surface over the wire: alice's pre-seeded private link shows up in her
// ListAccounts exactly as it was linked (caller_linked, provider, server),
// and it stays invisible to bob — a private linked account feeds only its
// owner's view (PLAN.md §5.1, §7.5).
func TestM5SeedLinkVisibleToOwnerAndInvisibleToOthers(t *testing.T) {
	stack := newM5Stack(t, fakeJellyfinServer(t))
	client := apiv1.NewCoreServiceClient(startWireServer(t, stack))

	alice, err := client.ListAccounts(authedCtx(t.Context(), stack.aliceToken), &apiv1.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("alice ListAccounts: %v", err)
	}
	if len(alice.GetAccounts()) != 1 {
		t.Fatalf("alice sees %d accounts, want exactly her private linked one", len(alice.GetAccounts()))
	}
	a := alice.GetAccounts()[0]
	if a.GetAccountId() != "lnk_alice_home" {
		t.Errorf("account id = %q, want lnk_alice_home", a.GetAccountId())
	}
	if !a.GetCallerLinked() {
		t.Error("caller_linked = false, want true (alice linked this account)")
	}
	if a.GetProvider() != "jellyfin" {
		t.Errorf("provider = %q, want jellyfin", a.GetProvider())
	}
	if got := strings.TrimRight(a.GetBaseUrl(), "/"); got != stack.baseURL {
		t.Errorf("base_url = %q, want %q", a.GetBaseUrl(), stack.baseURL)
	}
	if a.GetStatus() != apiv1.AccountStatus_ACCOUNT_STATUS_LINKED {
		t.Errorf("status = %v, want linked", a.GetStatus())
	}

	bob, err := client.ListAccounts(authedCtx(t.Context(), stack.bobToken), &apiv1.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("bob ListAccounts: %v", err)
	}
	if len(bob.GetAccounts()) != 0 {
		t.Fatalf("bob sees %d accounts, want 0 (alice's private link is owner-only)", len(bob.GetAccounts()))
	}
}

// TestM5LinkAccountOverWireProbesThenVaults proves LinkAccount end-to-end:
// the credentials are probed against the provider, the confirmed session is
// vaulted, a fresh account id is minted, the owner's link event is published,
// and the linked account becomes visible to its owner (PLAN.md §3.5, §7.5).
func TestM5LinkAccountOverWireProbesThenVaults(t *testing.T) {
	stack := newM5Stack(t, fakeJellyfinServer(t))
	client := apiv1.NewCoreServiceClient(startWireServer(t, stack))
	bobCtx := authedCtx(t.Context(), stack.bobToken)

	// Subscribe to bob's owner events before triggering the link: the bus is
	// ephemeral (PLAN.md §9.2).
	evCh := stack.bus.Subscribe("m5-link-bob", stack.bob.UserID)
	defer stack.bus.Unsubscribe("m5-link-bob")

	link, err := client.LinkAccount(bobCtx, &apiv1.LinkAccountRequest{
		Provider: "jellyfin",
		BaseUrl:  stack.baseURL, // use provider server URL directly
		AuthMethod: &apiv1.LinkAccountRequest_Password{
			Password: &apiv1.AccountPassword{Username: "bob.secondserver.user", Password: []byte("pw-secret")},
		},
	})
	if err != nil {
		t.Fatalf("LinkAccount: %v", err)
	}
	if link.GetAccountId() == "" {
		t.Fatal("LinkAccount returned an empty account id")
	}

	// The link event is the at-most-once notification to its owner.
	select {
	case env := <-evCh:
		if env.GetType() != corev1.EventType_EVENT_TYPE_ACCOUNT_SESSION_LINKED {
			t.Fatalf("event type = %v, want account-session-linked", env.GetType())
		}
		if env.GetAccountSession().GetAccountId() != link.GetAccountId() {
			t.Fatalf("event account = %q, want %q", env.GetAccountSession().GetAccountId(), link.GetAccountId())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the account-session-linked event")
	}

	// Bob's view now carries his link; alice's private link is still invisible.
	bobs, err := client.ListAccounts(bobCtx, &apiv1.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("bob ListAccounts: %v", err)
	}
	if len(bobs.GetAccounts()) != 1 {
		t.Fatalf("bob sees %d accounts, want exactly his new link", len(bobs.GetAccounts()))
	}
	if got := bobs.GetAccounts()[0].GetAccountId(); got != link.GetAccountId() {
		t.Errorf("bob's account id = %q, want %q", got, link.GetAccountId())
	}

	alice, err := client.ListAccounts(authedCtx(t.Context(), stack.aliceToken), &apiv1.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("alice ListAccounts: %v", err)
	}
	if len(alice.GetAccounts()) != 1 || alice.GetAccounts()[0].GetAccountId() != "lnk_alice_home" {
		t.Fatalf("alice's view changed across bob's link: %d accounts", len(alice.GetAccounts()))
	}
}

// TestM5LinkAccountRejectedNeverVaulted proves the negative half of the link
// contract: a credential the provider rejects yields Unauthenticated and
// nothing is linked or vaulted (PLAN.md §3.5 — the core never stores material
// it has not confirmed works).
func TestM5LinkAccountRejectedNeverVaulted(t *testing.T) {
	stack := newM5Stack(t, fakeJellyfinServer(t))
	client := apiv1.NewCoreServiceClient(startWireServer(t, stack))
	bobCtx := authedCtx(t.Context(), stack.bobToken)

	_, err := client.LinkAccount(bobCtx, &apiv1.LinkAccountRequest{
		Provider: "jellyfin",
		BaseUrl:  stack.baseURL,
		AuthMethod: &apiv1.LinkAccountRequest_Password{
			Password: &apiv1.AccountPassword{Username: "rejected-user", Password: []byte("wrong")},
		},
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("rejected link: got %v, want Unauthenticated", status.Code(err))
	}

	bobs, err := client.ListAccounts(bobCtx, &apiv1.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("bob ListAccounts: %v", err)
	}
	if len(bobs.GetAccounts()) != 0 {
		t.Fatalf("bob sees %d accounts after a rejected link, want 0", len(bobs.GetAccounts()))
	}
}

// TestM5RemoveAccountPublishesRevoked proves RemoveAccount end-to-end: the
// owner's revocation event fires, the account leaves the view, and a second
// removal of the same id is a clean NotFound (PLAN.md §7.5).
func TestM5RemoveAccountPublishesRevoked(t *testing.T) {
	stack := newM5Stack(t, fakeJellyfinServer(t))
	client := apiv1.NewCoreServiceClient(startWireServer(t, stack))
	bobCtx := authedCtx(t.Context(), stack.bobToken)

	link, err := client.LinkAccount(bobCtx, &apiv1.LinkAccountRequest{
		Provider: "jellyfin",
		BaseUrl:  stack.baseURL,
		AuthMethod: &apiv1.LinkAccountRequest_Password{
			Password: &apiv1.AccountPassword{Username: "bob.secondserver.user", Password: []byte("pw-secret")},
		},
	})
	if err != nil {
		t.Fatalf("LinkAccount: %v", err)
	}

	evCh := stack.bus.Subscribe("m5-remove-bob", stack.bob.UserID)
	defer stack.bus.Unsubscribe("m5-remove-bob")

	if _, err := client.RemoveAccount(bobCtx, &apiv1.RemoveAccountRequest{AccountId: link.GetAccountId()}); err != nil {
		t.Fatalf("RemoveAccount: %v", err)
	}

	select {
	case env := <-evCh:
		if env.GetType() != corev1.EventType_EVENT_TYPE_ACCOUNT_SESSION_REVOKED {
			t.Fatalf("event type = %v, want account-session-revoked", env.GetType())
		}
		if env.GetAccountSession().GetAccountId() != link.GetAccountId() {
			t.Fatalf("event account = %q, want %q", env.GetAccountSession().GetAccountId(), link.GetAccountId())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the account-session-revoked event")
	}

	bobs, err := client.ListAccounts(bobCtx, &apiv1.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("bob ListAccounts: %v", err)
	}
	if len(bobs.GetAccounts()) != 0 {
		t.Fatalf("bob still sees %d accounts after removing his link", len(bobs.GetAccounts()))
	}

	_, err = client.RemoveAccount(bobCtx, &apiv1.RemoveAccountRequest{AccountId: link.GetAccountId()})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("second RemoveAccount: got %v, want NotFound", status.Code(err))
	}
}
