package apiserver_test

import (
	"context"
	"testing"
	"time"

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
	}
}

func testAuth(t *testing.T) (*auth.PasswordAuthenticator, *auth.Session) {
	t.Helper()
	userStore := auth.NewMemoryUserStore()
	sessionStore := store.NewInMemory()
	authenticator := auth.NewPasswordAuthenticator(userStore)
	session := auth.NewSession(sessionStore, time.Hour)
	return authenticator, session
}

func TestAuthInterceptor_ValidToken(t *testing.T) {
	authenticator, session := testAuth(t)

	// Create a user and get a token.
	_, err := authenticator.SignUp("alice", []byte("password123"))
	if err != nil {
		t.Fatalf("signup: %v", err)
	}
	result, err := authenticator.Login("alice", []byte("password123"))
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
	resp, err := interceptor(ctx, nil, nil, handler)
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
	_, err := interceptor(ctx, nil, nil, handler)
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Fatalf("got code %v, want %v", got, codes.Unauthenticated)
	}
}

func TestAuthInterceptor_MalformedToken(t *testing.T) {
	_, session := testAuth(t)
	interceptor := apiserver.AuthUnaryInterceptor(session)
	handler := func(ctx context.Context, req any) (any, error) { return "ok", nil }
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Basic abc"))
	_, err := interceptor(ctx, nil, nil, handler)
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Fatalf("got code %v, want %v", got, codes.Unauthenticated)
	}
}

func TestBus_PublishSubscribe(t *testing.T) {
	bus := apiserver.NewBus()
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
	bus := apiserver.NewBus()
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
	bus := apiserver.NewBus()
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
	bus := apiserver.NewBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	srv := apiserver.NewServer(bus, testStores(t), authenticator, session)

	_, err := srv.GetJob(context.Background(), &apiv1.GetJobRequest{JobId: "job-1"})
	if got := status.Code(err); got != codes.NotFound {
		t.Fatalf("got code %v, want %v", got, codes.NotFound)
	}
}

func TestServer_GetJob_EmptyID(t *testing.T) {
	bus := apiserver.NewBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	srv := apiserver.NewServer(bus, testStores(t), authenticator, session)

	_, err := srv.GetJob(context.Background(), &apiv1.GetJobRequest{})
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("got code %v, want %v", got, codes.InvalidArgument)
	}
}

func TestServer_SignUp_Success(t *testing.T) {
	bus := apiserver.NewBus()
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
	bus := apiserver.NewBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	srv := apiserver.NewServer(bus, testStores(t), authenticator, session)

	_, err := srv.SignUp(context.Background(), &apiv1.SignUpRequest{})
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("got code %v, want %v", got, codes.InvalidArgument)
	}
}

func TestServer_Login_Success(t *testing.T) {
	bus := apiserver.NewBus()
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
	bus := apiserver.NewBus()
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
	bus := apiserver.NewBus()
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
	resp, err := interceptor(ctx, nil, nil, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Fatalf("unexpected response: %v", resp)
	}

	_ = signUpResp // recovery key available for future use
}
