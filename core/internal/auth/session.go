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
	// StoreDEK caches the user's unwrapped DEK under this session's key.
	// The entry lives exactly as long as the session: Revoke and expiry
	// drop it together with the token (IMPLEMENTATION.md §1.3).
	StoreDEK(token string, dek []byte) error
	// GetDEK returns the DEK cached for this session's token, or nil.
	GetDEK(token string) []byte
}

// SessionHandler is a Session backed by a TokenStore and DEKCache.
// Tokens are opaque bearer tokens; the store holds the token's SHA-256
// hash, never the token value.
type SessionHandler struct {
	tokens TokenStore
	deks   DEKCache
	ttl    time.Duration
}

// NewSessionHandler returns a Session backed by the given stores with the
// given TTL.
func NewSessionHandler(tokens TokenStore, deks DEKCache, ttl time.Duration) *SessionHandler {
	return &SessionHandler{tokens: tokens, deks: deks, ttl: ttl}
}

// Mint generates a new session token, stores its hash, and returns the
// raw token string.
func (s *SessionHandler) Mint(userID string) (string, error) {
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
func (s *SessionHandler) Validate(token string) (string, error) {
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
		// Clean up the expired token together with its cached DEK.
		_ = s.tokens.Delete(context.TODO(), key)
		_ = s.deks.DeleteDEK(dekKey(token))
		return "", fmt.Errorf("session: token expired")
	}

	return userID, nil
}

// dekKey derives a DEK cache key from a bearer token. The DEK entry shares
// the token's hash, so token and key material are evicted as one unit.
func dekKey(token string) string {
	hash := sha256.Sum256([]byte(token))
	return "dek:" + hex.EncodeToString(hash[:])
}

// Revoke removes a session: its token record and its cached DEK go together.
func (s *SessionHandler) Revoke(token string) error {
	hash := sha256.Sum256([]byte(token))
	key := "session:" + hex.EncodeToString(hash[:])
	if err := s.tokens.Delete(context.TODO(), key); err != nil {
		return err
	}
	return s.deks.DeleteDEK(dekKey(token))
}

// GetDEK retrieves the DEK cached for this session's token. Returns nil if
// none is cached (e.g. the session was not created via login).
func (s *SessionHandler) GetDEK(token string) []byte {
	return s.deks.GetDEK(dekKey(token))
}

// StoreDEK caches the user's unwrapped DEK under this session's key.
// Called after login, when the server unwraps the DEK with the password-KEK.
func (s *SessionHandler) StoreDEK(token string, dek []byte) error {
	return s.deks.StoreDEK(dekKey(token), dek)
}
