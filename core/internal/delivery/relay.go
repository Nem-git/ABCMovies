package delivery

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
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
	token     string
	fetch     FetchFunc
	expiry    time.Time
}

// FetchFunc pulls one track's bytes from the provider. rangeHeader carries the
// caller's HTTP Range value ("" means the whole stream): a byte-addressable
// request forwards it to the provider so only the requested segment is pulled
// (PLAN.md §3.6 — byte-addressable sinks pull segments or byte ranges through
// relay URLs, and the engine fetches only what the sink asks for). A fetch
// that is not range-addressable may ignore it and return the full stream.
type FetchFunc func(ctx context.Context, rangeHeader string) (FetchResult, error)

// FetchResult is one provider response to a pull: the provider's own status
// and headers plus the body. The relay is a pipe, not a decoder — the provider
// answered 206 for a ranged pull (Content-Range and Content-Length included),
// and those are exactly what the serving layer re-emits, so a player seeking
// through a stream never sees a mangled range response.
type FetchResult struct {
	StatusCode int
	Header     http.Header
	Body       io.ReadCloser
}

// NewRelay returns an empty relay.
func NewRelay() *Relay {
	return &Relay{tickets: map[string]*ticket{}, now: time.Now}
}

// Grant registers a byte source for one session's track and returns the opaque
// token that resolves to it. Tokens are random and unguessable; expiry bounds
// how long a granted URL stays valid (stream URLs expire in minutes-to-hours,
// PLAN.md §6.2).
func (r *Relay) Grant(sessionID, trackID string, fetch FetchFunc, ttl time.Duration) (string, error) {
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
		token:     tok,
		fetch:     fetch,
		expiry:    r.now().Add(ttl),
	}
	r.mu.Unlock()
	return tok, nil
}

// OpenOption customizes one pull through a granted relay URL.
type OpenOption func(*openOptions)

type openOptions struct {
	rangeHeader string
}

// WithRange forwards an HTTP Range header to the provider on this pull, so a
// byte-addressable sink downloads only the segment it asked for. The
// provider's own response — status (206), Content-Range, Content-Length — is
// returned untouched.
func WithRange(v string) OpenOption {
	return func(o *openOptions) { o.rangeHeader = v }
}

// Open resolves a token to a provider response. It validates the ticket is
// live (granted and not expired), then pulls the bytes — forwarding any Range
// the caller requested. The pull runs under the caller's context: when the
// caller goes away (a browser tab closes, a seeking player gives up), the
// provider fetch is cancelled rather than left running to completion. A
// consumed ticket stays valid until it expires or the session is revoked, so
// a frontend may re-pull a track.
func (r *Relay) Open(ctx context.Context, token string, opts ...OpenOption) (FetchResult, error) {
	r.mu.Lock()
	t, ok := r.tickets[token]
	if !ok {
		r.mu.Unlock()
		return FetchResult{}, fmt.Errorf("relay: unknown token")
	}
	if r.now().After(t.expiry) {
		r.mu.Unlock()
		return FetchResult{}, fmt.Errorf("relay: grant expired")
	}
	r.mu.Unlock()

	var o openOptions
	for _, opt := range opts {
		opt(&o)
	}
	res, err := t.fetch(ctx, o.rangeHeader)
	if err != nil {
		return FetchResult{}, fmt.Errorf("relay: fetch: %w", err)
	}
	return res, nil
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

// SessionTicket is one staged relay ticket for a session: the track it serves
// and the opaque token that resolves to it. The token is what reaches a sink or
// frontend; the relay never exposes the fetch closure or the provider's
// auth_context (PLAN.md §6.2).
type SessionTicket struct {
	TrackID string
	Token   string
}

// SessionTickets lists every live ticket granted to a session, in grant order.
// It is the relay's read surface for a session's staged bytes — the Package
// pipeline step reads the per-track tokens to assemble the play manifest, so a
// manifest references exactly the tokens the sink staged, with no second
// granting path. A session with no grants (a download, or a play that staged
// nothing) yields an empty slice, never an error.
func (r *Relay) SessionTickets(sessionID string) []SessionTicket {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]SessionTicket, 0, len(r.tickets))
	for _, t := range r.tickets {
		if t.sessionID == sessionID {
			out = append(out, SessionTicket{TrackID: t.trackID, Token: t.token})
		}
	}
	return out
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
