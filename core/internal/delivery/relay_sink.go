package delivery

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

// defaultGrantTTL bounds a relay grant; provider stream URLs expire in
// minutes-to-hours (PLAN.md §6.2), so grants are short-lived and re-granted.
const defaultGrantTTL = 30 * time.Minute

// RelaySink is the unified pull sink (PLAN.md §6.4): it stages one relay
// ticket per located track, so any conformant player — a user's device or a
// media server — can open them and do its own language, subtitle, and quality
// selection and ABR. It never copies bytes inline and never knows about
// packaging: the engine's Package step (not the sink) assembles the staged
// tokens into a manifest when the player requests one. The sink and the menu
// format are orthogonal.
//
// This is the collapsed successor of DeviceSink and MediaServerSink, which
// were functionally identical (both staged per-track relay tokens) once the
// media-server sink's manifest generation moved to the engine's Package step.
type RelaySink struct {
	relay         *Relay
	opener        *Opener
	ttl           time.Duration
	sessionID     string
	mu            sync.Mutex
	tokensByTrack map[string]string
}

var _ Sink = (*RelaySink)(nil)

// Pushes reports that the engine does not drive byte delivery to a relay sink:
// it stages a relay ticket the far side pulls through, so a relay download is
// not self-driving (PLAN.md §3.6).
func (d *RelaySink) Pushes() bool { return false }

// RelaySinkFactory builds a RelaySink per play session over a relay.
type RelaySinkFactory struct {
	// Relay issues the granted tickets. Required.
	Relay *Relay
	// Opener produces clear bytes for relay tickets. Required.
	Opener *Opener
	// GrantTTL bounds how long a granted relay URL stays valid; zero uses the
	// default grant lifespan.
	GrantTTL time.Duration
}

var _ SinkFactory = (*RelaySinkFactory)(nil)

// NewSink prepares a RelaySink for the session.
func (f *RelaySinkFactory) NewSink(_ context.Context, s *Session, _ []*corev1.Track) (Sink, error) {
	if f.Relay == nil {
		return nil, fmt.Errorf("relay sink: no relay wired")
	}
	if f.Opener == nil {
		return nil, fmt.Errorf("relay sink: no opener wired")
	}
	ttl := f.GrantTTL
	if ttl <= 0 {
		ttl = defaultGrantTTL
	}
	return &RelaySink{relay: f.Relay, opener: f.Opener, ttl: ttl, sessionID: s.ID, tokensByTrack: map[string]string{}}, nil
}

// Deliver stages a relay ticket for one track so a client can pull its bytes.
// The track's clear bytes are produced by the opener — DRM-encrypted tracks
// are acquired and decrypted through the content-key cache before the relay
// serves them. The track's provider location and auth_context are captured
// into the ticket's fetch closure — the pull attaches credentials engine-side,
// on demand. A carried-in track (no location) stages nothing; the player reads
// it off the carrier track's delivery (WHOLE_MUX).
func (d *RelaySink) Deliver(_ context.Context, s *Session, track *corev1.Track, _ io.Reader) (int64, error) {
	dl := track.GetDelivery()
	if dl == nil {
		return 0, fmt.Errorf("relay sink: track %q has no delivery", track.GetId())
	}
	if len(dl.GetLocations()) == 0 {
		return 0, fmt.Errorf("relay sink: track %q has no locations to relay", track.GetId())
	}
	tok, err := d.relay.Grant(d.sessionID, track.GetId(), d.trackFetch(s, track), d.ttl)
	if err != nil {
		return 0, err
	}
	d.mu.Lock()
	d.tokensByTrack[track.GetId()] = tok
	d.mu.Unlock()
	// Nothing was copied inline; the deliverable is staged for pull.
	return 0, nil
}

// trackFetch returns the relay fetch closure for one track. Each pull notes
// activity on the session: a generic player never heartbeats, so pull activity
// is its liveness (PLAN.md §9.1 — the same rule downloads use).
func (d *RelaySink) trackFetch(s *Session, track *corev1.Track) FetchFunc {
	return func(ctx context.Context, rangeHeader string) (FetchResult, error) {
		s.NoteActivity()
		return d.opener.Fetch(s, track)(ctx, rangeHeader)
	}
}

// RelayToken returns the staged relay token for a track, ok=false if the track
// has not been staged yet.
func (d *RelaySink) RelayToken(trackID string) (string, bool) {
	d.mu.Lock()
	tok, ok := d.tokensByTrack[trackID]
	d.mu.Unlock()
	return tok, ok
}

// Finalize revokes the session's relay grants (play is done).
func (d *RelaySink) Finalize(_ context.Context, _ *Session) error {
	d.relay.Revoke(d.sessionID)
	return nil
}

// Abort revokes the session's relay grants (play ended early).
func (d *RelaySink) Abort(_ context.Context, _ *Session) {
	d.relay.Revoke(d.sessionID)
}
