package auth_test

import (
	"testing"
	"time"

	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/store"
)

func TestSession_Mint_Validate(t *testing.T) {
	tokens := auth.NewStoreTokenStore(store.NewInMemory())
	deks := auth.NewMemoryDEKCache()
	session := auth.NewSessionHandler(tokens, deks, time.Hour)

	token, err := session.Mint("user:alice")
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if token == "" {
		t.Fatal("token should not be empty")
	}

	uid, err := session.Validate(token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if uid != "user:alice" {
		t.Fatalf("UserID = %q, want %q", uid, "user:alice")
	}
}

func TestSession_Validate_InvalidToken(t *testing.T) {
	tokens := auth.NewStoreTokenStore(store.NewInMemory())
	deks := auth.NewMemoryDEKCache()
	session := auth.NewSessionHandler(tokens, deks, time.Hour)

	_, err := session.Validate("nonexistent-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestSession_Revoke(t *testing.T) {
	tokens := auth.NewStoreTokenStore(store.NewInMemory())
	deks := auth.NewMemoryDEKCache()
	session := auth.NewSessionHandler(tokens, deks, time.Hour)

	token, _ := session.Mint("user:alice")

	err := session.Revoke(token)
	if err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	_, err = session.Validate(token)
	if err == nil {
		t.Fatal("expected error after revoke")
	}
}

func TestSession_DEK_PerSessionKeying(t *testing.T) {
	tokens := auth.NewStoreTokenStore(store.NewInMemory())
	deks := auth.NewMemoryDEKCache()
	session := auth.NewSessionHandler(tokens, deks, time.Hour)

	tokenA, _ := session.Mint("user:alice")
	tokenB, _ := session.Mint("user:alice") // second session, same user

	dek := []byte("data-encryption-key")
	if err := session.StoreDEK(tokenA, dek); err != nil {
		t.Fatalf("StoreDEK: %v", err)
	}

	// The DEK is bound to token A's session only.
	if got := session.GetDEK(tokenA); string(got) != string(dek) {
		t.Fatalf("GetDEK(session A) = %q, want %q", got, dek)
	}
	if got := session.GetDEK(tokenB); got != nil {
		t.Fatalf("GetDEK(session B) = %q, want nil", got)
	}

	// Revoking session A drops its key material; session B keeps living.
	if err := session.Revoke(tokenA); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if got := session.GetDEK(tokenA); got != nil {
		t.Fatalf("GetDEK after Revoke = %v, want nil", got)
	}
}

func TestSession_DEK_EvictedOnExpiry(t *testing.T) {
	tokens := auth.NewStoreTokenStore(store.NewInMemory())
	deks := auth.NewMemoryDEKCache()
	session := auth.NewSessionHandler(tokens, deks, time.Millisecond)

	token, _ := session.Mint("user:alice")
	if err := session.StoreDEK(token, []byte("key")); err != nil {
		t.Fatalf("StoreDEK: %v", err)
	}

	time.Sleep(5 * time.Millisecond)

	// Validating the expired token cleans up token and key material.
	if _, err := session.Validate(token); err == nil {
		t.Fatal("expected error for expired token")
	}
	if got := session.GetDEK(token); got != nil {
		t.Fatalf("GetDEK after expiry = %v, want nil", got)
	}
}

func TestSession_TokenExpiry(t *testing.T) {
	tokens := auth.NewStoreTokenStore(store.NewInMemory())
	deks := auth.NewMemoryDEKCache()
	session := auth.NewSessionHandler(tokens, deks, time.Millisecond)

	token, _ := session.Mint("user:alice")

	// Wait for token to expire.
	time.Sleep(5 * time.Millisecond)

	_, err := session.Validate(token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestSession_DifferentUsers(t *testing.T) {
	tokens := auth.NewStoreTokenStore(store.NewInMemory())
	deks := auth.NewMemoryDEKCache()
	session := auth.NewSessionHandler(tokens, deks, time.Hour)

	token1, _ := session.Mint("user:alice")
	token2, _ := session.Mint("user:bob")

	uid1, _ := session.Validate(token1)
	uid2, _ := session.Validate(token2)

	if uid1 == uid2 {
		t.Fatal("tokens for different users should yield different user IDs")
	}
}
