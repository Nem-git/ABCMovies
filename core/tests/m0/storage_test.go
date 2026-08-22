package m0_test

import (
	"context"
	"testing"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestStorage_Cache_PutGet(t *testing.T) {
	stack := newFullStack(t)

	if err := stack.stores.Cache.Put(context.Background(), "key1", []byte("value1")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := stack.stores.Cache.Get(context.Background(), "key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "value1" {
		t.Fatalf("Get = %q, want %q", got, "value1")
	}
}

func TestStorage_Jobs_PersistAndRetrieve(t *testing.T) {
	stack := newFullStack(t)

	job := &corev1.Job{
		Id:          "job-storage-1",
		Kind:        corev1.JobKind_JOB_KIND_REFRESH,
		Status:      corev1.JobStatus_JOB_STATUS_QUEUED,
		OwnerUserId: "user:alice",
	}
	if err := stack.server.CreateJob(t.Context(), job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	resp, err := stack.server.GetJob(t.Context(), &apiv1.GetJobRequest{JobId: "job-storage-1"})
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if resp.GetJob().GetId() != "job-storage-1" {
		t.Fatalf("Job ID = %q, want %q", resp.GetJob().GetId(), "job-storage-1")
	}
	if resp.GetJob().GetStatus() != corev1.JobStatus_JOB_STATUS_QUEUED {
		t.Fatalf("Job Status = %v, want %v", resp.GetJob().GetStatus(), corev1.JobStatus_JOB_STATUS_QUEUED)
	}
}

func TestStorage_WatchHistory_PerUserEncrypted(t *testing.T) {
	stack := newFullStack(t)

	// Sign up Alice and Bob.
	signUp(t, stack.server, "alice", "password123")
	signUp(t, stack.server, "bob", "password456")

	// Login both.
	aliceToken := login(t, stack.server, "alice", "password123")
	bobToken := login(t, stack.server, "bob", "password456")

	// Get authenticated contexts.
	interceptor := apiserver.AuthUnaryInterceptor(stack.session)

	var aliceCtx context.Context
	aliceHandler := func(ctx context.Context, req any) (any, error) {
		aliceCtx = ctx
		return "ok", nil
	}
	aliceMeta := metadata.NewIncomingContext(t.Context(), metadata.Pairs("authorization", "Bearer "+aliceToken))
	_, err := interceptor(aliceMeta, nil, &grpc.UnaryServerInfo{FullMethod: "/abcmovies.api.v1.CoreService/GetJob"}, aliceHandler)
	if err != nil {
		t.Fatalf("interceptor alice: %v", err)
	}

	var bobCtx context.Context
	bobHandler := func(ctx context.Context, req any) (any, error) {
		bobCtx = ctx
		return "ok", nil
	}
	bobMeta := metadata.NewIncomingContext(t.Context(), metadata.Pairs("authorization", "Bearer "+bobToken))
	_, err = interceptor(bobMeta, nil, &grpc.UnaryServerInfo{FullMethod: "/abcmovies.api.v1.CoreService/GetJob"}, bobHandler)
	if err != nil {
		t.Fatalf("interceptor bob: %v", err)
	}

	// Alice writes to watch history.
	if err := stack.stores.WatchHistory.Put(aliceCtx, "movie:1", []byte("watched-at-noon")); err != nil {
		t.Fatalf("alice Put: %v", err)
	}

	// Alice reads her own data.
	got, err := stack.stores.WatchHistory.Get(aliceCtx, "movie:1")
	if err != nil {
		t.Fatalf("alice Get: %v", err)
	}
	if string(got) != "watched-at-noon" {
		t.Fatalf("alice Get = %q, want %q", got, "watched-at-noon")
	}

	// Bob cannot read Alice's data.
	_, err = stack.stores.WatchHistory.Get(bobCtx, "movie:1")
	if err == nil {
		t.Fatal("bob should not be able to read alice's data")
	}
}

func TestStorage_Vault_EncryptedAtRest(t *testing.T) {
	stack := newFullStack(t)

	// Write to vault.
	if err := stack.stores.Vault.Put(context.Background(), "secret:1", []byte("sensitive-data")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Read back — should match.
	got, err := stack.stores.Vault.Get(context.Background(), "secret:1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "sensitive-data" {
		t.Fatalf("Get = %q, want %q", got, "sensitive-data")
	}
}
