package apiserver_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/store"
)

func testStores(t *testing.T) config.Stores {
	t.Helper()
	return config.Stores{
		Cache:        store.NewInMemory(),
		Vault:        store.NewInMemory(),
		WatchHistory: store.NewInMemory(),
		Jobs:         store.NewInMemory(),
		Sessions:     store.NewInMemory(),
		Users:        store.NewInMemory(),
	}
}

func testAuth(t *testing.T) (*auth.CompositeAuthenticator, auth.Session) {
	t.Helper()
	userStore := auth.NewMemoryUserStore()
	tokens := auth.NewStoreTokenStore(store.NewInMemory())
	deks := auth.NewStoreDEKCache(store.NewInMemory())
	composite, err := auth.NewAuthenticators([]string{"password"}, userStore)
	if err != nil {
		t.Fatalf("NewAuthenticators: %v", err)
	}
	session := auth.NewSessionHandler(tokens, deks, time.Hour)
	return composite, session
}

func getPasswordAuth(t *testing.T, c *auth.CompositeAuthenticator) *auth.PasswordAuthenticator {
	t.Helper()
	a, ok := c.Get("password")
	if !ok {
		t.Fatal("password authenticator not found in composite")
	}
	return a.(*auth.PasswordAuthenticator)
}

func TestAuthInterceptor_ValidToken(t *testing.T) {
	authenticator, session := testAuth(t)

	// Create a user and get a token.
	pwd := getPasswordAuth(t, authenticator)
	_, err := pwd.SignUp("alice", []byte("password123"))
	if err != nil {
		t.Fatalf("signup: %v", err)
	}
	result, err := pwd.Login("alice", []byte("password123"))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	token, err := session.Mint(result.UserID)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	interceptor := apiserver.AuthUnaryInterceptor(session)
	handler := func(ctx context.Context, req any) (any, error) {
		uid, ok := apiserver.UserIDFromContext(ctx)
		if !ok {
			t.Fatal("user ID not in context")
		}
		if uid != "user:alice" {
			t.Fatalf("got user ID %q, want %q", uid, "user:alice")
		}
		return "ok", nil
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))
	resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/abcmovies.api.v1.CoreService/GetJob"}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Fatalf("unexpected response: %v", resp)
	}
}

func TestAuthInterceptor_MissingToken(t *testing.T) {
	_, session := testAuth(t)
	interceptor := apiserver.AuthUnaryInterceptor(session)
	handler := func(ctx context.Context, req any) (any, error) { return "ok", nil }
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs())
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/abcmovies.api.v1.CoreService/GetJob"}, handler)
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Fatalf("got code %v, want %v", got, codes.Unauthenticated)
	}
}

func TestAuthInterceptor_MalformedToken(t *testing.T) {
	_, session := testAuth(t)
	interceptor := apiserver.AuthUnaryInterceptor(session)
	handler := func(ctx context.Context, req any) (any, error) { return "ok", nil }
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Basic abc"))
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/abcmovies.api.v1.CoreService/GetJob"}, handler)
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Fatalf("got code %v, want %v", got, codes.Unauthenticated)
	}
}

