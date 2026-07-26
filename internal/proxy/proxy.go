package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/nem-git/abcmovies/internal/stream"
)

// CacheBucketConfig configures caching for a category of content.
type CacheBucketConfig struct {
	Enabled bool          `yaml:"enabled"`
	TTL     time.Duration `yaml:"ttl"`
	MaxSize int64         `yaml:"max_size"`
}

// CacheConfig holds caching settings for manifests and segments.
type CacheConfig struct {
	Manifests CacheBucketConfig `yaml:"manifests"`
	Segments  CacheBucketConfig `yaml:"segments"`
}

// AuthConfig holds auth injection settings.
type AuthConfig struct {
	Headers map[string]string `yaml:"headers"`
}

// DRMConfig holds DRM settings (stub for future use).
type DRMConfig struct{}

// Config holds per-provider proxy configuration.
type Config struct {
	Strategy  string       `yaml:"strategy"` // "auto", "hls", "dash", "passthrough"
	Decorators []string    `yaml:"decorators"`
	Cache     CacheConfig  `yaml:"cache"`
	Auth      AuthConfig   `yaml:"auth"`
	DRM       DRMConfig    `yaml:"drm"`
}

// Dependencies holds the components needed by the Proxy.
type Dependencies struct {
	Fetcher Fetcher
	State   StateStore
	Configs map[string]*Config // provider tag -> proxy config
}

// Proxy coordinates strategies, state, and fetching for stream proxying.
type Proxy struct {
	deps Dependencies
}

func New(deps Dependencies) *Proxy {
	return &Proxy{deps: deps}
}

// buildStateKey builds a deterministic state key from metadata.
func buildStateKey(tag, contentKey, format, rendition string) string {
	return tag + ":" + contentKey + ":" + format + ":" + rendition
}

// BuildContentKey builds a content key from type and ID parts.
func BuildContentKey(contentType string, ids ...string) string {
	return contentType + ":" + strings.Join(ids, ":")
}

// ResolveBaseURL extracts the base URL (directory) from a full URL.
func ResolveBaseURL(rawURL string) string {
	return resolveBaseURL(rawURL)
}

// resolveBaseURL is the unexported version for internal use.
func resolveBaseURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	dir := filepath.Dir(u.Path)
	if dir == "/" {
		u.Path = "/"
	} else {
		u.Path = dir + "/"
	}
	return u.String()
}

// copyHeader copies selected headers from src to dst.
func copyHeader(dst, src http.Header, keys ...string) {
	for _, k := range keys {
		if v := src.Get(k); v != "" {
			dst.Set(k, v)
		}
	}
}

// ManifestContentType returns the Content-Type for a manifest given the format.
func ManifestContentType(format string) string {
	switch format {
	case "hls":
		return "application/vnd.apple.mpegurl"
	case "dash":
		return "application/dash+xml"
	case "mp4":
		return "video/mp4"
	default:
		return "application/octet-stream"
	}
}

// SegmentContentType returns the Content-Type for a segment given the format and filename.
func SegmentContentType(format, segment string) string {
	switch format {
	case "hls":
		if strings.HasSuffix(segment, ".ts") {
			return "video/mp2t"
		}
		return "video/mp4"
	case "dash":
		return "video/mp4"
	default:
		return "application/octet-stream"
	}
}

// ServeManifest fetches the upstream manifest, applies the strategy, and returns the
// rewritten content as a reader along with the content type.
func (p *Proxy) ServeManifest(ctx context.Context, tag, contentKey, format, file string, locator stream.Locator) (io.ReadCloser, string, error) {
	cfg := p.deps.Configs[tag]

	meta := StreamMeta{
		ProviderTag:    tag,
		ContentKey:     contentKey,
		StreamFile:     file,
		Format:         format,
		Headers:        locator.Headers,
		Query:          locator.Query,
		EncodingFormat: locator.EncodingFormat,
	}

	strategy := newStrategy(format, cfg, p.deps)

	var buf bytes.Buffer
	upstreamBaseURL, err := strategy.ServeManifest(ctx, &buf, locator, &meta)
	if err != nil {
		return nil, "", err
	}

	// Store state for segment resolution
	stateKey := buildStateKey(tag, contentKey, format, filepath.Base(file))
	meta.UpstreamBaseURL = upstreamBaseURL
	meta.ExpiresAt = time.Now().Add(5 * time.Minute)
	_ = p.deps.State.Put(ctx, stateKey, meta)

	return io.NopCloser(&buf), ManifestContentType(format), nil
}

// ServeSegment fetches a segment from upstream and streams it through a pipe.
// The caller receives a reader that streams the segment data.
func (p *Proxy) ServeSegment(ctx context.Context, tag, contentKey, format, rendition, segment string) (io.ReadCloser, string, error) {
	stateKey := buildStateKey(tag, contentKey, format, rendition)
	meta, found, err := p.deps.State.Get(ctx, stateKey)
	if err != nil {
		return nil, "", err
	}
	if !found {
		return nil, "", &httpError{code: 404, msg: "segment state not found"}
	}

	cfg := p.deps.Configs[tag]

	locator := stream.Locator{
		URL:            meta.UpstreamBaseURL + segment,
		Headers:        meta.Headers,
		Query:          meta.Query,
		EncodingFormat: meta.EncodingFormat,
	}

	strategy := newStrategy(format, cfg, p.deps)

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		if err := strategy.ServeSegment(ctx, pw, locator, segment); err != nil {
			pw.CloseWithError(err)
		}
	}()

	return pr, SegmentContentType(format, segment), nil
}

// httpError is a simple HTTP error.
type httpError struct {
	code int
	msg  string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("http %d: %s", e.code, e.msg)
}

// StatusCode returns the HTTP status code.
func (e *httpError) StatusCode() int {
	return e.code
}
