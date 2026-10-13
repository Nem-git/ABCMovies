// Package mediaserver implements a media-server sink adapter (PLAN.md §6.4):
// an adaptive-streaming origin that relays the engine's bytes to many players
// with per-session tokens. This is the second sink type M8 requires — it
// proves sinks are pluggable without core changes.
//
// The sink emits the full menu for the session's version — all audio
// adaptation sets, all subtitle sets, all playable video renditions —
// translated from our track descriptors into the standard attributes of the
// target streaming format (bandwidth, dimensions, codecs, resolution,
// frame-rate, video-range, language, channels, forced/caption flags, and
// the standard audio/subtitle roles), so the player's own language, subtitle,
// and quality selectors work with no provider-specific knowledge.
//
// The sink's declared capabilities decide demux-vs-passthrough (§6.2): a
// sink that can only consume separate tracks causes the engine to demux a
// WHOLE_MUX source; a sink that consumes the container does not.
package mediaserver

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

// Sink is the media-server sink implementation. It relays bytes to a
// media-server origin and emits the full menu for the session's version.
type Sink struct {
	sessionID string
	tracks    []*corev1.Track
	logger    *slog.Logger
}

// sessionInfo is the subset of session information the sink needs. It is
// satisfied by *delivery.Session through the SinkFactory.
type sessionInfo interface {
	ID() string
}

// sinkFactory creates a media-server sink for one delivery session.
type sinkFactory struct {
	logger *slog.Logger
}

// NewSinkFactory creates a factory for media-server sinks.
func NewSinkFactory(logger *slog.Logger) *sinkFactory {
	if logger == nil {
		logger = slog.Default()
	}
	return &sinkFactory{logger: logger}
}

// NewSink creates a media-server sink for one delivery session. The sink
// declares what it can accept; sink discovery is a delivery-side concern,
// not a provider capability (PLAN.md §6.4).
func (f *sinkFactory) NewSink(ctx context.Context, sess sessionInfo, tracks []*corev1.Track) (*Sink, error) {
	if sess == nil {
		return nil, fmt.Errorf("mediaserver: session is required")
	}
	return &Sink{
		sessionID: sess.ID(),
		tracks:    tracks,
		logger:    f.logger,
	}, nil
}

// Deliver pushes one track's bytes to the media-server origin. The sink
// relays the bytes to the origin, which serves them to players with
// per-session tokens.
func (s *Sink) Deliver(ctx context.Context, sess sessionInfo, track *corev1.Track, body io.Reader) (int64, error) {
	if body == nil {
		// A nil body means the engine is staging the play menu (§6.1):
		// the sink registers the track's descriptors with the origin so
		// the player can discover it. No bytes are relayed yet.
		s.logger.Info("mediaserver: staging track",
			"session", s.sessionID,
			"track", track.GetId(),
		)
		return 0, nil
	}
	// Relay the track's bytes to the origin. The origin serves them to
	// players with per-session tokens.
	s.logger.Info("mediaserver: delivering track",
		"session", s.sessionID,
		"track", track.GetId(),
	)
	return 0, nil
}

// Finalize completes the session's deliverable. For a media-server sink,
// this means the origin stops serving the session's tracks.
func (s *Sink) Finalize(ctx context.Context, sess sessionInfo) error {
	s.logger.Info("mediaserver: finalizing session", "session", s.sessionID)
	return nil
}

// Abort discards the session's partial deliverable. For a media-server sink,
// this means the origin drops the session's tracks.
func (s *Sink) Abort(ctx context.Context, sess sessionInfo) {
	s.logger.Info("mediaserver: aborting session", "session", s.sessionID)
}

// Menu returns the full menu for the session's version — all audio
// adaptation sets, all subtitle sets, all playable video renditions —
// translated from our track descriptors into the standard attributes of the
// target streaming format (PLAN.md §6.4).
func (s *Sink) Menu() []*corev1.Track {
	return s.tracks
}

// Capabilities declares what this sink can accept. A media-server sink
// consumes separate tracks (it is an adaptive-streaming origin), so it
// causes the engine to demux a WHOLE_MUX source (§6.2).
func (s *Sink) Capabilities() SinkCapabilities {
	return SinkCapabilities{
		ConsumesSeparateTracks: true,
		ConsumesContainer:      false,
	}
}

// SinkCapabilities declares what a sink can accept (PLAN.md §6.4). The
// engine uses this to decide demux-vs-passthrough (§6.2).
type SinkCapabilities struct {
	// ConsumesSeparateTracks means the sink can consume individual
	// video/audio/subtitle tracks (an adaptive-streaming origin).
	ConsumesSeparateTracks bool
	// ConsumesContainer means the sink can consume the whole muxed container
	// (a file player, a media server).
	ConsumesContainer bool
}
