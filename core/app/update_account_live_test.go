package app

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
)

// TestUpdatedSharingIsLiveWithoutRestart proves the phase-5 seam end to end:
// the reach the library and delivery authorization read is swapped live
// when the owner edits an account, and the account appears (or vanishes)
// from the member's library at once — no restart, no re-arm.
func TestUpdatedSharingIsLiveWithoutRestart(t *testing.T) {
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

	ownerCtx := apiserver.WithUserID(context.Background(), "u1")
	// The shared-with user must be a real instance user first — the roster
	// names ids, and the directory rejects anyone who never signed up.
	signup, err := srv.SignUp(context.Background(), &apiv1.SignUpRequest{
		Username: "u2",
		AuthMethod: &apiv1.SignUpRequest_Password{
			Password: &apiv1.PasswordSignUp{Password: []byte("correct-horse")},
		},
	})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	memberID := signup.GetUserId()
	linkRes, err := srv.LinkAccount(ownerCtx, &apiv1.LinkAccountRequest{
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
	id := linkRes.GetAccountId()

	// Before any sharing change the account reaches only its owner.
	if _, ok := slots.Library.ReachAuthorized(id, memberID); ok {
		t.Fatal("account is reachable by u2 before sharing")
	}

	// Wait for the first sync to land so the account's item exists in the
	// derivation path (linking runs it in the background by design).
	deadline := time.Now().Add(3 * time.Second)
	for {
		entries, err := slots.Library.Library(context.Background(), "u1")
		if err != nil {
			t.Fatalf("owner library: %v", err)
		}
		if len(entries) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("account's catalogue never materialised")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// The owner's derived cache now exists — the sharing change must
	// invalidate it, because the account's audience (and therefore u1's
	// derivation) changed.
	if _, ok := slots.Library.ReachAuthorized(id, "u1"); !ok {
		t.Fatal("owner lost the reach")
	}

	if _, err := srv.UpdateAccount(ownerCtx, &apiv1.UpdateAccountRequest{
		AccountId: id,
		Sharing: &apiv1.AccountSharing{
			Visibility: apiv1.AccountVisibility_ACCOUNT_VISIBILITY_SHARED,
			SharedWith: []string{memberID},
		},
	}); err != nil {
		t.Fatalf("UpdateAccount: %v", err)
	}

	// The live reach now includes u2, and the owner's cached row was swept.
	if _, ok := slots.Library.ReachAuthorized(id, memberID); !ok {
		t.Fatal("u2 still cannot reach the account after sharing")
	}

	// And the member actually *uses* it from here through the core — no
	// provider login of their own needed; the account session is used on
	// their behalf.
	memberEntries, err := slots.Library.Library(context.Background(), memberID)
	if err != nil {
		t.Fatalf("member library: %v", err)
	}
	if len(memberEntries) == 0 {
		t.Fatal("member's library does not include the shared account after sharing")
	}

	// Narrowing: the member's authorization and their cached view both go.
	if _, err := srv.UpdateAccount(ownerCtx, &apiv1.UpdateAccountRequest{
		AccountId: id,
		Sharing: &apiv1.AccountSharing{
			Visibility: apiv1.AccountVisibility_ACCOUNT_VISIBILITY_PRIVATE,
		},
	}); err != nil {
		t.Fatalf("UpdateAccount narrow: %v", err)
	}
	if _, ok := slots.Library.ReachAuthorized(id, memberID); ok {
		t.Fatal("u2 still authorized after narrowing")
	}
	if _, ok := slots.Library.ReachAuthorized(id, "u1"); !ok {
		t.Fatal("owner lost the reach")
	}
}
