package m0_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/policy"
	"github.com/nem-git/abcmovies/core/internal/store"
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

	resp, err := stack.server.GetJob(apiserver.WithUserID(t.Context(), "user:alice"), &apiv1.GetJobRequest{JobId: "job-storage-1"})
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
	aliceMeta := metadata.NewIncomingContext(t.Context(), metadata.Pairs("authorization", apiserver.BearerPrefix+aliceToken))
	_, err := interceptor(aliceMeta, nil, &grpc.UnaryServerInfo{FullMethod: "/abcmovies.api.v1.CoreService/GetJob"}, aliceHandler)
	if err != nil {
		t.Fatalf("interceptor alice: %v", err)
	}

	var bobCtx context.Context
	bobHandler := func(ctx context.Context, req any) (any, error) {
		bobCtx = ctx
		return "ok", nil
	}
	bobMeta := metadata.NewIncomingContext(t.Context(), metadata.Pairs("authorization", apiserver.BearerPrefix+bobToken))
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

// TestStorage_Jobs_RecoverAfterCrash proves the jobs storage class is
// checkpointed and loss-recoverable (PLAN.md §2.4): a job created before a
// simulated crash is still queryable after the process restarts over the
// same SQLite file. The job's full state — id, kind, status, owner,
// delivery context, progress — must round-trip intact.
func TestStorage_Jobs_RecoverAfterCrash(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "jobs.db")

	// --- Phase 1: create the job, then "crash" ---

	cfg1 := config.Default()
	cfg1.Stores.Jobs = config.StoreConfig{Backend: "local-file", Path: dbPath}

	stores1, err := config.BuildStores(ctx, cfg1, nil)
	if err != nil {
		t.Fatalf("BuildStores: %v", err)
	}
	stores1.WatchHistory = store.NewUserBlobStore(stores1.WatchHistory)

	users1, tokens1, deks1, err := config.BuildAuth(stores1.Users, stores1.Sessions, cfg1.Auth.DEKCache, nil)
	if err != nil {
		t.Fatalf("BuildAuth: %v", err)
	}
	composite1, err := config.BuildAuthenticator([]string{"password"}, users1)
	if err != nil {
		t.Fatalf("BuildAuthenticator: %v", err)
	}
	session1 := config.BuildSession(tokens1, deks1, time.Hour)

	bus1 := apiserver.NewInMemoryBus()
	defer bus1.Close()

	srv1 := apiserver.NewServer(apiserver.Settings{Bus: bus1, Stores: stores1, Authenticator: composite1, Session: session1})

	job := &corev1.Job{
		Id:          "job-recovery-1",
		Kind:        corev1.JobKind_JOB_KIND_DELIVERY,
		Status:      corev1.JobStatus_JOB_STATUS_RUNNING,
		OwnerUserId: "user:alice",
		Progress: &corev1.JobProgress{
			Percent: 42,
			Message: "downloading",
		},
		Delivery: &corev1.DeliveryContext{
			Provider:     "jellyfin",
			AccountId:    "acct-1",
			MemberUserId: "user:alice",
			Policy: &corev1.Policy{
				Limits: map[string]string{policy.KeyConcurrentStreams: "2"},
			},
			ProviderCap: &corev1.Policy{
				Limits: map[string]string{policy.KeyConcurrentStreams: "3"},
			},
		},
	}
	if err := srv1.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	// Simulate crash: close all stores without any graceful shutdown.
	if err := config.CloseStores(stores1); err != nil {
		t.Fatalf("CloseStores: %v", err)
	}

	// --- Phase 2: restart over the same SQLite file ---

	cfg2 := config.Default()
	cfg2.Stores.Jobs = config.StoreConfig{Backend: "local-file", Path: dbPath}

	stores2, err := config.BuildStores(ctx, cfg2, nil)
	if err != nil {
		t.Fatalf("BuildStores (recovery): %v", err)
	}
	stores2.WatchHistory = store.NewUserBlobStore(stores2.WatchHistory)

	users2, tokens2, deks2, err := config.BuildAuth(stores2.Users, stores2.Sessions, cfg2.Auth.DEKCache, nil)
	if err != nil {
		t.Fatalf("BuildAuth (recovery): %v", err)
	}
	composite2, err := config.BuildAuthenticator([]string{"password"}, users2)
	if err != nil {
		t.Fatalf("BuildAuthenticator (recovery): %v", err)
	}
	session2 := config.BuildSession(tokens2, deks2, time.Hour)

	bus2 := apiserver.NewInMemoryBus()
	defer bus2.Close()

	srv2 := apiserver.NewServer(apiserver.Settings{Bus: bus2, Stores: stores2, Authenticator: composite2, Session: session2})

	// The job must be fully intact after recovery.
	resp, err := srv2.GetJob(apiserver.WithUserID(ctx, "user:alice"), &apiv1.GetJobRequest{JobId: "job-recovery-1"})
	if err != nil {
		t.Fatalf("GetJob after recovery: %v", err)
	}

	got := resp.GetJob()
	if got.GetId() != "job-recovery-1" {
		t.Errorf("Job ID = %q, want job-recovery-1", got.GetId())
	}
	if got.GetKind() != corev1.JobKind_JOB_KIND_DELIVERY {
		t.Errorf("Job Kind = %v, want JOB_KIND_DELIVERY", got.GetKind())
	}
	if got.GetStatus() != corev1.JobStatus_JOB_STATUS_RUNNING {
		t.Errorf("Job Status = %v, want JOB_STATUS_RUNNING", got.GetStatus())
	}
	if got.GetOwnerUserId() != "user:alice" {
		t.Errorf("OwnerUserId = %q, want user:alice", got.GetOwnerUserId())
	}
	if got.GetProgress().GetPercent() != 42 {
		t.Errorf("Progress.Percent = %d, want 42", got.GetProgress().GetPercent())
	}
	if got.GetDelivery().GetProvider() != "jellyfin" {
		t.Errorf("Provider = %q, want jellyfin", got.GetDelivery().GetProvider())
	}
	if got.GetDelivery().GetAccountId() != "acct-1" {
		t.Errorf("AccountId = %q, want acct-1", got.GetDelivery().GetAccountId())
	}
	if got.GetDelivery().GetMemberUserId() != "user:alice" {
		t.Errorf("MemberUserId = %q, want user:alice", got.GetDelivery().GetMemberUserId())
	}
	if got.GetDelivery().GetPolicy().GetLimits()["concurrentStreams"] != "2" {
		t.Errorf("Policy limit = %q, want 2", got.GetDelivery().GetPolicy().GetLimits()["concurrentStreams"])
	}
}
