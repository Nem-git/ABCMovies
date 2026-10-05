package app

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/slotwiring"
)

// TestUnlinkedAccountLeavesRunningSlot pins the removal half of the runtime
// account seam: unlink must take the account out of the live slot (cached
// session, refresh job, source-cache rows), not just hide it from the
// permission views. The running slot must never keep serving an unknown
// account.
func TestUnlinkedAccountLeavesRunningSlot(t *testing.T) {
	fake := newFakeJellyfin(t)
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
      server: "`+fake.URL()+`"
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
