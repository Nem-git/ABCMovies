package proxy

import (
	"context"
	"io"

	"github.com/nem-git/abcmovies/internal/stream"
)

// Strategy knows how to serve a specific stream format.
type Strategy interface {
	// ServeManifest fetches the upstream manifest, rewrites URLs,
	// stores state for segment resolution, and writes to w.
	// Returns the upstream base URL for state storage.
	ServeManifest(ctx context.Context, w io.Writer, locator stream.Locator, meta *StreamMeta) (upstreamBaseURL string, err error)

	// ServeSegment fetches an upstream segment and writes to w.
	ServeSegment(ctx context.Context, w io.Writer, locator stream.Locator, segmentPath string) error
}

// newStrategy creates a strategy based on format and config.
func newStrategy(format string, cfg *Config, deps Dependencies) Strategy {
	sdeps := StrategyDeps{Fetcher: deps.Fetcher, State: deps.State}

	var s Strategy
	switch cfg.Strategy {
	case "hls":
		s = &HLSStrategy{deps: sdeps}
	case "dash":
		s = &DASHStrategy{deps: sdeps}
	case "passthrough":
		s = &PassthroughStrategy{deps: sdeps}
	default: // "auto" — detect from format
		switch format {
		case "hls":
			s = &HLSStrategy{deps: sdeps}
		case "dash":
			s = &DASHStrategy{deps: sdeps}
		default:
			s = &PassthroughStrategy{deps: sdeps}
		}
	}
	// Apply decorators in reverse order (outermost first)
	for i := len(cfg.Decorators) - 1; i >= 0; i-- {
		switch cfg.Decorators[i] {
		case "cache":
			s = &CachingDecorator{inner: s, cfg: &cfg.Cache}
		case "auth":
			s = &AuthDecorator{inner: s, cfg: &cfg.Auth}
		case "drm":
			s = &DRMDecorator{inner: s, cfg: &cfg.DRM}
		}
	}
	return s
}
