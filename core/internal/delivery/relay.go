package delivery

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"sync"
	"time"
)

// Relay is the engine-granted byte-stream mechanism (PLAN.md §3.6, §1.12): a
// session-scoped, expiring registry of tickets that resolve to media bytes.
// The engine grants a ticket per track; the fetch closure it registers carries
// the provider's stream URL and the auth_context needed to dial it. The token
// — never the auth_context — is what reaches a sink or frontend, so the
// provider's credentials stay engine-side (PLAN.md §6.2: track delivery auth
// is "engine-side only", never forwarded to sinks or frontends).
//
// The relay is transport-neutral: it hands back an io.Reader, and a serving
// layer mounts it behind whatever URL scheme it wants.
type Relay struct {
	mu      sync.Mutex
	tickets map[string]*ticket
	now     func() time.Time
}

type ticket struct {
	sessionID string
	trackID   string
	fetch     func(context.Context) (io.ReadCloser, error)
	expiry    time.Time
}

// NewRelay returns an empty relay.
func NewRelay() *Relay {
	return &Relay{tickets: map[string]*ticket{}, now: time.Now}
}

// Grant registers a byte source for one session's track and returns the opaque
// token that resolves to it. Tokens are random and unguessable; expiry bounds
// how long a granted URL stays valid (stream URLs expire in minutes-to-hours,
// PLAN.md §6.2).
func (r *Relay) Grant(sessionID, trackID string, fetch func(context.Context) (io.ReadCloser, error), ttl time.Duration) (string, error) {
	if sessionID == "" || trackID == "" || fetch == nil {
		return "", fmt.Errorf("relay: session, track, and fetch are required")
	}
	if ttl <= 0 {
		return "", fmt.Errorf("relay: grant TTL must be positive")
	}
	tok := newToken()
	r.mu.Lock()
	r.tickets[tok] = &ticket{
		sessionID: sessionID,
		trackID:   trackID,
		fetch:     fetch,
		expiry:    r.now().Add(ttl),
	}
	r.mu.Unlock()
	return tok, nil
}

// Open resolves a token to a byte reader. It validates the ticket is live
// (granted and not expired), and returns the reader plus a release func the
// caller must close/forget on abort. A consumed ticket stays valid until it
// expires or the session is revoked, so a frontend may re-pull a track.
func (r *Relay) Open(token string) (io.ReadCloser, func(), error) {
	r.mu.Lock()
	t, ok := r.tickets[token]
	if !ok {
		r.mu.Unlock()
		return nil, nil, fmt.Errorf("relay: unknown token")
	}
	if r.now().After(t.expiry) {
		r.mu.Unlock()
		return nil, nil, fmt.Errorf("relay: grant expired")
	}
	r.mu.Unlock()

	body, err := t.fetch(context.Background())
	if err != nil {
		return nil, nil, fmt.Errorf("relay: fetch: %w", err)
	}
	release := func() {}
	return body, release, nil
}

// TokenTrack returns the session and track a token belongs to, or ok=false.
func (r *Relay) TokenTrack(token string) (sessionID, trackID string, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tickets[token]
	if !ok {
		return "", "", false
	}
	return t.sessionID, t.trackID, true
}

// Revoke drops every ticket granted to a session (on finalize, abort, or
// revocation), closing their relays so bytes can no longer be pulled.
func (r *Relay) Revoke(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for tok, t := range r.tickets {
		if t.sessionID == sessionID {
			delete(r.tickets, tok)
		}
	}
}

// newToken returns a random 16-byte hex token.
func newToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("relay: rand: %v", err))
	}
	return hex.EncodeToString(b)
}
