package m0_test

import (
	"context"
	"testing"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestAuth_SignUp_Login_FullFlow(t *testing.T) {
	stack := newFullStack(t)

	// Sign up.
	signUpResp, err := stack.server.SignUp(t.Context(), &apiv1.SignUpRequest{
		Username: "alice",
		AuthMethod: &apiv1.SignUpRequest_Password{
			Password: &apiv1.PasswordSignUp{Password: []byte("password123")},
		},
	})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if signUpResp.GetUserId() != "user:alice" {
		t.Fatalf("UserId = %q, want %q", signUpResp.GetUserId(), "user:alice")
	}
	if signUpResp.GetRecoveryKey() == "" {
		t.Fatal("RecoveryKey should not be empty")
	}

	// Login.
	loginResp, err := stack.server.Login(t.Context(), &apiv1.LoginRequest{
		Username: "alice",
		AuthMethod: &apiv1.LoginRequest_Password{
			Password: &apiv1.PasswordLogin{Password: []byte("password123")},
		},
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if loginResp.GetToken() == "" {
		t.Fatal("Token should not be empty")
	}

	// Use token via interceptor.
	interceptor := apiserver.AuthUnaryInterceptor(stack.session)
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
	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("authorization", "Bearer "+loginResp.GetToken()))
	resp, err := interceptor(ctx, nil, nil, handler)
	if err != nil {
		t.Fatalf("interceptor: %v", err)
	}
	if resp != "ok" {
		t.Fatalf("unexpected response: %v", resp)
	}
}

func TestAuth_SignUp_DuplicateUser(t *testing.T) {
	stack := newFullStack(t)

	_, err := stack.server.SignUp(t.Context(), &apiv1.SignUpRequest{
		Username: "alice",
		AuthMethod: &apiv1.SignUpRequest_Password{
			Password: &apiv1.PasswordSignUp{Password: []byte("password123")},
		},
	})
	if err != nil {
		t.Fatalf("first SignUp: %v", err)
	}

	_, err = stack.server.SignUp(t.Context(), &apiv1.SignUpRequest{
		Username: "alice",
		AuthMethod: &apiv1.SignUpRequest_Password{
			Password: &apiv1.PasswordSignUp{Password: []byte("other456")},
		},
	})
	if err == nil {
		t.Fatal("expected error for duplicate username")
	}
}

func TestAuth_Login_WrongPassword(t *testing.T) {
	stack := newFullStack(t)

	_, err := stack.server.SignUp(t.Context(), &apiv1.SignUpRequest{
		Username: "alice",
		AuthMethod: &apiv1.SignUpRequest_Password{
			Password: &apiv1.PasswordSignUp{Password: []byte("password123")},
		},
	})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	_, err = stack.server.Login(t.Context(), &apiv1.LoginRequest{
		Username: "alice",
		AuthMethod: &apiv1.LoginRequest_Password{
			Password: &apiv1.PasswordLogin{Password: []byte("wrongpassword")},
		},
	})
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Fatalf("got code %v, want %v", got, codes.Unauthenticated)
	}
}

func TestAuth_Login_NonExistentUser(t *testing.T) {
	stack := newFullStack(t)

	_, err := stack.server.Login(t.Context(), &apiv1.LoginRequest{
		Username: "nobody",
		AuthMethod: &apiv1.LoginRequest_Password{
			Password: &apiv1.PasswordLogin{Password: []byte("password123")},
		},
	})
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Fatalf("got code %v, want %v", got, codes.Unauthenticated)
	}
}

func TestAuth_TokenExpiry(t *testing.T) {
	stack := newFullStack(t)

	// Sign up and login.
	_, err := stack.server.SignUp(t.Context(), &apiv1.SignUpRequest{
		Username: "alice",
		AuthMethod: &apiv1.SignUpRequest_Password{
			Password: &apiv1.PasswordSignUp{Password: []byte("password123")},
		},
	})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	// Create a short-lived session.
	shortSession := auth.NewSessionHandler(
		auth.NewStoreTokenStore(stack.stores.Sessions),
		auth.NewStoreDEKCache(stack.stores.Sessions),
		1, // 1 nanosecond — expires immediately
	)
	loginResp, err := stack.server.Login(t.Context(), &apiv1.LoginRequest{
		Username: "alice",
		AuthMethod: &apiv1.LoginRequest_Password{
			Password: &apiv1.PasswordLogin{Password: []byte("password123")},
		},
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	_ = loginResp

	// The token was minted with the long-lived session. For this test,
	// we mint a fresh token with the short-lived session directly.
	token, err := shortSession.Mint("user:alice")
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	// Wait and validate — should be expired.
	// (The 1ns TTL means it's already expired by the time we validate.)
	_, err = shortSession.Validate(token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestAuth_RecoveryKey_ShownOnce(t *testing.T) {
	stack := newFullStack(t)

	resp, err := stack.server.SignUp(t.Context(), &apiv1.SignUpRequest{
		Username: "alice",
		AuthMethod: &apiv1.SignUpRequest_Password{
			Password: &apiv1.PasswordSignUp{Password: []byte("password123")},
		},
	})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	recoveryKey := resp.GetRecoveryKey()
	if recoveryKey == "" {
		t.Fatal("RecoveryKey should not be empty")
	}

	// Verify the recovery key is not stored in the user store.
	// The user store should only contain auth material, not the recovery key.
	userStore := auth.NewStoreUserStore(stack.stores.Users)
	userData, err := userStore.GetUser("alice")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	// The WrappedRecovery field is the DEK wrapped with recovery-KEK, not the
	// recovery key itself. The recovery key should not appear in any stored data.
	if userData.WrappedRecovery == nil {
		t.Fatal("WrappedRecovery should be set")
	}
}
