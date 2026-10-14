package delivery

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nem-git/abcmovies/adapters/mediaserver"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

// MediaServerSink is the media-server sink (PLAN.md §6.4): a pull sink that
// exposes a play session as a standard adaptive-stream manifest (HLS) whose
// byte URLs point at the engine's relay, so any conformant player can open it
// with a per-session token and do its own language, subtitle, and quality
// selection and ABR. It never copies bytes inline: Deliver stages a relay
// ticket per track and a manifest ticket the player pulls.
type MediaServerSink struct {
	relay  *Relay
	opener *Opener
	ttl    time.Duration

	sessionID string
	tracks    []*corev1.Track

	mu            sync.Mutex
	tokensByTrack map[string]string
	manifestToken string
}

var _ Sink = (*MediaServerSink)(nil)

// Pushes reports that the engine does not drive byte delivery to a media-server
// sink: it stages a manifest and per-track relay tickets the player pulls, so a
// media-server session is not self-driving (PLAN.md §3.6, §6.4).
func (m *MediaServerSink) Pushes() bool { return false }

// MediaServerSinkFactory builds a MediaServerSink per play session.
type MediaServerSinkFactory struct {
	// Relay issues the granted tickets. Required.
	Relay *Relay
	// Opener produces clear bytes for relay tickets. Required.
	Opener *Opener
	// TTL bounds how long a granted relay URL stays valid; zero uses the
	// default grant lifespan.
	TTL time.Duration
}

var _ SinkFactory = (*MediaServerSinkFactory)(nil)

// defaultMediaServerTTL bounds a media-server grant; provider stream URLs
// expire in minutes-to-hours (PLAN.md §6.2), so grants are short-lived and the
// player re-fetches the manifest to refresh them.
const defaultMediaServerTTL = 30 * time.Minute

// NewSink prepares a MediaServerSink for the session.
func (f *MediaServerSinkFactory) NewSink(_ context.Context, s *Session, tracks []*corev1.Track) (Sink, error) {
	if f.Relay == nil {
		return nil, fmt.Errorf("media-server sink: no relay wired")
	}
	if f.Opener == nil {
		return nil, fmt.Errorf("media-server sink: no opener wired")
	}
	ttl := f.TTL
	if ttl <= 0 {
		ttl = defaultMediaServerTTL
	}
	return &MediaServerSink{
		relay: f.Relay, opener: f.Opener, ttl: ttl,
		sessionID: s.ID, tracks: tracks, tokensByTrack: map[string]string{},
	}, nil
}

// Deliver stages a relay ticket for one track so a player can pull its bytes
// through the manifest. A carried-in track (no location) stages nothing — the
// player reads it off the carrier track's delivery (WHOLE_MUX).
func (m *MediaServerSink) Deliver(ctx context.Context, s *Session, track *corev1.Track, _ io.Reader) (int64, error) {
	dl := track.GetDelivery()
	if dl == nil || len(dl.GetLocations()) == 0 {
		return 0, nil
	}
	tok, err := m.relay.Grant(m.sessionID, track.GetId(), m.trackFetch(s, track), m.ttl)
	if err != nil {
		return 0, err
	}
	m.mu.Lock()
	m.tokensByTrack[track.GetId()] = tok
	m.mu.Unlock()
	return 0, nil
}

// trackFetch returns the relay fetch closure for one track. Each pull notes
// activity on the session: a generic player never heartbeats, so pull activity
// is its liveness (PLAN.md §9.1 — the same rule downloads use).
func (m *MediaServerSink) trackFetch(s *Session, track *corev1.Track) FetchFunc {
	return func(ctx context.Context, rangeHeader string) (FetchResult, error) {
		s.NoteActivity()
		return m.opener.Fetch(s, track)(ctx, rangeHeader)
	}
}

// ManifestToken returns the relay token serving the session's HLS manifest,
// granted lazily on first call (after every track has staged). The manifest
// references the per-track relay URLs, so a player opens this URL and pulls
// the renditions it selects.
func (m *MediaServerSink) ManifestToken() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.manifestToken != "" {
		return m.manifestToken, nil
	}
	tok, err := m.relay.Grant(m.sessionID, ".m3u8", m.manifestFetch, m.ttl)
	if err != nil {
		return "", err
	}
	m.manifestToken = tok
	return tok, nil
}

// TrackToken returns the staged relay token for a track, ok=false if the track
// has not been staged yet.
func (m *MediaServerSink) TrackToken(trackID string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tok, ok := m.tokensByTrack[trackID]
	return tok, ok
}

// manifestFetch renders the HLS master playlist from the staged tracks.
func (m *MediaServerSink) manifestFetch(_ context.Context, _ string) (FetchResult, error) {
	m.mu.Lock()
	tokens := make(map[string]string, len(m.tokensByTrack))
	for k, v := range m.tokensByTrack {
		tokens[k] = v
	}
	tracks := m.tracks
	m.mu.Unlock()

	urlFor := func(trackID string) (string, error) {
		tok, ok := tokens[trackID]
		if !ok {
			return "", fmt.Errorf("track %q not staged", trackID)
		}
		return RelayPrefix + tok, nil
	}
	manifest, err := mediaserver.BuildHLS(tracks, urlFor)
	if err != nil {
		return FetchResult{}, err
	}
	return FetchResult{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/vnd.apple.mpegurl"}},
		Body:       io.NopCloser(strings.NewReader(manifest)),
	}, nil
}

// Finalize revokes the session's relay grants (play is done).
func (m *MediaServerSink) Finalize(_ context.Context, _ *Session) error {
	m.relay.Revoke(m.sessionID)
	return nil
}

// Abort revokes the session's relay grants (play ended early).
func (m *MediaServerSink) Abort(_ context.Context, _ *Session) {
	m.relay.Revoke(m.sessionID)
}
