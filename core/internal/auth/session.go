package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/nem-git/abcmovies/core/internal/store"
)

const tokenLen = 32

// Session manages API session tokens (TECHNICAL-DECISIONS §1.12).
// Tokens are opaque bearer tokens; the store holds the token's SHA-256
// hash, never the token value.
type Session struct {
	store store.Store
	ttl   time.Duration
}

// NewSession returns a Session backed by the given store with the given TTL.
func NewSession(store store.Store, ttl time.Duration) *Session {
	return &Session{store: store, ttl: ttl}
}

// Mint generates a new session token, stores its hash, and returns the
// raw token string.
func (s *Session) Mint(userID string) (string, error) {
	b := make([]byte, tokenLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("session: generate token: %w", err)
	}
	token := hex.EncodeToString(b)

	hash := sha256.Sum256([]byte(token))
	key := "session:" + hex.EncodeToString(hash[:])

	expiry := time.Now().Add(s.ttl).UTC().Format(time.RFC3339)
	value := []byte(userID + "\n" + expiry)

	if err := s.store.Put(context.TODO(), key, value); err != nil {
		return "", fmt.Errorf("session: store: %w", err)
	}
	return token, nil
}

// Validate checks a token and returns the associated user ID.
// Returns an error if the token is invalid or expired.
func (s *Session) Validate(token string) (string, error) {
	hash := sha256.Sum256([]byte(token))
	key := "session:" + hex.EncodeToString(hash[:])

	value, err := s.store.Get(context.TODO(), key)
	if err != nil {
		return "", fmt.Errorf("session: invalid token")
	}

	// Parse userID\nexpiry.
	data := string(value)
	var userID, expiryStr string
	for i := range data {
		if data[i] == '\n' {
			userID = data[:i]
			expiryStr = data[i+1:]
			break
		}
	}
	if userID == "" || expiryStr == "" {
		return "", fmt.Errorf("session: corrupted entry")
	}

	expiry, err := time.Parse(time.RFC3339, expiryStr)
	if err != nil {
		return "", fmt.Errorf("session: corrupted expiry")
	}
	if time.Now().After(expiry) {
		// Clean up expired token.
		_ = s.store.Delete(context.TODO(), key)
		return "", fmt.Errorf("session: token expired")
	}

	return userID, nil
}

// Revoke removes a session token.
func (s *Session) Revoke(token string) error {
	hash := sha256.Sum256([]byte(token))
	key := "session:" + hex.EncodeToString(hash[:])
	return s.store.Delete(context.TODO(), key)
}
