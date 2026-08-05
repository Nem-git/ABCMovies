package proxy

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/nem-git/abcmovies/internal/hashid"
	"github.com/nem-git/abcmovies/internal/proxy/parser"
	"github.com/nem-git/abcmovies/internal/stream"
)

// StrategyDeps holds shared dependencies for strategies.
type StrategyDeps struct {
	Fetcher Fetcher
	State   StateStore
}

// HLSStrategy handles HLS manifest rewriting with two-pass state population.
type HLSStrategy struct {
	deps StrategyDeps
}

// NewHLSStrategy creates an HLS strategy with the given dependencies.
func NewHLSStrategy(deps StrategyDeps) *HLSStrategy {
	return &HLSStrategy{deps: deps}
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

	upstreamBaseURL := ResolveBaseURL(locator.URL)
	p := parser.HLSParser{}

	master, media, err := p.Decode(data)
	if err != nil {
		return "", fmt.Errorf("decode HLS playlist: %w", err)
	}

	if media != nil {
		return s.handleSingleMediaPlaylist(ctx, w, meta, p, media, upstreamBaseURL, locator)
	}

	return s.handleMasterPlaylist(ctx, w, meta, p, master, upstreamBaseURL)
}

func (s *HLSStrategy) handleMasterPlaylist(ctx context.Context, w io.Writer, meta *StreamMeta, p parser.HLSParser, master *parser.MasterPlaylist, upstreamBaseURL string) (string, error) {
	// Store playlist state per variant and rewrite variant URIs
	for i := range master.Variants {
		variantID := hashid.URLHash(parser.ResolveURL(upstreamBaseURL, master.Variants[i].URI))
		playlistKey := hlsPlaylistStateKey(meta.ProviderTag, meta.ContentKey, "variants", variantID)
		playlistMeta := *meta
		playlistMeta.UpstreamBaseURL = parser.ResolveURL(upstreamBaseURL, master.Variants[i].URI)
		playlistMeta.ExpiresAt = time.Now().Add(5 * time.Minute)
		s.deps.State.Put(ctx, playlistKey, playlistMeta)
		master.Variants[i].URI = path.Join(meta.ProxyBaseURL, "variants", variantID)
	}

	// Store playlist state per rendition (skip those without URI — muxed content)
	for _, a := range master.GetAllAlternatives() {
		if a.URI == "" {
			continue
		}
		playlistKey := hlsPlaylistStateKey(meta.ProviderTag, meta.ContentKey, "renditions", a.GroupId+"/"+a.Name)
		playlistMeta := *meta
		playlistMeta.UpstreamBaseURL = parser.ResolveURL(upstreamBaseURL, a.URI)
		playlistMeta.ExpiresAt = time.Now().Add(5 * time.Minute)
		s.deps.State.Put(ctx, playlistKey, playlistMeta)
	}

	// Rewrite rendition URIs in the master playlist
	for _, a := range master.GetAllAlternatives() {
		if a.URI == "" {
			continue
		}
		// Find the alternative in the variants and rewrite
		for vi, v := range master.Variants {
			for ai, alt := range v.Alternatives {
				if alt.GroupId == a.GroupId && alt.Name == a.Name {
					master.Variants[vi].Alternatives[ai].URI = path.Join(meta.ProxyBaseURL, "groups", a.GroupId, "renditions", a.Name)
				}
			}
		}
	}

	// Store and rewrite session keys
	for i := range master.SessionKeys {
		if master.SessionKeys[i] == nil || master.SessionKeys[i].URI == "" {
			continue
		}
		abs := parser.ResolveURL(upstreamBaseURL, master.SessionKeys[i].URI)
		hash := hashid.URLHash(abs)
		resourceKey := hlsResourceStateKey(meta.ProviderTag, meta.ContentKey, "session-key", hash)
		resourceMeta := *meta
		resourceMeta.UpstreamBaseURL = abs
		resourceMeta.ExpiresAt = time.Now().Add(5 * time.Minute)
		s.deps.State.Put(ctx, resourceKey, resourceMeta)
		master.SessionKeys[i].URI = path.Join(meta.ProxyBaseURL, "session-keys", hash)
	}

	// Store and rewrite session data URIs
	for i := range master.SessionDatas {
		if master.SessionDatas[i] == nil || master.SessionDatas[i].URI == "" {
			continue
		}
		abs := parser.ResolveURL(upstreamBaseURL, master.SessionDatas[i].URI)
		hash := hashid.URLHash(abs)
		resourceKey := hlsResourceStateKey(meta.ProviderTag, meta.ContentKey, "session-data", hash)
		resourceMeta := *meta
		resourceMeta.UpstreamBaseURL = abs
		resourceMeta.ExpiresAt = time.Now().Add(5 * time.Minute)
		s.deps.State.Put(ctx, resourceKey, resourceMeta)
		master.SessionDatas[i].URI = path.Join(meta.ProxyBaseURL, "session-data", hash)
	}

	// Store and rewrite content steering URI
	if master.ContentSteering != nil && master.ContentSteering.ServerURI != "" {
		abs := parser.ResolveURL(upstreamBaseURL, master.ContentSteering.ServerURI)
		resourceKey := hlsResourceStateKey(meta.ProviderTag, meta.ContentKey, "steering", "")
		resourceMeta := *meta
		resourceMeta.UpstreamBaseURL = abs
		resourceMeta.ExpiresAt = time.Now().Add(5 * time.Minute)
		s.deps.State.Put(ctx, resourceKey, resourceMeta)
		master.ContentSteering.ServerURI = path.Join(meta.ProxyBaseURL, "steering")
	}

	rewritten, err := p.EncodeMaster(master)
	if err != nil {
		return "", err
	}
	w.Write(rewritten)
	return upstreamBaseURL, nil
}

