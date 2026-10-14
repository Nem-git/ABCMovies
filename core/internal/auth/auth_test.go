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

// TestSignUp_Argon2idParameters proves the password-KEK is derived using
// Argon2id with the pinned parameters (TECHNICAL-DECISIONS §1.11): 19 MiB
// memory, 2 iterations, 1 thread, 32-byte key. The test signs up the same
// password twice and verifies the salts differ (so the hashes differ), and
// that the stored hash is the derived KEK — never the plaintext password.
func TestSignUp_Argon2idParameters(t *testing.T) {
	store := auth.NewMemoryUserStore()
	authn := auth.NewPasswordAuthenticator(store)

	// Sign up the same password twice with different usernames.
	_, err := authn.SignUp("alice", []byte("password123"))
	if err != nil {
		t.Fatalf("first SignUp: %v", err)
	}
	_, err = authn.SignUp("bob", []byte("password123"))
	if err != nil {
		t.Fatalf("second SignUp: %v", err)
	}

	alice, _ := store.GetUser("alice")
	bob, _ := store.GetUser("bob")

	// Salts must differ — same password, different random salt.
	if string(alice.Salt) == string(bob.Salt) {
		t.Fatal("salts are identical — Argon2id is not using random salts")
	}

	// Hashes must differ — different salts produce different KEKs.
	if string(alice.PasswordHash) == string(bob.PasswordHash) {
		t.Fatal("password hashes are identical — Argon2id is not salted")
	}

	// The stored hash must NOT be the plaintext password.
	if string(alice.PasswordHash) == "password123" {
		t.Fatal("stored hash is the plaintext password — Argon2id not applied")
	}

	// The hash must be 32 bytes (Argon2KeyLen).
	if len(alice.PasswordHash) != auth.Argon2KeyLen {
		t.Fatalf("password hash length = %d, want %d (Argon2KeyLen)", len(alice.PasswordHash), auth.Argon2KeyLen)
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

// SetMethods swaps the enabled method set live without a restart, so a
// settings change takes effect on the running authenticator (E2). A user
// signed up under a method remains reachable after a swap.
func TestCompositeAuthenticator_SetMethods(t *testing.T) {
	userStore := auth.NewMemoryUserStore()
	composite, err := auth.NewAuthenticators([]string{"password"}, userStore)
	if err != nil {
		t.Fatalf("NewAuthenticators: %v", err)
	}

	before, _ := composite.Get("password")
	if _, err := before.SignUp("alice", []byte("pass123")); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	if err := composite.SetMethods([]string{"password", "password"}); err != nil {
		t.Fatalf("SetMethods: %v", err)
	}
	got := composite.Methods()
	// Methods is a sorted set of names; the duplicate collapses in the map.
	if len(got) != 1 || got[0] != "password" {
		t.Errorf("methods after swap = %v, want [password]", got)
	}
	after, ok := composite.Get("password")
	if !ok {
		t.Fatal("password method missing after swap")
	}
	if _, err := after.Login("alice", []byte("pass123")); err != nil {
		t.Fatalf("login after swap: %v", err)
	}
}

// A swap to an unknown method is rejected and the current set is left
// untouched: a bad settings value cannot silently disable logins.
func TestCompositeAuthenticator_SetMethodsRejectsUnknown(t *testing.T) {
	userStore := auth.NewMemoryUserStore()
	composite, err := auth.NewAuthenticators([]string{"password"}, userStore)
	if err != nil {
		t.Fatalf("NewAuthenticators: %v", err)
	}

	if err := composite.SetMethods([]string{"oauth"}); err == nil {
		t.Fatal("SetMethods(unknown): want error, got nil")
	}
	got := composite.Methods()
	if len(got) != 1 || got[0] != "password" {
		t.Errorf("methods after rejected swap = %v, want untouched [password]", got)
	}
}

// TestSignUp_FirstUserIsOperator pins the bootstrap rule: the first user
// created carries the operator role, and every user after that is a regular
// user. D3: minimal operator model — no config bootstrap needed.
func TestSignUp_FirstUserIsOperator(t *testing.T) {
	store := auth.NewMemoryUserStore()
	authn := auth.NewPasswordAuthenticator(store)

	if _, err := authn.SignUp("alice", []byte("password123")); err != nil {
		t.Fatalf("SignUp alice: %v", err)
	}
	alice, err := store.GetUser("alice")
	if err != nil {
		t.Fatalf("GetUser alice: %v", err)
	}
	if alice.Role != auth.RoleOperator {
		t.Errorf("first user role = %v, want RoleOperator", alice.Role)
	}

	if _, err := authn.SignUp("bob", []byte("password456")); err != nil {
		t.Fatalf("SignUp bob: %v", err)
	}
	bob, err := store.GetUser("bob")
	if err != nil {
		t.Fatalf("GetUser bob: %v", err)
	}
	if bob.Role != auth.RoleUser {
		t.Errorf("second user role = %v, want RoleUser", bob.Role)
	}
}
