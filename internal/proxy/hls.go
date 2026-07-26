package proxy

import (
	"context"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/nem-git/abcmovies/internal/proxy/parser"
	"github.com/nem-git/abcmovies/internal/stream"
)

// StrategyDeps holds shared dependencies for strategies.
type StrategyDeps struct {
	Fetcher Fetcher
	State   StateStore
}

// HLSStrategy handles HLS manifest rewriting.
type HLSStrategy struct {
	deps StrategyDeps
	p    parser.Parser
}

func (s *HLSStrategy) getParser() parser.Parser {
	if s.p != nil {
		return s.p
	}
	return parser.ManifestorParser{}
}

func (s *HLSStrategy) ServeManifest(ctx context.Context, w io.Writer, locator stream.Locator, meta *StreamMeta) (string, error) {
	body, _, err := s.deps.Fetcher.Fetch(ctx, locator.URL, locator.Headers, locator.Query)
	if err != nil {
		return "", err
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}

	upstreamBaseURL := resolveBaseURL(locator.URL)
	rewriteFn := s.buildRewriteFunc(meta, upstreamBaseURL)

	rewritten, err := s.getParser().Rewrite(data, rewriteFn)
	if err != nil {
		return "", err
	}

	w.Write(rewritten)

	return upstreamBaseURL, nil
}

func (s *HLSStrategy) buildRewriteFunc(meta *StreamMeta, upstreamBaseURL string) parser.RewriteFunc {
	return func(upstreamAbsoluteURL string) string {
		u, err := url.Parse(upstreamAbsoluteURL)
		if err != nil {
			return upstreamAbsoluteURL
		}
		base := filepath.Base(u.Path)
		ext := filepath.Ext(base)

		switch ext {
		case ".m3u8":
			// Sub-playlist (variant). Store state, return proxy URL.
			name := strings.TrimSuffix(base, ext)
			renditionKey := buildStateKey(meta.ProviderTag, meta.ContentKey, meta.Format, name)
			subMeta := *meta
			subMeta.UpstreamBaseURL = resolveBaseURL(upstreamAbsoluteURL)
			subMeta.ExpiresAt = time.Now().Add(5 * time.Minute)
			s.deps.State.Put(context.Background(), renditionKey, subMeta)
			return name + ".m3u8"

		case ".ts", ".m4s", ".mp4":
			// Segment. Extract rendition from path.
			rendition := extractRendition(u.Path, meta.Format)
			return rendition + "/" + base
		}

		return upstreamAbsoluteURL
	}
}

func (s *HLSStrategy) ServeSegment(ctx context.Context, w io.Writer, locator stream.Locator, segmentPath string) error {
	body, _, err := s.deps.Fetcher.Fetch(ctx, locator.URL, locator.Headers, locator.Query)
	if err != nil {
		return err
	}
	defer body.Close()

	io.Copy(w, body)
	return nil
}

// extractRendition tries to extract the rendition name from a URL path.
// e.g., "/content/movie/720p/segment.ts" -> "720p"
func extractRendition(path string, format string) string {
	dir := filepath.Dir(path)
	parts := strings.Split(dir, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "default"
}