func TestBus_PublishSubscribe(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()

	ch := bus.Subscribe("test")
	event := &corev1.EventEnvelope{
		Id:       "evt-1",
		Type:     corev1.EventType_EVENT_TYPE_JOB_STATUS,
		Audience: corev1.EventAudience_EVENT_AUDIENCE_USER,
		UserId:   "user:1",
	}
	bus.Publish(event)

	select {
	case got := <-ch:
		if got.GetId() != "evt-1" {
			t.Fatalf("got event id %q, want %q", got.GetId(), "evt-1")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestBus_Unsubscribe(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()

	ch := bus.Subscribe("test")
	bus.Unsubscribe("test")

	// Channel should be closed.
	_, ok := <-ch
	if ok {
		t.Fatal("channel should be closed after unsubscribe")
	}

	// Unsubscribing again should be a no-op.
	bus.Unsubscribe("test")
}

func TestBus_PublishSlow(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()

	// Subscribe with a full buffer channel (capacity 64).
	ch := bus.Subscribe("slow")

	// Publish 65 events: the 65th should not block (dropped silently).
	for i := range 70 {
		bus.Publish(&corev1.EventEnvelope{
			Id:       "evt",
			Type:     corev1.EventType_EVENT_TYPE_JOB_STATUS,
			Audience: corev1.EventAudience_EVENT_AUDIENCE_USER,
			UserId:   "user:1",
		})
		_ = i
	}

	// Should still have events in the channel (buffered), but publishing
	// did not block.
	got := len(ch)
	if got == 0 || got > 64 {
		t.Fatalf("expected buffered events, got %d", got)
	}
}

func TestServer_GetJob_NotFound(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	srv := apiserver.NewServer(bus, testStores(t), authenticator, session)

	_, err := srv.GetJob(context.Background(), &apiv1.GetJobRequest{JobId: "job-1"})
	if got := status.Code(err); got != codes.NotFound {
		t.Fatalf("got code %v, want %v", got, codes.NotFound)
	}
}

func TestServer_GetJob_EmptyID(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	srv := apiserver.NewServer(bus, testStores(t), authenticator, session)

	_, err := srv.GetJob(context.Background(), &apiv1.GetJobRequest{})
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("got code %v, want %v", got, codes.InvalidArgument)
	}
}

func TestServer_SignUp_Success(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	srv := apiserver.NewServer(bus, testStores(t), authenticator, session)

	resp, err := srv.SignUp(context.Background(), &apiv1.SignUpRequest{
		Username: "alice",
		AuthMethod: &apiv1.SignUpRequest_Password{
			Password: &apiv1.PasswordSignUp{Password: []byte("password123")},
		},
	})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if resp.UserId != "user:alice" {
		t.Fatalf("UserId = %q, want %q", resp.UserId, "user:alice")
	}
	if resp.RecoveryKey == "" {
		t.Fatal("RecoveryKey should not be empty")
	}
}

func TestServer_SignUp_EmptyUsername(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	srv := apiserver.NewServer(bus, testStores(t), authenticator, session)

	_, err := srv.SignUp(context.Background(), &apiv1.SignUpRequest{})
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("got code %v, want %v", got, codes.InvalidArgument)
	}
}

func TestServer_Login_Success(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	srv := apiserver.NewServer(bus, testStores(t), authenticator, session)

	// Sign up.
	_, err := srv.SignUp(context.Background(), &apiv1.SignUpRequest{
		Username: "alice",
		AuthMethod: &apiv1.SignUpRequest_Password{
			Password: &apiv1.PasswordSignUp{Password: []byte("password123")},
		},
	})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	// Login.
	loginResp, err := srv.Login(context.Background(), &apiv1.LoginRequest{
		Username: "alice",
		AuthMethod: &apiv1.LoginRequest_Password{
			Password: &apiv1.PasswordLogin{Password: []byte("password123")},
		},
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if loginResp.Token == "" {
		t.Fatal("Token should not be empty")
	}
}

func TestServer_Login_WrongPassword(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	srv := apiserver.NewServer(bus, testStores(t), authenticator, session)

	_, err := srv.SignUp(context.Background(), &apiv1.SignUpRequest{
		Username: "alice",
		AuthMethod: &apiv1.SignUpRequest_Password{
			Password: &apiv1.PasswordSignUp{Password: []byte("password123")},
		},
	})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	_, err = srv.Login(context.Background(), &apiv1.LoginRequest{
		Username: "alice",
		AuthMethod: &apiv1.LoginRequest_Password{
			Password: &apiv1.PasswordLogin{Password: []byte("wrongpassword")},
		},
	})
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Fatalf("got code %v, want %v", got, codes.Unauthenticated)
	}
}

func TestServer_SignUp_Login_GetJob_Flow(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	srv := apiserver.NewServer(bus, testStores(t), authenticator, session)

	// 1. Sign up.
	signUpResp, err := srv.SignUp(context.Background(), &apiv1.SignUpRequest{
		Username: "alice",
		AuthMethod: &apiv1.SignUpRequest_Password{
			Password: &apiv1.PasswordSignUp{Password: []byte("password123")},
		},
	})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	// 2. Login.
	loginResp, err := srv.Login(context.Background(), &apiv1.LoginRequest{
		Username: "alice",
		AuthMethod: &apiv1.LoginRequest_Password{
			Password: &apiv1.PasswordLogin{Password: []byte("password123")},
		},
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	// 3. Use token to call GetJob (unauthenticated = 401, authenticated = not found).
	// Without auth metadata, should get Unauthenticated.
	_, err = srv.GetJob(context.Background(), &apiv1.GetJobRequest{JobId: "job-1"})
	// GetJob doesn't check auth directly (it's at the interceptor level),
	// but the full gRPC stack would enforce it. Verify we can call it.
	if got := status.Code(err); got != codes.NotFound {
		t.Fatalf("got code %v, want %v", got, codes.NotFound)
	}

	// Verify the token works via the interceptor.
	interceptor := apiserver.AuthUnaryInterceptor(session)
	handler := func(ctx context.Context, req any) (any, error) {
		uid, ok := apiserver.UserIDFromContext(ctx)
		if !ok {
			t.Fatal("user ID not in context")
		}
		if uid != "user:alice" {
			t.Fatalf("got user ID %q, want %q", uid, "user:alice")
		}
		return "ok", nil
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+loginResp.Token))
	resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/abcmovies.api.v1.CoreService/GetJob"}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Fatalf("unexpected response: %v", resp)
	}

	_ = signUpResp // recovery key available for future use
}

func TestServer_GetJob_CreateAndRetrieve(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	srv := apiserver.NewServer(bus, testStores(t), authenticator, session)

	// Create a job.
	job := &corev1.Job{
		Id:     "job-001",
		Kind:   corev1.JobKind_JOB_KIND_REFRESH,
		Status: corev1.JobStatus_JOB_STATUS_QUEUED,
	}
	if err := srv.CreateJob(context.Background(), job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	// Retrieve it through GetJob.
	resp, err := srv.GetJob(context.Background(), &apiv1.GetJobRequest{JobId: "job-001"})
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if resp.GetJob().GetId() != "job-001" {
		t.Fatalf("Job ID = %q, want %q", resp.GetJob().GetId(), "job-001")
	}
	if resp.GetJob().GetKind() != corev1.JobKind_JOB_KIND_REFRESH {
		t.Fatalf("Job Kind = %v, want %v", resp.GetJob().GetKind(), corev1.JobKind_JOB_KIND_REFRESH)
	}
	if resp.GetJob().GetStatus() != corev1.JobStatus_JOB_STATUS_QUEUED {
		t.Fatalf("Job Status = %v, want %v", resp.GetJob().GetStatus(), corev1.JobStatus_JOB_STATUS_QUEUED)
	}
}

func TestPerUserBlobEncryption_FullFlow(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	stores := testStores(t)

	// Wrap WatchHistory with UserBlobStore for per-user encryption.
	inner := stores.WatchHistory
	stores.WatchHistory = store.NewUserBlobStore(inner)

	srv := apiserver.NewServer(bus, stores, authenticator, session)

	// 1. Sign up Alice.
	_, err := srv.SignUp(context.Background(), &apiv1.SignUpRequest{
		Username: "alice",
		AuthMethod: &apiv1.SignUpRequest_Password{
			Password: &apiv1.PasswordSignUp{Password: []byte("password123")},
		},
	})
	if err != nil {
		t.Fatalf("SignUp alice: %v", err)
	}

	// 2. Login Alice — DEK gets cached.
	loginResp, err := srv.Login(context.Background(), &apiv1.LoginRequest{
		Username: "alice",
		AuthMethod: &apiv1.LoginRequest_Password{
			Password: &apiv1.PasswordLogin{Password: []byte("password123")},
		},
	})
	if err != nil {
		t.Fatalf("Login alice: %v", err)
	}

	// 3. Get an authenticated context with Alice's DEK.
	interceptor := apiserver.AuthUnaryInterceptor(session)
	var aliceCtx context.Context
	aliceHandler := func(ctx context.Context, req any) (any, error) {
		aliceCtx = ctx
		return "ok", nil
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+loginResp.Token))
	_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/abcmovies.api.v1.CoreService/GetJob"}, aliceHandler)
	if err != nil {
		t.Fatalf("interceptor: %v", err)
	}

	// 4. Put a value through WatchHistory using Alice's context.
	if err := stores.WatchHistory.Put(aliceCtx, "movie:1", []byte("watched-at-noon")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// 5. Verify the raw value on the inner store is encrypted.
	raw, err := inner.Get(context.Background(), "user:user:alice:movie:1")
	if err != nil {
		t.Fatalf("inner Get: %v", err)
	}
	if string(raw) == "watched-at-noon" {
		t.Fatal("raw value contains plaintext — not encrypted")
	}

	// 6. Get the value back — should decrypt correctly.
	got, err := stores.WatchHistory.Get(aliceCtx, "movie:1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "watched-at-noon" {
		t.Fatalf("Get = %q, want %q", got, "watched-at-noon")
	}

	// 7. Sign up Bob, login, and verify he can't read Alice's data.
	_, err = srv.SignUp(context.Background(), &apiv1.SignUpRequest{
		Username: "bob",
		AuthMethod: &apiv1.SignUpRequest_Password{
			Password: &apiv1.PasswordSignUp{Password: []byte("password456")},
		},
	})
	if err != nil {
		t.Fatalf("SignUp bob: %v", err)
	}
	bobLoginResp, err := srv.Login(context.Background(), &apiv1.LoginRequest{
		Username: "bob",
		AuthMethod: &apiv1.LoginRequest_Password{
			Password: &apiv1.PasswordLogin{Password: []byte("password456")},
		},
	})
	if err != nil {
		t.Fatalf("Login bob: %v", err)
	}

	var bobCtx context.Context
	bobHandler := func(ctx context.Context, req any) (any, error) {
		bobCtx = ctx
		return "ok", nil
	}
	bobMeta := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+bobLoginResp.Token))
	_, err = interceptor(bobMeta, nil, &grpc.UnaryServerInfo{FullMethod: "/abcmovies.api.v1.CoreService/GetJob"}, bobHandler)
	if err != nil {
		t.Fatalf("interceptor bob: %v", err)
	}

	// Bob should not be able to read Alice's data.
	_, err = stores.WatchHistory.Get(bobCtx, "movie:1")
	if err == nil {
		t.Fatal("bob should not be able to read alice's data")
	}
}

func TestServer_CreateJob_PublishesEvent(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	srv := apiserver.NewServer(bus, testStores(t), authenticator, session)

	ch := bus.Subscribe("test-sub")
	defer bus.Unsubscribe("test-sub")

	job := &corev1.Job{
		Id:          "job-evt-1",
		Kind:        corev1.JobKind_JOB_KIND_REFRESH,
		Status:      corev1.JobStatus_JOB_STATUS_QUEUED,
		OwnerUserId: "user:alice",
	}
	if err := srv.CreateJob(context.Background(), job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	select {
	case event := <-ch:
		if event.GetType() != corev1.EventType_EVENT_TYPE_JOB_STATUS {
			t.Fatalf("event type = %v, want %v", event.GetType(), corev1.EventType_EVENT_TYPE_JOB_STATUS)
		}
		if event.GetAudience() != corev1.EventAudience_EVENT_AUDIENCE_USER {
			t.Fatalf("event audience = %v, want %v", event.GetAudience(), corev1.EventAudience_EVENT_AUDIENCE_USER)
		}
		if event.GetUserId() != "user:alice" {
			t.Fatalf("event user_id = %q, want %q", event.GetUserId(), "user:alice")
		}
		payload := event.GetJobStatus()
		if payload == nil {
			t.Fatal("event payload should be JobStatus")
		}
		if payload.GetJobId() != "job-evt-1" {
			t.Fatalf("payload job_id = %q, want %q", payload.GetJobId(), "job-evt-1")
		}
		if payload.GetStatus() != corev1.JobStatus_JOB_STATUS_QUEUED {
			t.Fatalf("payload status = %v, want %v", payload.GetStatus(), corev1.JobStatus_JOB_STATUS_QUEUED)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for job-status event")
	}
}

func TestServer_SignUp_DuplicateUsername(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	srv := apiserver.NewServer(bus, testStores(t), authenticator, session)

	_, err := srv.SignUp(context.Background(), &apiv1.SignUpRequest{
		Username: "alice",
		AuthMethod: &apiv1.SignUpRequest_Password{
			Password: &apiv1.PasswordSignUp{Password: []byte("password123")},
		},
	})
	if err != nil {
		t.Fatalf("first SignUp: %v", err)
	}

	_, err = srv.SignUp(context.Background(), &apiv1.SignUpRequest{
		Username: "alice",
		AuthMethod: &apiv1.SignUpRequest_Password{
			Password: &apiv1.PasswordSignUp{Password: []byte("other456")},
		},
	})
	if err == nil {
		t.Fatal("expected error for duplicate username")
	}
}

func TestServer_Login_BeforeSignUp(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	srv := apiserver.NewServer(bus, testStores(t), authenticator, session)

	_, err := srv.Login(context.Background(), &apiv1.LoginRequest{
		Username: "nobody",
		AuthMethod: &apiv1.LoginRequest_Password{
			Password: &apiv1.PasswordLogin{Password: []byte("password123")},
		},
	})
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Fatalf("got code %v, want %v", got, codes.Unauthenticated)
	}
}

func TestServer_GetJob_AuthBehavior(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	srv := apiserver.NewServer(bus, testStores(t), authenticator, session)

	// GetJob without auth metadata returns NotFound (auth is at interceptor level).
	_, err := srv.GetJob(context.Background(), &apiv1.GetJobRequest{JobId: "job-1"})
	if got := status.Code(err); got != codes.NotFound {
		t.Fatalf("got code %v, want %v", got, codes.NotFound)
	}
}
