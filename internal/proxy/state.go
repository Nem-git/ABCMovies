package proxy

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// StreamMeta stores the state for a proxied stream session.
// Used so segment requests can resolve upstream URLs without re-calling the provider.
type StreamMeta struct {
	ProviderTag           string
	ContentKey            string // e.g. "movies:123" or "series:456:789:101"
	StreamFile            string // e.g. "master.m3u8", "manifest.mpd"
	Format                string // "hls", "dash", "mp4"
	UpstreamBaseURL       string // HLS: base URL for resolving relative segment paths
	UpstreamMediaTemplate string // DASH: full upstream media segment URL template
	UpstreamInitTemplate  string // DASH: full upstream init segment URL template
	UpstreamRepID         string // DASH: representation Id
	UpstreamBandwidth     string // DASH: representation bandwidth
	ProxyBaseURL          string // Proxy base URL for constructing absolute URLs
	Headers               http.Header
	Query                 url.Values
	EncodingFormat        string
	ExpiresAt             time.Time
}

// StateStore maps stream rendition keys to upstream metadata.
type StateStore interface {
	Put(ctx context.Context, key string, meta StreamMeta) error
	Get(ctx context.Context, key string) (StreamMeta, bool, error)
	Delete(ctx context.Context, key string) error
	Cleanup(ctx context.Context) error
}
