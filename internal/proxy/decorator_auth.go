package proxy

import (
	"context"
	"io"
	"net/http"

	"github.com/nem-git/abcmovies/internal/stream"
)

// AuthDecorator injects auth headers into upstream requests.
type AuthDecorator struct {
	inner Strategy
	cfg   *AuthConfig
}

func (d *AuthDecorator) ServeManifest(ctx context.Context, w io.Writer, locator stream.Locator, meta *StreamMeta) (string, error) {
	if d.cfg.Headers != nil {
		if locator.Headers == nil {
			locator.Headers = make(http.Header)
		}
		for k, v := range d.cfg.Headers {
			locator.Headers.Set(k, v)
		}
	}
	return d.inner.ServeManifest(ctx, w, locator, meta)
}

func (d *AuthDecorator) ServeSegment(ctx context.Context, w io.Writer, locator stream.Locator, segmentPath string) error {
	if d.cfg.Headers != nil {
		if locator.Headers == nil {
			locator.Headers = make(http.Header)
		}
		for k, v := range d.cfg.Headers {
			locator.Headers.Set(k, v)
		}
	}
	return d.inner.ServeSegment(ctx, w, locator, segmentPath)
}
