package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/Nem-git/ABCMovies/internal/core/internal/connector/source/jellyfin"
)

// Session credential cache and playback tickets.
//
// Both live in process memory only. Nothing here is ever written to the
// store, logged, or returned to browsers. This is the single place that
// changes when DEC-003 (Build 2) decides encrypted at-rest credential
// custody: the cache's storage moves behind this seam, the rest of the code
// does not.
//
// SessionState bundles the credential cache and the ticket store so the
// source and library services share one in-memory session state (wired in
// core.go). Nothing in it is per-source by design: the cache is per user
// (see ADR 0005) and tickets are per actor+title; per-source eviction for
// disconnect is done wholesale via flushUserCreds/purgeTicketsForSource.
const (
	// sessionCredTTL bounds how long source credentials linger in memory
	// without a successful use. Sliding: refreshed on every read.
	sessionCredTTL = 12 * time.Hour
	// ticketIdleTTL bounds how long a minted playback ticket stays valid
	// without a successful resolve. Sliding: every resolve refreshes it, so
	// the ticket lives as long as the video is actively being fetched (the
	// browser re-issues Range requests throughout playback) but dies
	// ticketIdleTTL after the last fetch.
	ticketIdleTTL = 30 * time.Minute
)

// SessionState is the shared, in-memory session state of the core services
// (credentials + tickets). It is constructed once per core in core.go and
// passed to both the source and library services.
type SessionState struct {
	creds   *sessionCredStore
	tickets *ticketStore
}

// NewSessionState returns a fresh, empty SessionState.
func NewSessionState() *SessionState {
	return &SessionState{creds: newSessionCredStore(), tickets: newTicketStore()}
}

// flushUserCreds evicts every cached credential for userID.
func (s *SessionState) flushUserCreds(userID string) {
	s.creds.Flush(userID)
}

// purgeTicketsForSource revokes all minted tickets scoped to sourceID.
// In-flight responses already streaming to a browser cannot be recalled;
// every subsequent request (incl. seeking) is refused.
func (s *SessionState) purgeTicketsForSource(sourceID string) int {
	return s.tickets.purgeSource(sourceID)
}

type cachedCreds struct {
	creds     jellyfin.Credentials
	expiresAt time.Time
}

type sessionCredStore struct {
	mu sync.Mutex
	m  map[string]cachedCreds
}

func newSessionCredStore() *sessionCredStore { return &sessionCredStore{m: map[string]cachedCreds{}} }

func (s *sessionCredStore) Get(userID string) (jellyfin.Credentials, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.m[userID]
	if !ok {
		return jellyfin.Credentials{}, false
	}
	now := time.Now()
	if now.After(c.expiresAt) {
		delete(s.m, userID)
		return jellyfin.Credentials{}, false
	}
	c.expiresAt = now.Add(sessionCredTTL) // sliding expiry
	s.m[userID] = c
	return c.creds, true
}

func (s *sessionCredStore) Put(userID string, creds jellyfin.Credentials) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[userID] = cachedCreds{creds: creds, expiresAt: time.Now().Add(sessionCredTTL)}
}

// Flush evicts the cached credentials for userID (if any).
func (s *sessionCredStore) Flush(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, userID)
}

type playTicket struct {
	actorUserID   string
	titleID       string
	sourceID      string
	streamURL     string
	authorization string
	expiresAt     time.Time
}

type ticketStore struct {
	mu sync.Mutex
	m  map[string]playTicket
	// now supplies the current time; tests override it to exercise expiry.
	now func() time.Time
}

func newTicketStore() *ticketStore {
	return &ticketStore{m: map[string]playTicket{}, now: time.Now}
}

var errTicketNotFound = errors.New("ticket not found or expired")

func (s *ticketStore) mint(t playTicket) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	id := hex.EncodeToString(buf)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeLocked(s.now())
	s.m[id] = t
	return id, nil
}

func (s *ticketStore) resolve(actorUserID, ticket string) (playTicket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeLocked(s.now())
	t, ok := s.m[ticket]
	if !ok || t.actorUserID != actorUserID {
		// Uniform error: do not reveal whether the ticket exists.
		return playTicket{}, errTicketNotFound
	}
	// Sliding expiry: each successful resolve refreshes the idle lifetime.
	t.expiresAt = s.now().Add(ticketIdleTTL)
	s.m[ticket] = t
	return t, nil
}

// purgeSource drops every ticket scoped to sourceID and returns how many
// were dropped. Expired tickets are dropped first so they are never counted.
func (s *ticketStore) purgeSource(sourceID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeLocked(s.now())
	n := 0
	for id, t := range s.m {
		if t.sourceID == sourceID {
			delete(s.m, id)
			n++
		}
	}
	return n
}

func (s *ticketStore) purgeLocked(now time.Time) {
	for k, v := range s.m {
		if now.After(v.expiresAt) {
			delete(s.m, k)
		}
	}
}