func (s *HLSStrategy) handleSingleMediaPlaylist(ctx context.Context, w io.Writer, meta *StreamMeta, p parser.HLSParser, media *parser.MediaPlaylist, upstreamBaseURL string, locator stream.Locator) (string, error) {
	// Generate synthetic master
	variantHash := hashid.URLHash(locator.URL)
	variantURI := path.Join(meta.ProxyBaseURL, "variants", variantHash)
	master := parser.SyntheticMaster(variantURI)

	// Store playlist state for the single variant
	playlistKey := hlsPlaylistStateKey(meta.ProviderTag, meta.ContentKey, "variants", variantHash)
	playlistMeta := *meta
	playlistMeta.UpstreamBaseURL = locator.URL
	playlistMeta.ExpiresAt = time.Now().Add(5 * time.Minute)
	s.deps.State.Put(ctx, playlistKey, playlistMeta)

	// Compute upstream segment base URL before rewriting
	upstreamSegmentBaseURL := computeUpstreamSegmentBaseURL(media, upstreamBaseURL)

	// Rewrite segment URLs and resources in the media playlist
	segmentBaseURL := path.Join(meta.ProxyBaseURL, "variants", variantHash, "segments")
	s.rewriteMediaPlaylist(ctx, meta, media, upstreamBaseURL, segmentBaseURL)

	// Store segment state for the single variant
	segmentKey := hlsSegmentStateKey(meta.ProviderTag, meta.ContentKey, "variants", variantHash)
	segmentMeta := *meta
	segmentMeta.UpstreamBaseURL = upstreamSegmentBaseURL
	segmentMeta.ExpiresAt = time.Now().Add(5 * time.Minute)
	s.deps.State.Put(ctx, segmentKey, segmentMeta)

	// Encode and return synthetic master
	rewritten, err := p.EncodeMaster(master)
	if err != nil {
		return "", err
	}
	w.Write(rewritten)
	return upstreamBaseURL, nil
}

