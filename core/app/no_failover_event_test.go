package app

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
)

// A failed delivery never publishes a provider-switched event. The engine has
// no failover path (PLAN.md §6.5): it fails honestly and the re-auth flow
// runs, so the composed stack announces the failure — never a switch that did
// not happen. This is the end-to-end lock on "failover is the user's choice":
// nothing on the wire advertises a provider switch.
func TestFailedPlayPublishesNoProviderSwitched(t *testing.T) {
	fake := newFakeJellyfin(t)
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
core:
  api:
    bind: "127.0.0.1:0"
slots:
  providers:
    - adapter: jellyfin
      id: jellyfin
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
	t.Setenv("JF_TEST_PASSWORD", "sekret")

	stack, err := New(context.Background(), configPath, slog.Default())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer stack.Close()

	srv, ok := stack.Service().(*apiserver.Server)
	if !ok {
		t.Fatal("service is not a concrete *apiserver.Server")
	}

	ctx := apiserver.WithUserID(context.Background(), "u1")
	private := apiv1.AccountVisibility_ACCOUNT_VISIBILITY_PRIVATE
	link, err := srv.LinkAccount(ctx, &apiv1.LinkAccountRequest{
		Provider:   "jellyfin",
		BaseUrl:    fake.URL(),
		AuthMethod: &apiv1.LinkAccountRequest_Password{Password: &apiv1.AccountPassword{Username: "u1", Password: []byte("pw")}},
		Visibility: &private,
	})
	if err != nil {
		t.Fatalf("LinkAccount: %v", err)
	}

	// Subscribe as the owner before the failing play: the bus keeps no
	// history (PLAN.md §9.2), so a switch published during the failure would
	// only be caught by a listener already attached.
	evCh := stack.bus.Subscribe("no-failover", "u1")
	defer stack.bus.Unsubscribe("no-failover")

	// The provider rejects the play: the delivery must fail rather than
	// reach for another provider.
	fake.playback401 = true
	if _, err := srv.StartDelivery(ctx, &apiv1.StartDeliveryRequest{
		Goal:      apiv1.DeliveryGoal_DELIVERY_GOAL_PLAY,
		Provider:  "jellyfin",
		AccountId: link.GetAccountId(),
		NativeId:  "movie-1",
		Sink:      "device",
	}); err == nil {
		t.Fatal("a play against a dead session must fail, never switch providers")
	}

	// The failure is announced (the re-auth flow fires, PLAN.md §7.5), which
	// proves the delivery path published its real events — then assert none
	// of them is a switch.
	sawFailure := false
	deadline := time.After(3 * time.Second)
	for !sawFailure {
		select {
		case env := <-evCh:
			if env.GetType() == corev1.EventType_EVENT_TYPE_PROVIDER_SWITCHED {
				t.Fatalf("a failed play published PROVIDER_SWITCHED: %+v", env.GetProviderSwitched())
			}
			if env.GetType() == corev1.EventType_EVENT_TYPE_ACCOUNT_SESSION_EXPIRED {
				sawFailure = true
			}
		case <-deadline:
			t.Fatal("no failure event observed; cannot prove the delivery path ran")
		}
	}
	// Drain whatever else is queued (job status and the like), still
	// refusing a switch event.
	for {
		select {
		case env := <-evCh:
			if env.GetType() == corev1.EventType_EVENT_TYPE_PROVIDER_SWITCHED {
				t.Fatalf("a failed play published PROVIDER_SWITCHED: %+v", env.GetProviderSwitched())
			}
		default:
			return
		}
	}
}
