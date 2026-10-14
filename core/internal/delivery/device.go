package delivery

import (
	"context"
	"fmt"
	"io"
	"sync"
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
	opener        *Opener
	ttl           time.Duration
	sessionID     string
	mu            sync.Mutex
	tokensByTrack map[string]string
}

var _ Sink = (*DeviceSink)(nil)

// Pushes reports that the engine does not drive byte delivery to a device
// sink: it stages a relay ticket the far side pulls through, so a device
// download is not self-driving (PLAN.md §3.6).
func (d *DeviceSink) Pushes() bool { return false }

// DeviceSinkFactory builds a DeviceSink per play session over a relay.
type DeviceSinkFactory struct {
	// Relay issues the granted tickets. Required.
	Relay *Relay
	// Opener produces clear bytes for relay tickets. Required.
	Opener *Opener
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
	if f.Opener == nil {
		return nil, fmt.Errorf("device sink: no opener wired")
	}
	ttl := f.GrantTTL
	if ttl <= 0 {
		ttl = defaultGrantTTL
	}
	return &DeviceSink{relay: f.Relay, opener: f.Opener, ttl: ttl, sessionID: s.ID, tokensByTrack: map[string]string{}}, nil
}

// Deliver stages a relay ticket for one track so a client can pull its bytes.
// The track's clear bytes are produced by the opener — DRM-encrypted tracks
// are acquired and decrypted through the content-key cache before the relay
// serves them. The track's provider location and auth_context are captured
// into the ticket's fetch closure — the pull attaches credentials engine-side,
// on demand.
func (d *DeviceSink) Deliver(_ context.Context, s *Session, track *corev1.Track, _ io.Reader) (int64, error) {
	dl := track.GetDelivery()
	if dl == nil {
		return 0, fmt.Errorf("device sink: track %q has no delivery", track.GetId())
	}
	if len(dl.GetLocations()) == 0 {
		return 0, fmt.Errorf("device sink: track %q has no locations to relay", track.GetId())
	}
	tok, err := d.relay.Grant(d.sessionID, track.GetId(), d.opener.Fetch(s, track), d.ttl)
	if err != nil {
		return 0, err
	}
	d.mu.Lock()
	d.tokensByTrack[track.GetId()] = tok
	d.mu.Unlock()
	// Nothing was copied inline; the deliverable is staged for pull.
	return 0, nil
}

// RelayToken returns the staged relay token for a track, ok=false if the track
// has not been staged yet.
func (d *DeviceSink) RelayToken(trackID string) (string, bool) {
	d.mu.Lock()
	tok, ok := d.tokensByTrack[trackID]
	d.mu.Unlock()
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
