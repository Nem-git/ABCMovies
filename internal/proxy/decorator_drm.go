package proxy

import (
	"context"
	"io"

	"github.com/nem-git/abcmovies/internal/stream"
)

// DRMDecorator is a stub for future DRM processing.
type DRMDecorator struct {
	inner Strategy
	cfg   *DRMConfig
}

func (d *DRMDecorator) ServeManifest(ctx context.Context, w io.Writer, locator stream.Locator, meta *StreamMeta) (string, error) {
	return d.inner.ServeManifest(ctx, w, locator, meta)
}

func (d *DRMDecorator) ServeSegment(ctx context.Context, w io.Writer, locator stream.Locator, segmentPath string) error {
	return d.inner.ServeSegment(ctx, w, locator, segmentPath)
}
