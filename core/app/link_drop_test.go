package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/slotwiring"
)

// writeConfigFile writes a minimal instance config with a single operator
// jellyfin slot pointed at fakeURL.
func writeLinkedJFConfig(t *testing.T, fakeURL string) string {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
core:
  api:
    bind: "127.0.0.1:0"
slots:
  providers:
    - adapter: jellyfin
      id: jf
      enabled: true
      server: "`+fakeURL+`"
      accounts:
        - id: op
          username: "alice.homeserver.user"
          password-env: JF_TEST_PASSWORD
  sinks:
    - adapter: device
      id: device
      enabled: true
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}

// TestUnlinkedAccountLeavesRunningSlot pins the removal half of the runtime
// account seam: unlink must take the account out of the live slot (cached
// session, refresh job, source-cache rows), not just hide it from the
// permission views. The running slot must never keep serving an unknown
// account.
func TestUnlinkedAccountLeavesRunningSlot(t *testing.T) {
	fake := newFakeJellyfin(t)
	configPath := writeLinkedJFConfig(t, fake.URL())
	t.Setenv("JF_TEST_PASSWORD", "operator-password-not-real")

	stack, err := Build(configPath, slog.Default())
	if err != nil {
		t.Fatalf("stack: %v", err)
	}
	slots, err := stack.BuildSlots(context.Background(), slog.Default())
	if err != nil {
		t.Fatalf("BuildSlots: %v", err)
	}
	srv, ok := stack.Service().(*apiserver.Server)
	if !ok {
		t.Fatalf("service is not a concrete *apiserver.Server")
	}

	ctx := apiserver.WithUserID(context.Background(), "u1")
	linkRes, err := srv.LinkAccount(ctx, &apiv1.LinkAccountRequest{
		Provider: "jellyfin",
		BaseUrl:  fake.URL(),
		AuthMethod: &apiv1.LinkAccountRequest_Password{
			Password: &apiv1.AccountPassword{Username: "u1link", Password: []byte("pw")},
		},
		Visibility: apiv1.AccountVisibility_ACCOUNT_VISIBILITY_PRIVATE,
	})
	if err != nil {
		t.Fatalf("LinkAccount: %v", err)
	}

	if _, err := srv.RemoveAccount(ctx, &apiv1.RemoveAccountRequest{AccountId: linkRes.GetAccountId()}); err != nil {
		t.Fatalf("RemoveAccount: %v", err)
	}
	if _, ok := slots.Library.ReachAuthorized(linkRes.GetAccountId(), "u1"); ok {
		t.Fatal("reach still authorized after unlink")
	}
	// The ghost must not linger: the running slot no longer serves the id.
	if len(slots.Providers) == 0 {
		t.Fatal("no built providers")
	}
	att, ok := slots.Providers[0].Impl.(slotwiring.AttachableSlot)
	if !ok {
		t.Fatalf("provider %q not attachable", slots.Providers[0].Entry.ID)
	}
	if _, err := att.CatalogueSync(context.Background(), &slotsv1.CatalogueSyncRequest{AccountId: linkRes.GetAccountId()}); err == nil || !strings.Contains(err.Error(), "unknown account") {
		t.Fatalf("CatalogueSync after unlink = %v, want unknown account", err)
	}
}

// TestLastAccountOfUserSlotRetiresIt pins the retirement rule: a slot that
// exists only because of linked accounts (a user-provided server slot, with
// no operator accounts of its own) is retired when its last account is
// unlinked — from the build output, the delivery resolvers, and the
// registry. Operator-declared slots survive an unlink, because their config
// references real servers regardless of accounts.
func TestLastAccountOfUserSlotRetiresIt(t *testing.T) {
	fakeA := newFakeJellyfin(t)
	fakeB := newFakeJellyfin(t)
	configPath := writeLinkedJFConfig(t, fakeA.URL())
	t.Setenv("JF_TEST_PASSWORD", "operator-password-not-real")

	stack, err := Build(configPath, slog.Default())
	if err != nil {
		t.Fatalf("stack: %v", err)
	}
	// Seed a linked account on a DIFFERENT server than the operator slot's,
	// before composition, so boot provisions a user-owned (synthetic) slot for
	// it rather than routing it to the configured operator slot.
	r1 := accounts.Record{ID: accounts.NewID(), Provider: "jellyfin", BaseURL: fakeB.URL(), Username: "u1vf", OwnerUserID: "u1", Visibility: accounts.VisibilityPrivate}
	linkedStore := accounts.NewStore(stack.stores.Vault, slog.Default())
	if err := linkedStore.Add(context.Background(), r1); err != nil {
		t.Fatalf("seed linked record: %v", err)
	}
	if err := linkedStore.Save(context.Background(), r1.ID, []byte(fmt.Sprintf(`{"AccessToken":%q,"User":{"Id":"u1"}}`, "faker-token"))); err != nil {
		t.Fatalf("seed linked session: %v", err)
	}

	slots, err := stack.BuildSlots(context.Background(), slog.Default())
	if err != nil {
		t.Fatalf("BuildSlots: %v", err)
	}
	srv, ok := stack.Service().(*apiserver.Server)
	if !ok {
		t.Fatalf("service is not a concrete *apiserver.Server")
	}
	// The synthetic user-owned slot exists at boot from the seeded record.
	if len(slots.Providers) != 2 {
		t.Fatalf("providers = %d, want operator slot + one provisioned user slot", len(slots.Providers))
	}
	ns := slotwiring.ServerNamespace(r1)
	if _, ok := slots.Resolvers[ns]; !ok {
		t.Fatalf("synthetic slot %q has no resolver entry", ns)
	}

	// Unlink it via the real service: the slot side (cached session, job,
	// rows) and the retirement of the now-empty synthetic slot must follow.
	if _, err := srv.RemoveAccount(apiserver.WithUserID(context.Background(), "u1"), &apiv1.RemoveAccountRequest{AccountId: r1.ID}); err != nil {
		t.Fatalf("RemoveAccount: %v", err)
	}
	if len(slots.Providers) != 1 {
		t.Fatalf("providers = %d, want only the operator slot", len(slots.Providers))
	}
	if slots.Providers[0].Entry.ID != "jf" {
		t.Fatalf("surviving provider = %q, want jf", slots.Providers[0].Entry.ID)
	}
	if _, ok := slots.Resolvers[ns]; ok {
		t.Fatalf("resolver for retired slot %q still present", ns)
	}
	snap := slots.deps.Registry.Snapshot()
	if _, gone := snap[ns]; gone {
		t.Fatalf("retired slot %q still in registry", ns)
	}
	if _, ok := snap["jf"]; !ok {
		t.Fatal("operator slot jf was retired by an unlink")
	}
}
