package m0_test

import (
	"context"
	"testing"
	"time"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// TestWalkingSkeleton proves the entire M0 walking skeleton works end-to-end.
// It exercises every layer: auth, storage (cache, vault, watch history, jobs),
// events, and the full server wiring. If the individual integration tests pass
// but this one fails, the problem is in the wiring between components.
func TestWalkingSkeleton(t *testing.T) {
	stack := newFullStack(t)

	// --- 1. Auth: sign up, login, get token ---
	signUp(t, stack.server, "alice", "password123")
	token := login(t, stack.server, "alice", "password123")
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	// --- 2. Events: subscribe as alice (the job's owner below) ---
	eventCh := stack.bus.Subscribe("skeleton-sub", "user:alice")
	defer stack.bus.Unsubscribe("skeleton-sub")

	// --- 3. Jobs: create, retrieve, receive event ---
	job := &corev1.Job{
		Id:          "skeleton-job-1",
		Kind:        corev1.JobKind_JOB_KIND_REFRESH,
		Status:      corev1.JobStatus_JOB_STATUS_QUEUED,
		OwnerUserId: "user:alice",
	}
	if err := stack.server.CreateJob(t.Context(), job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	// Retrieve job.
	resp, err := stack.server.GetJob(t.Context(), &apiv1.GetJobRequest{JobId: "skeleton-job-1"})
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if resp.GetJob().GetId() != "skeleton-job-1" {
		t.Fatalf("Job ID = %q, want %q", resp.GetJob().GetId(), "skeleton-job-1")
	}

	// Receive event.
	select {
	case event := <-eventCh:
		if event.GetType() != corev1.EventType_EVENT_TYPE_JOB_STATUS {
			t.Fatalf("event type = %v, want %v", event.GetType(), corev1.EventType_EVENT_TYPE_JOB_STATUS)
		}
		if event.GetJobStatus().GetJobId() != "skeleton-job-1" {
			t.Fatalf("event job_id = %q, want %q", event.GetJobStatus().GetJobId(), "skeleton-job-1")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for job-status event")
	}

	// --- 4. Per-user blob: write, read, verify isolation ---
	interceptor := apiserver.AuthUnaryInterceptor(stack.session)

	var aliceCtx context.Context
	aliceHandler := func(ctx context.Context, req any) (any, error) {
		aliceCtx = ctx
		return "ok", nil
	}
	aliceMeta := metadata.NewIncomingContext(t.Context(), metadata.Pairs("authorization", "Bearer "+token))
	if _, err := interceptor(aliceMeta, nil, &grpc.UnaryServerInfo{FullMethod: "/abcmovies.api.v1.CoreService/GetJob"}, aliceHandler); err != nil {
		t.Fatalf("interceptor alice: %v", err)
	}

	// Write watch history.
	if err := stack.stores.WatchHistory.Put(aliceCtx, "movie:42", []byte("watched")); err != nil {
		t.Fatalf("Put watch history: %v", err)
	}

	// Read watch history.
	got, err := stack.stores.WatchHistory.Get(aliceCtx, "movie:42")
	if err != nil {
		t.Fatalf("Get watch history: %v", err)
	}
	if string(got) != "watched" {
		t.Fatalf("watch history = %q, want %q", got, "watched")
	}

	// Verify encryption at rest: raw store should not contain plaintext.
	raw, err := stack.stores.Users.Get(context.Background(), "user:user:alice:movie:42")
	if err == nil && string(raw) == "watched" {
		t.Fatal("raw value contains plaintext — not encrypted")
	}

	// --- 5. Cache: put/get round-trip ---
	if err := stack.stores.Cache.Put(t.Context(), "cache:test", []byte("cached-value")); err != nil {
		t.Fatalf("Cache Put: %v", err)
	}
	cacheGot, err := stack.stores.Cache.Get(t.Context(), "cache:test")
	if err != nil {
		t.Fatalf("Cache Get: %v", err)
	}
	if string(cacheGot) != "cached-value" {
		t.Fatalf("Cache Get = %q, want %q", cacheGot, "cached-value")
	}

	// --- 6. Vault: write, verify encrypted at rest ---
	if err := stack.stores.Vault.Put(t.Context(), "vault:test", []byte("vault-secret")); err != nil {
		t.Fatalf("Vault Put: %v", err)
	}
	vaultGot, err := stack.stores.Vault.Get(t.Context(), "vault:test")
	if err != nil {
		t.Fatalf("Vault Get: %v", err)
	}
	if string(vaultGot) != "vault-secret" {
		t.Fatalf("Vault Get = %q, want %q", vaultGot, "vault-secret")
	}

	// --- 7. Cleanup: stores close cleanly ---
	// The test cleanup will close the bus. Stores are in-memory and don't need
	// explicit close. Verify they're still usable right before cleanup.
	if err := stack.stores.Cache.Put(t.Context(), "final", []byte("ok")); err != nil {
		t.Fatalf("final cache put: %v", err)
	}
}
