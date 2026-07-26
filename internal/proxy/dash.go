package proxy

import (
	"context"
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/nem-git/abcmovies/internal/proxy/parser"
	"github.com/nem-git/abcmovies/internal/stream"
)

// DASHStrategy handles DASH manifest rewriting.
type DASHStrategy struct {
	deps StrategyDeps
	p    parser.Parser
}

func (s *DASHStrategy) getParser() parser.Parser {
	if s.p != nil {
		return s.p
	}
	return parser.ManifestorParser{}
}

func (s *DASHStrategy) ServeManifest(ctx context.Context, w io.Writer, locator stream.Locator, meta *StreamMeta) (string, error) {
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
	rewriteFn := s.buildRewriteFunc(meta)

	rewritten, err := s.getParser().Rewrite(data, rewriteFn)
	if err != nil {
		return "", err
	}

	w.Write(rewritten)

	return upstreamBaseURL, nil
}

func (s *DASHStrategy) buildRewriteFunc(meta *StreamMeta) parser.RewriteFunc {
	return func(upstreamAbsoluteURL string) string {
		u, err := url.Parse(upstreamAbsoluteURL)
		if err != nil {
			return upstreamAbsoluteURL
		}
		base := filepath.Base(u.Path)

		// DASH segments and init segments
		representation := extractRepresentation(u.Path)
		if representation != "" {
			return representation + "/" + base
		}

		return upstreamAbsoluteURL
	}
}

func (s *DASHStrategy) ServeSegment(ctx context.Context, w io.Writer, locator stream.Locator, segmentPath string) error {
	body, _, err := s.deps.Fetcher.Fetch(ctx, locator.URL, locator.Headers, locator.Query)
	if err != nil {
		return err
	}
	defer body.Close()

	io.Copy(w, body)
	return nil
}

// extractRepresentation tries to extract the representation ID from a DASH URL path.
// e.g., "/content/movie/v1/init.mp4" -> "v1"
func extractRepresentation(path string) string {
	dir := filepath.Dir(path)
	parts := strings.Split(dir, "/")
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		// Skip common non-representation directories
		if last != "." && last != "/" && last != "content" {
			return last
		}
	}
	return ""
}
