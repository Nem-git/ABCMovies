package proxy

import (
	"context"
	"io"

	"github.com/nem-git/abcmovies/internal/stream"
)

// PassthroughStrategy fetches upstream and streams it directly.
type PassthroughStrategy struct {
	deps StrategyDeps
}

func (s *PassthroughStrategy) ServeManifest(ctx context.Context, w io.Writer, locator stream.Locator, meta *StreamMeta) (string, error) {
	body, _, err := s.deps.Fetcher.Fetch(ctx, locator.URL, locator.Headers, locator.Query)
	if err != nil {
		return "", err
	}
	defer body.Close()

	io.Copy(w, body)
	return ResolveBaseURL(locator.URL), nil
}

func (s *PassthroughStrategy) ServeSegment(ctx context.Context, w io.Writer, locator stream.Locator, segmentPath string) error {
	body, _, err := s.deps.Fetcher.Fetch(ctx, locator.URL, locator.Headers, locator.Query)
	if err != nil {
		return err
	}
	defer body.Close()

	io.Copy(w, body)
	return nil
}
