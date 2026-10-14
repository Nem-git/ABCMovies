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

// A download that queues through the composed stack promotes when a seat
// frees: the job transitions QUEUED → RUNNING on the bus, and no new event
// type is needed (PLAN.md §6.5, §9.1).
func TestQueuedDownloadPromotesWhenSeatFrees(t *testing.T) {
	fake := newFakeJellyfin(t)
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
core:
  api:
    bind: "127.0.0.1:0"
stores:
  slots:
    backend: in-memory
policy:
  concurrentStreams: 1
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("JF_TEST_PASSWORD", "sekret")

	stack, err := New(context.Background(), configPath, slog.Default())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer stack.Close()
	seedJellyfinSlots(t, stack, "jellyfin", fake.URL())

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

	// Subscribe before the queue forms: the bus keeps no history (§9.2).
	evCh := stack.bus.Subscribe("queue-e2e", "u1")
	defer stack.bus.Unsubscribe("queue-e2e")

	// The harness policy allows one concurrent stream. The first download
	// starts; the second queues.
	start := func() *apiv1.StartDeliveryResponse {
		res, err := srv.StartDelivery(ctx, &apiv1.StartDeliveryRequest{
			Goal:      apiv1.DeliveryGoal_DELIVERY_GOAL_DOWNLOAD,
			Provider:  "jellyfin",
			AccountId: link.GetAccountId(),
			NativeId:  "movie-1",
			Sink:      "relay",
		})
		if err != nil {
			t.Fatalf("StartDelivery: %v", err)
		}
		return res
	}
	first := start()
	second := start()

	if first.GetJob().GetStatus() != corev1.JobStatus_JOB_STATUS_RUNNING {
		t.Fatalf("first download status = %v, want RUNNING", first.GetJob().GetStatus())
	}
	if second.GetJob().GetStatus() != corev1.JobStatus_JOB_STATUS_QUEUED {
		t.Fatalf("second download status = %v, want QUEUED", second.GetJob().GetStatus())
	}
	if pos := second.GetJob().GetQueued().GetPosition(); pos != 1 {
		t.Errorf("queued position = %d, want 1", pos)
	}
	queuedID := second.GetJob().GetId()

	// Complete the first download through the engine; the seat-release hook
	// promotes the queued one. Drain until the promotion's RUNNING event
	// arrives.
	if err := stack.delivery.Complete(context.Background(), first.GetJob().GetId()); err != nil {
		t.Fatalf("complete: %v", err)
	}
	deadline := time.After(3 * time.Second)
	for {
		select {
		case env := <-evCh:
			if env.GetType() != corev1.EventType_EVENT_TYPE_JOB_STATUS {
				continue
			}
			if env.GetJobStatus().GetJobId() == queuedID && env.GetJobStatus().GetStatus() == corev1.JobStatus_JOB_STATUS_RUNNING {
				return
			}
		case <-deadline:
			t.Fatal("queued download did not promote to RUNNING on the bus")
		}
	}
}
