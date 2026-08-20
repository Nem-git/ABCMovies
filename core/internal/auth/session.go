package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

const tokenLen = 32

// Session defines the session operations used by the API layer.
type Session interface {
	Mint(userID string) (string, error)
	Validate(token string) (string, error)
	Revoke(token string) error
	StoreDEK(userID string, dek []byte)
	GetDEK(userID string) []byte
}

// InMemorySession is a Session backed by a TokenStore and DEKCache.
// Tokens are opaque bearer tokens; the store holds the token's SHA-256
// hash, never the token value.
type InMemorySession struct {
	tokens TokenStore
	deks   DEKCache
	ttl    time.Duration
}

// NewInMemorySession returns a Session backed by the given stores with the
// given TTL.
func NewInMemorySession(tokens TokenStore, deks DEKCache, ttl time.Duration) *InMemorySession {
	return &InMemorySession{tokens: tokens, deks: deks, ttl: ttl}
}

// Mint generates a new session token, stores its hash, and returns the
// raw token string.
func (s *InMemorySession) Mint(userID string) (string, error) {
	b := make([]byte, tokenLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("session: generate token: %w", err)
	}
	token := hex.EncodeToString(b)

	hash := sha256.Sum256([]byte(token))
	key := "session:" + hex.EncodeToString(hash[:])

	expiry := time.Now().Add(s.ttl).UTC().Format(time.RFC3339)
	value := []byte(userID + "\n" + expiry)

	if err := s.tokens.Save(context.TODO(), key, value); err != nil {
		return "", fmt.Errorf("session: store: %w", err)
	}
	return token, nil
}

// Validate checks a token and returns the associated user ID.
// Returns an error if the token is invalid or expired.
func (s *InMemorySession) Validate(token string) (string, error) {
	hash := sha256.Sum256([]byte(token))
	key := "session:" + hex.EncodeToString(hash[:])

	value, err := s.tokens.Load(context.TODO(), key)
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
		_ = s.tokens.Delete(context.TODO(), key)
		return "", fmt.Errorf("session: token expired")
	}

	return userID, nil
}

// Revoke removes a session token.
func (s *InMemorySession) Revoke(token string) error {
	hash := sha256.Sum256([]byte(token))
	key := "session:" + hex.EncodeToString(hash[:])
	return s.tokens.Delete(context.TODO(), key)
}

// GetDEK retrieves a cached DEK for the given user ID. Returns nil if not
// cached (e.g. session was not created via login).
func (s *InMemorySession) GetDEK(userID string) []byte {
	return s.deks.GetDEK(userID)
}

// StoreDEK caches a user's DEK for the session lifetime. Called after login
// when the server unwraps the DEK from the password-KEK.
func (s *InMemorySession) StoreDEK(userID string, dek []byte) {
	s.deks.StoreDEK(userID, dek)
}
