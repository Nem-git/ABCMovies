package auth_test

import (
	"testing"

	"github.com/nem-git/abcmovies/core/internal/auth"
)

func TestSignUp_Success(t *testing.T) {
	store := auth.NewMemoryUserStore()
	authn := auth.NewPasswordAuthenticator(store)

	result, err := authn.SignUp("alice", []byte("password123"))
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if result.UserID != "user:alice" {
		t.Fatalf("UserID = %q, want %q", result.UserID, "user:alice")
	}
	if result.RecoveryKey == "" {
		t.Fatal("RecoveryKey should not be empty")
	}
}

func TestSignUp_EmptyUsername(t *testing.T) {
	store := auth.NewMemoryUserStore()
	authn := auth.NewPasswordAuthenticator(store)

	_, err := authn.SignUp("", []byte("password123"))
	if err == nil {
		t.Fatal("expected error for empty username")
	}
}

func TestSignUp_EmptyPassword(t *testing.T) {
	store := auth.NewMemoryUserStore()
	authn := auth.NewPasswordAuthenticator(store)

	_, err := authn.SignUp("alice", nil)
	if err == nil {
		t.Fatal("expected error for nil password")
	}
}

func TestSignUp_DuplicateUsername(t *testing.T) {
	store := auth.NewMemoryUserStore()
	authn := auth.NewPasswordAuthenticator(store)

	_, err := authn.SignUp("alice", []byte("password123"))
	if err != nil {
		t.Fatalf("first SignUp: %v", err)
	}

	_, err = authn.SignUp("alice", []byte("other456"))
	if err == nil {
		t.Fatal("expected error for duplicate username")
	}
}

func TestLogin_Success(t *testing.T) {
	store := auth.NewMemoryUserStore()
	authn := auth.NewPasswordAuthenticator(store)

	_, err := authn.SignUp("alice", []byte("password123"))
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	result, err := authn.Login("alice", []byte("password123"))
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if result.UserID != "user:alice" {
		t.Fatalf("UserID = %q, want %q", result.UserID, "user:alice")
	}
	if len(result.DEK) == 0 {
		t.Fatal("DEK should not be empty")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	store := auth.NewMemoryUserStore()
	authn := auth.NewPasswordAuthenticator(store)

	_, err := authn.SignUp("alice", []byte("password123"))
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	_, err = authn.Login("alice", []byte("wrongpassword"))
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	store := auth.NewMemoryUserStore()
	authn := auth.NewPasswordAuthenticator(store)

	_, err := authn.Login("bob", []byte("password123"))
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

func TestCompositeAuthenticator_Get(t *testing.T) {
	userStore := auth.NewMemoryUserStore()
	composite, err := auth.NewAuthenticators([]string{"password"}, userStore)
	if err != nil {
		t.Fatalf("NewAuthenticators: %v", err)
	}

	a, ok := composite.Get("password")
	if !ok {
		t.Fatal("expected to find password authenticator")
	}
	if a == nil {
		t.Fatal("password authenticator should not be nil")
	}

	_, ok = composite.Get("unknown")
	if ok {
		t.Fatal("expected unknown method to not be found")
	}
}

func TestNewAuthenticators_Password(t *testing.T) {
	userStore := auth.NewMemoryUserStore()
	composite, err := auth.NewAuthenticators([]string{"password"}, userStore)
	if err != nil {
		t.Fatalf("NewAuthenticators: %v", err)
	}

	a, ok := composite.Get("password")
	if !ok {
		t.Fatal("password method should be registered")
	}

	result, err := a.SignUp("alice", []byte("pass123"))
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if result.UserID != "user:alice" {
		t.Fatalf("UserID = %q, want %q", result.UserID, "user:alice")
	}
}

func TestNewAuthenticators_UnknownMethod(t *testing.T) {
	userStore := auth.NewMemoryUserStore()
	_, err := auth.NewAuthenticators([]string{"oauth"}, userStore)
	if err == nil {
		t.Fatal("expected error for unknown method")
	}
}
