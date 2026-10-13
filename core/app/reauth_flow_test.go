package app

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nem-git/abcmovies/adapters/jellyfin"
	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/delivery"
)

// The structural seam between the adapter's typed failure and the core's
// probe must hold at the composition boundary: the adapter marks its error,
// the core recognizes it, and neither side imports the other's type.
func TestJellyfinNoSessionErrorIsRecognizedCoreSide(t *testing.T) {
	if !delivery.IsNoSession(jellyfin.NoSessionError{AccountID: "lnk_x"}) {
		t.Fatal("jellyfin.NoSessionError must satisfy the core's no-session probe")
	}
}

// TestSessionExpiredReauthFlowEndToEnd drives the whole re-auth flow
// (PLAN.md §7.5) through the composed stack: a linked account's session
// dies at the provider, the delivery that discovers it flips the account to
// EXPIRED, notifies the owner, and opens a re-auth job; the owner's re-link
// revives the same account — same id, no duplicate — and delivery works
// again on the freshly probed session.
func TestSessionExpiredReauthFlowEndToEnd(t *testing.T) {
	fake := newFakeJellyfin(t)
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
core:
  api:
    bind: "127.0.0.1:0"
stores:
  slots:
    backend: in-memory
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("JF_TEST_PASSWORD", "operator-password-not-real")

	stack, err := New(context.Background(), configPath, slog.Default())
	if err != nil {
		t.Fatalf("stack: %v", err)
	}
	seedJellyfinSlots(t, stack, "jellyfin", fake.URL())
	srv, ok := stack.Service().(*apiserver.Server)
	if !ok {
		t.Fatal("service is not a concrete *apiserver.Server")
	}

	ctx := apiserver.WithUserID(context.Background(), "u1")
	private := apiv1.AccountVisibility_ACCOUNT_VISIBILITY_PRIVATE
	linkRes, err := srv.LinkAccount(ctx, &apiv1.LinkAccountRequest{
		Provider:   "jellyfin",
		BaseUrl:    fake.URL(),
		AuthMethod: &apiv1.LinkAccountRequest_Password{Password: &apiv1.AccountPassword{Username: "u1link", Password: []byte("sekret")}},
		Visibility: &private,
	})
	if err != nil {
		t.Fatalf("LinkAccount: %v", err)
	}
	id := linkRes.GetAccountId()

	// Subscribe before the discovery: the bus keeps no history
	// (PLAN.md §9.2, at-most-once), and sync noise may interleave — drain
	// until the expiry event arrives.
	evCh := stack.bus.Subscribe("reauth-e2e", "u1")
	defer stack.bus.Unsubscribe("reauth-e2e")

	// Kill the session at the provider: the cached token is rejected, and
	// the adapter blanks its vaulted copy on rejection. A linked account has
	// no password-env fallback, so the retry discovers a dead session.
	fake.playback401 = true
	if _, err := srv.StartDelivery(ctx, &apiv1.StartDeliveryRequest{
		Goal:      apiv1.DeliveryGoal_DELIVERY_GOAL_PLAY,
		Provider:  "jellyfin",
		AccountId: id,
		NativeId:  "movie-1",
		Sink:      "device",
	}); err == nil {
		t.Fatal("a play against a dead session must fail")
	}

	// The re-auth flow ran: the account is EXPIRED and the owner was
	// notified.
	st := accounts.NewStore(stack.stores.Vault, nil)
	rec, err := st.Get(ctx, id)
	if err != nil {
		t.Fatalf("account record: %v", err)
	}
	if rec.Status != accounts.StatusExpired {
		t.Fatalf("account status = %v, want EXPIRED after the discovery", rec.Status)
	}
	deadline := time.After(3 * time.Second)
	seenExpiry := false
	for !seenExpiry {
		select {
		case env := <-evCh:
			if env.GetType() != corev1.EventType_EVENT_TYPE_ACCOUNT_SESSION_EXPIRED {
				continue
			}
			if env.GetAccountSession().GetAccountId() != id {
				t.Fatalf("event account = %q, want %q", env.GetAccountSession().GetAccountId(), id)
			}
			seenExpiry = true
		case <-deadline:
			t.Fatal("no ACCOUNT_SESSION_EXPIRED event reached the owner")
		}
	}

	// A re-auth job waits for the owner: AWAITING_ACTION, re-link is the
	// only act, and the owner is the only actor.
	jobResp, err := srv.GetJob(ctx, &apiv1.GetJobRequest{JobId: "reauth-" + id})
	if err != nil {
		t.Fatalf("re-auth job: %v", err)
	}
	job := jobResp.GetJob()
	if job.GetKind() != corev1.JobKind_JOB_KIND_RE_AUTH {
		t.Fatalf("job kind = %v, want RE_AUTH", job.GetKind())
	}
	if job.GetStatus() != corev1.JobStatus_JOB_STATUS_AWAITING_ACTION {
		t.Fatalf("job status = %v, want AWAITING_ACTION", job.GetStatus())
	}
	if job.GetAwaitingAction().GetCommon() != corev1.ActionType_ACTION_TYPE_RELINK {
		t.Fatalf("awaiting action = %v, want RELINK", job.GetAwaitingAction().GetCommon())
	}
	if job.GetAwaitingAction().GetActorUserId() != "u1" {
		t.Fatalf("actor = %q, want the owner u1", job.GetAwaitingAction().GetActorUserId())
	}

	// The owner re-links: the same account returns to LINKED — same id, no
	// duplicate record — with a freshly probed session.
	fake.playback401 = false
	relink, err := srv.LinkAccount(ctx, &apiv1.LinkAccountRequest{
		Provider:   "jellyfin",
		BaseUrl:    fake.URL(),
		AuthMethod: &apiv1.LinkAccountRequest_Password{Password: &apiv1.AccountPassword{Username: "u1link", Password: []byte("rotated")}},
	})
	if err != nil {
		t.Fatalf("re-link: %v", err)
	}
	if relink.GetAccountId() != id {
		t.Fatalf("re-link returned %q, want the same account %q", relink.GetAccountId(), id)
	}
	rec, err = st.Get(ctx, id)
	if err != nil {
		t.Fatalf("account record after re-link: %v", err)
	}
	if rec.Status != accounts.StatusLinked {
		t.Fatalf("account status after re-link = %v, want LINKED", rec.Status)
	}

	// And the revived session delivers: the same play now succeeds. The
	// provider's shared governor — deliberately one throttled account
	// throttles all (PLAN.md §5.4) — carries a short backoff seeded by the
	// dead-session failures, so retry until it expires; a refused start
	// costs nothing and never grows the backoff (pacing fails fast, without
	// a note).
	playDeadline := time.Now().Add(10 * time.Second)
	for {
		_, err = srv.StartDelivery(ctx, &apiv1.StartDeliveryRequest{
			Goal:      apiv1.DeliveryGoal_DELIVERY_GOAL_PLAY,
			Provider:  "jellyfin",
			AccountId: id,
			NativeId:  "movie-1",
			Sink:      "device",
		})
		if err == nil {
			break
		}
		if time.Now().After(playDeadline) {
			t.Fatalf("play after re-link: %v", err)
		}
		time.Sleep(250 * time.Millisecond)
	}
}
