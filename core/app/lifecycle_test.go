package app

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/scheduler"
)

// Construction starts no periodic work and Start launches it; Close stops it.
// A stack that is never started is the embedder's and the tests' opt-out
// (PLAN.md §10: core lifecycle operations are explicit, never a side effect
// of constructing or hosting).
func TestStackStartRunsPeriodicWorkAndCloseStopsIt(t *testing.T) {
	ctx := context.Background()
	stack, err := New(ctx, "", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer stack.Close()

	var fires atomic.Int64
	stack.Slots().Scheduler.Register(scheduler.Job{
		Name:    "lifecycle-probe",
		Cadence: 20 * time.Millisecond,
		Run: func(context.Context) error {
			fires.Add(1)
			return nil
		},
	})

	// Registering the probe while nothing is started leaves it parked: the
	// scheduler only launches workers from Run.
	time.Sleep(150 * time.Millisecond)
	if got := fires.Load(); got != 0 {
		t.Fatalf("probe fired %d times before Start, want 0", got)
	}

	if err := stack.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool { return fires.Load() > 0 })

	stack.Close()
	time.Sleep(250 * time.Millisecond) // let an in-flight fire land
	settled := fires.Load()
	time.Sleep(150 * time.Millisecond)
	if got := fires.Load(); got != settled {
		t.Fatalf("probe kept firing after Close: %d -> %d, want no growth", settled, got)
	}
}

// Start is one-shot: a second call is an error, never a silent second run of
// the periodic loops; and a closed stack cannot be restarted.
func TestStackStartIsOneShotAndRefusedAfterClose(t *testing.T) {
	ctx := context.Background()
	stack, err := New(ctx, "", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := stack.Start(ctx); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if err := stack.Start(ctx); err == nil {
		t.Fatal("a second Start must be refused, not double-run the loops")
	}
	stack.Close()
	if err := stack.Start(ctx); err == nil {
		t.Fatal("Start after Close must be refused")
	}
}

// Start also launches the delivery watchdog (PLAN.md §9.1): a play session
// that never heartbeats is swept once its configured liveness window passes,
// and its job is recorded FAILED.
func TestStackStartWatchdogSweepsStalledSessions(t *testing.T) {
	stack, srv, fake := newLifecycleDeliveryStack(t)
	t.Cleanup(stack.Close)
	ctx := apiserver.WithUserID(context.Background(), "u1")
	if err := stack.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	id := linkForPlay(t, srv, fake, ctx)
	jobID := startPlay(t, srv, ctx, id)

	waitFor(t, 5*time.Second, func() bool {
		resp, err := srv.GetJob(ctx, &apiv1.GetJobRequest{JobId: jobID})
		return err == nil && resp.GetJob().GetStatus() == corev1.JobStatus_JOB_STATUS_FAILED
	})
}

// The opt-out twin: without Start there is no watchdog, so the same stalled
// session is still RUNNING well past its liveness window.
func TestStackWithoutStartLeavesSessionsUnswept(t *testing.T) {
	stack, srv, fake := newLifecycleDeliveryStack(t)
	t.Cleanup(stack.Close)
	ctx := apiserver.WithUserID(context.Background(), "u1")
	id := linkForPlay(t, srv, fake, ctx)
	jobID := startPlay(t, srv, ctx, id)

	time.Sleep(2500 * time.Millisecond) // more than two watchdog ticks at ttl=1s
	resp, err := srv.GetJob(ctx, &apiv1.GetJobRequest{JobId: jobID})
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got := resp.GetJob().GetStatus(); got != corev1.JobStatus_JOB_STATUS_RUNNING {
		t.Fatalf("session status without Start = %v, want RUNNING (nothing periodic runs)", got)
	}
}

// newLifecycleDeliveryStack composes a stack whose delivery timing makes the
// watchdog observable in a test: a 1s zombie cap and a 200ms liveness window.
func newLifecycleDeliveryStack(t *testing.T) (*Stack, *apiserver.Server, *fakeJellyfin) {
	t.Helper()
	fake := newFakeJellyfin(t)
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
core:
  api:
    bind: "127.0.0.1:0"
delivery:
  session-ttl: "1s"
  heartbeat:
    interval: "100ms"
    grace: "100ms"
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
	t.Setenv("JF_TEST_PASSWORD", "operator-password-not-real")

	stack, err := New(context.Background(), configPath, slog.Default())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	srv, ok := stack.Service().(*apiserver.Server)
	if !ok {
		t.Fatal("service is not a concrete *apiserver.Server")
	}
	return stack, srv, fake
}

// linkForPlay links a private account for u1 and returns its id.
func linkForPlay(t *testing.T, srv *apiserver.Server, fake *fakeJellyfin, ctx context.Context) string {
	t.Helper()
	private := apiv1.AccountVisibility_ACCOUNT_VISIBILITY_PRIVATE
	res, err := srv.LinkAccount(ctx, &apiv1.LinkAccountRequest{
		Provider:   "jellyfin",
		BaseUrl:    fake.URL(),
		AuthMethod: &apiv1.LinkAccountRequest_Password{Password: &apiv1.AccountPassword{Username: "u1link", Password: []byte("sekret")}},
		Visibility: &private,
	})
	if err != nil {
		t.Fatalf("LinkAccount: %v", err)
	}
	return res.GetAccountId()
}

// startPlay starts a play of the fake's only item and returns the job id.
func startPlay(t *testing.T, srv *apiserver.Server, ctx context.Context, accountID string) string {
	t.Helper()
	res, err := srv.StartDelivery(ctx, &apiv1.StartDeliveryRequest{
		Goal:      apiv1.DeliveryGoal_DELIVERY_GOAL_PLAY,
		Provider:  "jellyfin",
		AccountId: accountID,
		NativeId:  "movie-1",
		Sink:      "device",
	})
	if err != nil {
		t.Fatalf("StartDelivery: %v", err)
	}
	return res.GetJob().GetId()
}

// waitFor polls ok until it holds or the deadline passes.
func waitFor(t *testing.T, deadline time.Duration, ok func() bool) {
	t.Helper()
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before deadline")
}