// ServeSubPlaylist fetches an upstream sub-playlist, rewrites it, and returns it.
func (s *HLSStrategy) ServeSubPlaylist(ctx context.Context, w io.Writer, locator stream.Locator, meta *StreamMeta, stateKeyType, stateID string) error {
	body, _, err := s.deps.Fetcher.Fetch(ctx, locator.URL, locator.Headers, locator.Query)
	if err != nil {
		return err
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}

	upstreamBaseURL := ResolveBaseURL(locator.URL)
	p := parser.HLSParser{}

	_, media, err := p.Decode(data)
	if err != nil {
		return fmt.Errorf("decode HLS sub-playlist: %w", err)
	}
	if media == nil {
		return fmt.Errorf("upstream URL is not a media playlist")
	}

	// Compute upstream segment base URL before rewriting
	upstreamSegmentBaseURL := computeUpstreamSegmentBaseURL(media, upstreamBaseURL)

	// Rewrite segment URLs (absolute to the proxy base so they resolve regardless
	// of whether the playlist URL carries a trailing slash)
	var segmentBase string
	if stateKeyType == "renditions" {
		group, name, _ := strings.Cut(stateID, "/")
		segmentBase = path.Join(meta.ProxyBaseURL, "groups", group, "renditions", name, "segments")
	} else {
		segmentBase = path.Join(meta.ProxyBaseURL, "variants", stateID, "segments")
	}
	rewrittenMedia := s.rewriteMediaPlaylist(ctx, meta, media, upstreamBaseURL, segmentBase)

	// Store segment state
	segmentKey := hlsSegmentStateKey(meta.ProviderTag, meta.ContentKey, stateKeyType, stateID)
	segmentMeta := *meta
	segmentMeta.UpstreamBaseURL = upstreamSegmentBaseURL
	segmentMeta.ExpiresAt = time.Now().Add(5 * time.Minute)
	s.deps.State.Put(ctx, segmentKey, segmentMeta)

	// Encode and return
	rewritten, err := p.EncodeMedia(rewrittenMedia)
	if err != nil {
		return err
	}
	w.Write(rewritten)
	return nil
}

func (s *HLSStrategy) rewriteMediaPlaylist(ctx context.Context, meta *StreamMeta, media *parser.MediaPlaylist, playlistBaseURL, segmentBaseURL string) *parser.MediaPlaylist {
	// Rewrite the top-level Map (used by the encoder for EXT-X-MAP)
	if media.Map != nil && media.Map.URI != "" {
		abs := parser.ResolveURL(playlistBaseURL, media.Map.URI)
		filename := urlBaseName(abs)
		media.Map.URI = path.Join(segmentBaseURL, filename)
	}
	// Rewrite top-level Keys (playlist-level EXT-X-KEY)
	s.rewriteKeys(ctx, meta, media.Keys, playlistBaseURL)
	for i, seg := range media.Segments {
		if seg == nil {
			continue
		}
		if seg.URI != "" {
			abs := parser.ResolveURL(playlistBaseURL, seg.URI)
			filename := urlBaseName(abs)
			media.Segments[i].URI = path.Join(segmentBaseURL, filename)
		}
		if seg.Map != nil && seg.Map.URI != "" {
			abs := parser.ResolveURL(playlistBaseURL, seg.Map.URI)
			filename := urlBaseName(abs)
			newMap := *seg.Map
			newMap.URI = path.Join(segmentBaseURL, filename)
			media.Segments[i].Map = &newMap
		}
		// Rewrite per-segment Keys
		if len(seg.Keys) > 0 {
			media.Segments[i].Keys = s.rewriteKeys(ctx, meta, seg.Keys, playlistBaseURL)
		}
	}
	// Rewrite PartialSegment URIs (EXT-X-PART)
	for i, ps := range media.PartialSegments {
		if ps == nil || ps.URI == "" {
			continue
		}
		abs := parser.ResolveURL(playlistBaseURL, ps.URI)
		hash := hashid.URLHash(abs)
		resourceKey := hlsResourceStateKey(meta.ProviderTag, meta.ContentKey, "partial", hash)
		resourceMeta := *meta
		resourceMeta.UpstreamBaseURL = abs
		resourceMeta.ExpiresAt = time.Now().Add(5 * time.Minute)
		s.deps.State.Put(ctx, resourceKey, resourceMeta)
		media.PartialSegments[i].URI = path.Join(meta.ProxyBaseURL, "partials", hash)
	}
	// Rewrite PreloadHint URI (EXT-X-PRELOAD-HINT)
	if media.PreloadHints != nil && media.PreloadHints.URI != "" {
		abs := parser.ResolveURL(playlistBaseURL, media.PreloadHints.URI)
		hash := hashid.URLHash(abs)
		resourceKey := hlsResourceStateKey(meta.ProviderTag, meta.ContentKey, "preload-hint", hash)
		resourceMeta := *meta
		resourceMeta.UpstreamBaseURL = abs
		resourceMeta.ExpiresAt = time.Now().Add(5 * time.Minute)
		s.deps.State.Put(ctx, resourceKey, resourceMeta)
		media.PreloadHints.URI = path.Join(meta.ProxyBaseURL, "preload-hints", hash)
	}
	return media
}

