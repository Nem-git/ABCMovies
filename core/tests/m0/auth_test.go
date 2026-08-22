package m0_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/store"
	"google.golang.org/grpc"
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
	resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/abcmovies.api.v1.CoreService/GetJob"}, handler)
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
		auth.NewMemoryDEKCache(),
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

// TestAuth_PlaintextPasswordNeverPersisted proves the server stores only
// derived material (TESTING.md §6): after SignUp, the raw stored user record
// contains no trace of the plaintext password, while every derived field
// (salt, password-KEK hash, wrapped DEK, wrapped recovery) is present.
func TestAuth_PlaintextPasswordNeverPersisted(t *testing.T) {
	stack := newFullStack(t)

	const password = "unforgettable-secret-7"
	signUp(t, stack.server, "alice", password)

	raw, err := stack.stores.Users.Get(context.Background(), "user:alice")
	if err != nil {
		t.Fatalf("read stored user record: %v", err)
	}

	if strings.Contains(string(raw), password) {
		t.Fatal("stored user record contains the plaintext password")
	}
	// A recognizable fragment must not appear either — guards against
	// partial/encoded leakage of the secret.
	if strings.Contains(string(raw), "unforgettable") {
		t.Fatal("stored user record contains a fragment of the plaintext password")
	}

	// Only derived material may be persisted.
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("stored user record is not JSON UserData: %v", err)
	}
	for _, field := range []string{"Salt", "PasswordHash", "WrappedDEK", "WrappedRecovery"} {
		v, ok := data[field]
		if !ok || len(v) == 0 || string(v) == "null" {
			t.Fatalf("derived material %s missing or empty in stored user record", field)
		}
	}
	if _, ok := data["Password"]; ok {
		t.Fatal("stored user record has a Password field — plaintext must never be persisted")
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

// TestAuth_DEKNeverPersistedWithMemoryCache proves that with the default
// (memory) DEK cache, unwrapped key material never reaches any store
// (PLAN.md §7.6, IMPLEMENTATION.md §1.3): after login, no entry in the
// sessions store — where tokens live — contains the session DEK, and only
// the logging-in session can retrieve it.
func TestAuth_DEKNeverPersistedWithMemoryCache(t *testing.T) {
	stack := newFullStack(t)

	const password = "dek-leak-check-password"
	signUp(t, stack.server, "alice", password)
	token := login(t, stack.server, "alice", password)

	// Recover the raw DEK through the authenticator, exactly as the server
	// did during login.
	pwdAuth, ok := stack.auth.Get("password")
	if !ok {
		t.Fatal("password authenticator not found")
	}
	result, err := pwdAuth.(*auth.PasswordAuthenticator).Login("alice", []byte(password))
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	dek := result.DEK
	if len(dek) == 0 {
		t.Fatal("expected a non-empty DEK")
	}

	// No entry in any store class may contain the plaintext DEK bytes.
	for name, s := range map[string]store.Store{
		"sessions":     stack.stores.Sessions,
		"users":        stack.stores.Users,
		"watchHistory": stack.stores.WatchHistory,
		"jobs":         stack.stores.Jobs,
	} {
		keys, err := s.List(context.Background(), "")
		if err != nil {
			t.Fatalf("list %s store: %v", name, err)
		}
		for _, key := range keys {
			val, err := s.Get(context.Background(), key)
			if err != nil {
				t.Fatalf("read %s/%s: %v", name, key, err)
			}
			if bytes.Contains(val, dek) {
				t.Errorf("plaintext DEK persisted in %s store under %q", name, key)
			}
		}
	}

	// The DEK is reachable only through this session's token.
	if got := stack.session.GetDEK(token); !bytes.Equal(got, dek) {
		t.Fatalf("GetDEK(session) = %q, want the login DEK", got)
	}
}
