package delivery

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

// DeviceSink is the user's device sink (TECHNICAL-DECISIONS.md §1.13): the
// built-in sink with no config, whose bytes are streamed to a client via the
// engine-granted relay (PLAN.md §3.6). It never copies bytes inline — instead
// each track's Deliver stages a relay ticket that a frontend or consumer pulls
// on demand. The provider's auth_context stays engine-side inside the ticket's
// fetch closure; only the opaque token reaches the client.
type DeviceSink struct {
	relay         *Relay
	ttl           time.Duration
	sessionID     string
	tokens        []string
	tokensByTrack map[string]string
}

var _ Sink = (*DeviceSink)(nil)

// DeviceSinkFactory builds a DeviceSink per play session over a relay.
type DeviceSinkFactory struct {
	// Relay issues the granted tickets. Required.
	Relay *Relay
	// GrantTTL bounds how long a granted relay URL stays valid; zero uses the
	// default grant lifespan.
	GrantTTL time.Duration
}

var _ SinkFactory = (*DeviceSinkFactory)(nil)

// defaultGrantTTL bounds a relay grant; provider stream URLs expire in
// minutes-to-hours (PLAN.md §6.2), so grants are short-lived and re-granted.
const defaultGrantTTL = 30 * time.Minute

// NewSink prepares a DeviceSink for the session.
func (f *DeviceSinkFactory) NewSink(_ context.Context, s *Session, _ []*corev1.Track) (Sink, error) {
	if f.Relay == nil {
		return nil, fmt.Errorf("device sink: no relay wired")
	}
	ttl := f.GrantTTL
	if ttl <= 0 {
		ttl = defaultGrantTTL
	}
	return &DeviceSink{relay: f.Relay, ttl: ttl, sessionID: s.ID, tokensByTrack: map[string]string{}}, nil
}

// Deliver stages a relay ticket for one track so a client can pull its bytes.
// The track's provider location and auth_context are captured into the ticket's
// fetch closure — the pull attaches credentials engine-side, on demand.
func (d *DeviceSink) Deliver(_ context.Context, _ *Session, track *corev1.Track, _ io.Reader) (int64, error) {
	dl := track.GetDelivery()
	if dl == nil {
		return 0, fmt.Errorf("device sink: track %q has no delivery", track.GetId())
	}
	if len(dl.GetLocations()) == 0 {
		return 0, fmt.Errorf("device sink: track %q has no locations to relay", track.GetId())
	}
	location := dl.GetLocations()[0]
	auth := dl.GetAuthContext()
	tok, err := d.relay.Grant(d.sessionID, track.GetId(), fetchFrom(location, auth), d.ttl)
	if err != nil {
		return 0, err
	}
	d.tokens = append(d.tokens, tok)
	d.tokensByTrack[track.GetId()] = tok
	// Nothing was copied inline; the deliverable is staged for pull.
	return 0, nil
}

// RelayToken returns the staged relay token for a track, ok=false if the track
// has not been staged yet.
func (d *DeviceSink) RelayToken(trackID string) (string, bool) {
	tok, ok := d.tokensByTrack[trackID]
	return tok, ok
}

// Finalize revokes the session's relay grants (play is done).
func (d *DeviceSink) Finalize(_ context.Context, _ *Session) error {
	d.relay.Revoke(d.sessionID)
	return nil
}

// Abort revokes the session's relay grants (play ended early).
func (d *DeviceSink) Abort(_ context.Context, _ *Session) {
	d.relay.Revoke(d.sessionID)
}

// fetchFrom returns a fetch that pulls bytes from a provider stream URL,
// attaching the engine-side auth_context as an Authorization header and
// forwarding any request Range so providers that support it answer a partial
// (206) response. The provider's status and headers ride back in the result.
func fetchFrom(location, authContext string) FetchFunc {
	return func(ctx context.Context, rangeHeader string) (FetchResult, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
		if err != nil {
			return FetchResult{}, err
		}
		if authContext != "" {
			req.Header.Set("Authorization", authContext)
		}
		if rangeHeader != "" {
			req.Header.Set("Range", rangeHeader)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return FetchResult{}, err
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			_ = resp.Body.Close()
			return FetchResult{}, fmt.Errorf("device sink: provider returned %s", resp.Status)
		}
		return FetchResult{StatusCode: resp.StatusCode, Header: resp.Header.Clone(), Body: resp.Body}, nil
	}
}
