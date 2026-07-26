package proxy

import (
	"context"
	"io"

	"github.com/nem-git/abcmovies/internal/stream"
)

// CachingDecorator adds caching to a strategy.
type CachingDecorator struct {
	inner Strategy
	cfg   *CacheConfig
}

func (d *CachingDecorator) ServeManifest(ctx context.Context, w io.Writer, locator stream.Locator, meta *StreamMeta) (string, error) {
	if !d.cfg.Manifests.Enabled {
		return d.inner.ServeManifest(ctx, w, locator, meta)
	}
	// TODO: implement manifest caching
	return d.inner.ServeManifest(ctx, w, locator, meta)
}

func (d *CachingDecorator) ServeSegment(ctx context.Context, w io.Writer, locator stream.Locator, segmentPath string) error {
	if !d.cfg.Segments.Enabled {
		return d.inner.ServeSegment(ctx, w, locator, segmentPath)
	}
	// TODO: implement segment caching
	return d.inner.ServeSegment(ctx, w, locator, segmentPath)
}
