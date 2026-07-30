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
	Strategy   string      `yaml:"strategy"` // "auto", "hls", "dash", "passthrough"
	Decorators []string    `yaml:"decorators"`
	Cache      CacheConfig `yaml:"cache"`
	Auth       AuthConfig  `yaml:"auth"`
	DRM        DRMConfig   `yaml:"drm"`
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
func (p *Proxy) ServeManifest(ctx context.Context, tag, contentKey, format, file string, locator stream.Locator, proxyBaseURL string) (io.ReadCloser, string, error) {
	cfg := p.deps.Configs[tag]

	meta := StreamMeta{
		ProviderTag:    tag,
		ContentKey:     contentKey,
		StreamFile:     file,
		Format:         format,
		Headers:        locator.Headers,
		Query:          locator.Query,
		EncodingFormat: locator.EncodingFormat,
		ProxyBaseURL:   proxyBaseURL,
	}

	strategy := newStrategy(format, cfg, p.deps)

	var buf bytes.Buffer
	upstreamBaseURL, err := strategy.ServeManifest(ctx, &buf, locator, &meta)
	if err != nil {
		return nil, "", err
	}

	// Store state for legacy segment resolution (used by passthrough)
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

	baseURL, err := url.Parse(meta.UpstreamBaseURL)
	if err != nil {
		return nil, "", &httpError{code: 500, msg: "invalid upstream base URL"}
	}

	locator := stream.Locator{
		URL:            baseURL.JoinPath(segment).String(),
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

// ServeHLSSubPlaylist fetches an upstream HLS sub-playlist, rewrites it, and returns it.
func (p *Proxy) ServeHLSSubPlaylist(ctx context.Context, tag, contentKey, format, stateKeyType, stateID string) (io.ReadCloser, string, error) {
	stateKey := hlsPlaylistStateKey(tag, contentKey, stateKeyType, stateID)
	meta, found, err := p.deps.State.Get(ctx, stateKey)
	if err != nil {
		return nil, "", err
	}
	if !found {
		return nil, "", &httpError{code: 404, msg: "playlist state not found"}
	}

	cfg := p.deps.Configs[tag]
	locator := stream.Locator{
		URL:     meta.UpstreamBaseURL,
		Headers: meta.Headers,
		Query:   meta.Query,
	}

	var buf bytes.Buffer
	strategy := newStrategy(format, cfg, p.deps)
	hlsStrategy, ok := strategy.(*HLSStrategy)
	if !ok {
		// Unwrap decorators to find HLSStrategy
		hlsStrategy = unwrapHLS(strategy)
	}
	if hlsStrategy == nil {
		return nil, "", &httpError{code: 500, msg: "HLS strategy not found"}
	}

	if err := hlsStrategy.ServeSubPlaylist(ctx, &buf, locator, &meta, stateKeyType, stateID); err != nil {
		return nil, "", err
	}

	return io.NopCloser(&buf), ManifestContentType(format), nil
}

// ServeHLSSegment fetches an HLS segment and streams it.
func (p *Proxy) ServeHLSSegment(ctx context.Context, tag, contentKey, format, stateKeyType, stateID, segment string) (io.ReadCloser, string, error) {
	stateKey := hlsSegmentStateKey(tag, contentKey, stateKeyType, stateID)
	meta, found, err := p.deps.State.Get(ctx, stateKey)
	if err != nil {
		return nil, "", err
	}
	if !found {
		return nil, "", &httpError{code: 404, msg: "segment state not found"}
	}

	baseURL, err := url.Parse(meta.UpstreamBaseURL)
	if err != nil {
		return nil, "", &httpError{code: 500, msg: "invalid upstream base URL"}
	}

	locator := stream.Locator{
		URL:     baseURL.JoinPath(segment).String(),
		Headers: meta.Headers,
		Query:   meta.Query,
	}

	cfg := p.deps.Configs[tag]
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

// ServeHLSResource fetches an HLS resource (key, partial segment, session data, etc.)
// from the upstream URL stored in state, identified by resourceType and resourceID.
func (p *Proxy) ServeHLSResource(ctx context.Context, tag, contentKey, format, resourceType, resourceID string) (io.ReadCloser, string, error) {
	stateKey := hlsResourceStateKey(tag, contentKey, resourceType, resourceID)
	meta, found, err := p.deps.State.Get(ctx, stateKey)
	if err != nil {
		return nil, "", err
	}
	if !found {
		return nil, "", &httpError{code: 404, msg: "HLS resource state not found"}
	}

	locator := stream.Locator{
		URL:     meta.UpstreamBaseURL,
		Headers: meta.Headers,
		Query:   meta.Query,
	}

	body, _, err := p.deps.Fetcher.Fetch(ctx, locator.URL, locator.Headers, locator.Query)
	if err != nil {
		return nil, "", err
	}

	return body, resourceContentType(format, resourceID), nil
}

// ResourceContentType returns the Content-Type for an HLS resource file.
func ResourceContentType(format, resourceID string) string {
	return resourceContentType(format, resourceID)
}

// resourceContentType returns the Content-Type for an HLS resource file.
func resourceContentType(format, resourceID string) string {
	if strings.HasSuffix(resourceID, ".key") || resourceID == "" {
		return "application/octet-stream"
	}
	if strings.HasSuffix(resourceID, ".ts") {
		return "video/mp2t"
	}
	if strings.HasSuffix(resourceID, ".m4s") || strings.HasSuffix(resourceID, ".mp4") {
		return "video/mp4"
	}
	if strings.HasSuffix(resourceID, ".json") {
		return "application/json"
	}
	if format == "hls" {
		if strings.HasSuffix(resourceID, ".ts") {
			return "video/mp2t"
		}
		return "video/mp4"
	}
	return "application/octet-stream"
}

// ServeDASHSegment fetches a DASH segment by resolving the upstream template.
func (p *Proxy) ServeDASHSegment(ctx context.Context, tag, contentKey, format string, periodIdx, asIdx int, repID, segment string) (io.ReadCloser, string, error) {
	stateKey := dashStateKey(tag, contentKey, periodIdx, asIdx, repID)
	meta, found, err := p.deps.State.Get(ctx, stateKey)
	if err != nil {
		return nil, "", err
	}
	if !found {
		return nil, "", &httpError{code: 404, msg: "DASH state not found"}
	}

	// Reconstruct upstream URL from template
	upstreamURL := meta.UpstreamMediaTemplate
	upstreamURL = strings.ReplaceAll(upstreamURL, "$RepresentationID$", meta.UpstreamRepID)
	upstreamURL = strings.ReplaceAll(upstreamURL, "$Bandwidth$", meta.UpstreamBandwidth)
	if idx := strings.Index(upstreamURL, "$Number"); idx != -1 {
		rest := upstreamURL[idx+len("$Number"):]
		if end := strings.Index(rest, "$"); end != -1 {
			upstreamURL = upstreamURL[:idx] + segment + rest[end+1:]
		}
	} else if idx := strings.Index(upstreamURL, "$Time"); idx != -1 {
		rest := upstreamURL[idx+len("$Time"):]
		if end := strings.Index(rest, "$"); end != -1 {
			upstreamURL = upstreamURL[:idx] + segment + rest[end+1:]
		}
	}

	locator := stream.Locator{
		URL:     upstreamURL,
		Headers: meta.Headers,
		Query:   meta.Query,
	}

	cfg := p.deps.Configs[tag]
	strategy := newStrategy(format, cfg, p.deps)

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		if err := strategy.ServeSegment(ctx, pw, locator, segment); err != nil {
			pw.CloseWithError(err)
		}
	}()

	return pr, "video/mp4", nil
}

// ServeDASHInit fetches a DASH init segment by resolving the upstream template.
func (p *Proxy) ServeDASHInit(ctx context.Context, tag, contentKey, format string, periodIdx, asIdx int, repID string) (io.ReadCloser, string, error) {
	stateKey := dashStateKey(tag, contentKey, periodIdx, asIdx, repID)
	meta, found, err := p.deps.State.Get(ctx, stateKey)
	if err != nil {
		return nil, "", err
	}
	if !found {
		return nil, "", &httpError{code: 404, msg: "DASH state not found"}
	}

	// Reconstruct upstream URL from init template
	upstreamURL := meta.UpstreamInitTemplate
	upstreamURL = strings.ReplaceAll(upstreamURL, "$RepresentationID$", meta.UpstreamRepID)
	upstreamURL = strings.ReplaceAll(upstreamURL, "$Bandwidth$", meta.UpstreamBandwidth)

	locator := stream.Locator{
		URL:     upstreamURL,
		Headers: meta.Headers,
		Query:   meta.Query,
	}

	cfg := p.deps.Configs[tag]
	strategy := newStrategy(format, cfg, p.deps)

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		if err := strategy.ServeSegment(ctx, pw, locator, "init"); err != nil {
			pw.CloseWithError(err)
		}
	}()

	return pr, "video/mp4", nil
}

// unwrapHLS traverses decorators to find the inner HLSStrategy.
func unwrapHLS(s Strategy) *HLSStrategy {
	switch t := s.(type) {
	case *HLSStrategy:
		return t
	case *AuthDecorator:
		return unwrapHLS(t.inner)
	case *CachingDecorator:
		return unwrapHLS(t.inner)
	case *DRMDecorator:
		return unwrapHLS(t.inner)
	}
	return nil
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
