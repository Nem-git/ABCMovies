package auth_test

import (
	"testing"
	"time"

	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/store"
)

func TestSession_Mint_Validate(t *testing.T) {
	tokens := auth.NewStoreTokenStore(store.NewInMemory())
	deks := auth.NewStoreDEKCache(store.NewInMemory())
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
	deks := auth.NewStoreDEKCache(store.NewInMemory())
	session := auth.NewSessionHandler(tokens, deks, time.Hour)

	_, err := session.Validate("nonexistent-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestSession_Revoke(t *testing.T) {
	tokens := auth.NewStoreTokenStore(store.NewInMemory())
	deks := auth.NewStoreDEKCache(store.NewInMemory())
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

func TestSession_TokenExpiry(t *testing.T) {
	tokens := auth.NewStoreTokenStore(store.NewInMemory())
	deks := auth.NewStoreDEKCache(store.NewInMemory())
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
	deks := auth.NewStoreDEKCache(store.NewInMemory())
	session := auth.NewSessionHandler(tokens, deks, time.Hour)

	token1, _ := session.Mint("user:alice")
	token2, _ := session.Mint("user:bob")

	uid1, _ := session.Validate(token1)
	uid2, _ := session.Validate(token2)

	if uid1 == uid2 {
		t.Fatal("tokens for different users should yield different user IDs")
	}
}