// rewriteKeys rewrites EXT-X-KEY URIs to proxy paths and stores state for fetching them later.
// Returns the updated keys slice.
// Skips URIs that look like proxy-relative paths (already rewritten) to avoid
// resolving relative paths against the playlist base when Key objects are shared
// between playlist-level and segment-level key slices by the parser.
func (s *HLSStrategy) rewriteKeys(ctx context.Context, meta *StreamMeta, keys []parser.Key, playlistBaseURL string) []parser.Key {
	for i := range keys {
		if keys[i].URI == "" {
			continue
		}
		// Skip if URI is already a relative proxy path (already rewritten)
		if !strings.HasPrefix(keys[i].URI, "http://") && !strings.HasPrefix(keys[i].URI, "https://") {
			continue
		}
		abs := parser.ResolveURL(playlistBaseURL, keys[i].URI)
		hash := hashid.URLHash(abs)
		resourceKey := hlsResourceStateKey(meta.ProviderTag, meta.ContentKey, "key", hash)
		resourceMeta := *meta
		resourceMeta.UpstreamBaseURL = abs
		resourceMeta.ExpiresAt = time.Now().Add(5 * time.Minute)
		s.deps.State.Put(ctx, resourceKey, resourceMeta)
		keys[i].URI = path.Join(meta.ProxyBaseURL, "keys", hash)
	}
	return keys
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

// computeUpstreamSegmentBaseURL resolves the first segment's URL against the playlist
// base URL and returns the upstream directory. Must be called before rewriting the playlist.
func computeUpstreamSegmentBaseURL(media *parser.MediaPlaylist, playlistBaseURL string) string {
	for _, seg := range media.Segments {
		if seg == nil || seg.URI == "" {
			continue
		}
		abs := parser.ResolveURL(playlistBaseURL, seg.URI)
		u, err := url.Parse(abs)
		if err != nil {
			continue
		}
		dir := filepath.Dir(u.Path)
		u.Path = dir + "/"
		return u.String()
	}
	return playlistBaseURL
}

// urlBaseName returns the last path component of a (possibly absolute) URL,
// ignoring any query or fragment so rewritten segment and map URIs never carry
// upstream token query strings.
func urlBaseName(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return filepath.Base(rawURL)
	}
	return filepath.Base(u.Path)
}

// HLS state key helpers.

func hlsPlaylistStateKey(tag, contentKey, keyType, id string) string {
	return tag + ":" + contentKey + ":hls:playlist:" + keyType + ":" + id
}

func hlsSegmentStateKey(tag, contentKey, keyType, id string) string {
	return tag + ":" + contentKey + ":hls:segment:" + keyType + ":" + id
}

// HLSPlaylistStateKey returns the state key for an HLS playlist entry.
func HLSPlaylistStateKey(tag, contentKey, keyType, id string) string {
	return hlsPlaylistStateKey(tag, contentKey, keyType, id)
}

// HLSSegmentStateKey returns the state key for an HLS segment entry.
func HLSSegmentStateKey(tag, contentKey, keyType, id string) string {
	return hlsSegmentStateKey(tag, contentKey, keyType, id)
}

func hlsResourceStateKey(tag, contentKey, resourceType, id string) string {
	return tag + ":" + contentKey + ":hls:resource:" + resourceType + ":" + id
}

// HLSResourceStateKey returns the state key for an HLS resource entry.
func HLSResourceStateKey(tag, contentKey, resourceType, id string) string {
	return hlsResourceStateKey(tag, contentKey, resourceType, id)
}
