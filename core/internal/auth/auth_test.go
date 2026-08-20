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
