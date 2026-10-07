// Package stremio implements the generic Stremio-addon provider slot: one
// adapter that speaks the Stremio addon protocol (manifest + catalog + meta +
// stream over JSON/HTTP), so any addon an operator links works as a lazy
// provider without bespoke code (PLAN.md §5.4). Archive.org, Cinemeta, or a
// self-hosted demo addon are all just manifest URLs.
package stremio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// Account is one addon instance: the operator-declared provider whose lazy
// surface this slot serves. ManifestURL points at the addon's manifest.json;
// base URL for all addon endpoints is its directory.
type Account struct {
	ID          string
	ManifestURL string
}

// AccountSource resolves an account id, as in the jellyfin slot.
type AccountSource interface {
	Lookup(ctx context.Context, accountID string) (Account, error)
}

type singleAccountSource Account

func (s singleAccountSource) Lookup(_ context.Context, _ string) (Account, error) {
	return Account(s), nil
}

// Option customizes the slot.
type Option func(*clientConfig)

type clientConfig struct {
	httpClient *http.Client
}

// WithHTTPClient overrides the HTTP client (tests inject their server).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *clientConfig) { c.httpClient = hc }
}

// Slot serves the addons of the given account ids through one generic client.
type Slot struct {
	corev1.UnimplementedMetaServiceServer
	slotsv1.UnimplementedProviderServiceServer

	mu   sync.Mutex
	ids  map[string]bool
	src  AccountSource
	opts clientConfig
	man  map[string]*manifest // manifest cache by account id
}

type manifest struct {
	Types    []string        `json:"types"`
	Catalogs []catalogRecord `json:"catalogs"`
	base     string          // manifest URL directory, no trailing slash
}

type catalogRecord struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type catalogResponse struct {
	Metas []metaRecord `json:"metas"`
}

type metaRecord struct {
	ID   string      `json:"id"`
	Type string      `json:"type"`
	Name string      `json:"name"`
	Year interface{} `json:"year,omitempty"`
}

type metaResponse struct {
	Meta metaRecord `json:"meta"`
}

type streamResponse struct {
	Streams []struct {
		URL   string `json:"url"`
		Title string `json:"title,omitempty"`
	} `json:"streams"`
}

// New builds a slot. The first request per account fetches its manifest, so a
// bad manifest URL fails at first use with a typed error.
func New(ids []string, src AccountSource, opts ...Option) (*Slot, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("stremio: at least one account must be declared")
	}
	if src == nil {
		return nil, fmt.Errorf("stremio: an account source is required")
	}
	seen := make(map[string]bool)
	for _, id := range ids {
		if id == "" || seen[id] {
			return nil, fmt.Errorf("stremio: account ids must be non-empty and unique, got %q", id)
		}
		seen[id] = true
		if _, err := src.Lookup(context.Background(), id); err != nil {
			return nil, fmt.Errorf("stremio: account %q does not resolve: %w", id, err)
		}
	}
	cfg := clientConfig{httpClient: &http.Client{Timeout: 30 * time.Second}}
	for _, o := range opts {
		o(&cfg)
	}
	return &Slot{ids: seen, src: src, opts: cfg, man: map[string]*manifest{}}, nil
}

// CapabilityQuery declares the lazy capability set; it never declares
// catalogue sync, because a generic addon has no account-library index.
func (s *Slot) CapabilityQuery(_ context.Context, _ *corev1.CapabilityQueryRequest) (*corev1.CapabilityQueryResponse, error) {
	return &corev1.CapabilityQueryResponse{Capabilities: []*corev1.Capability{
		{Name: "meta", Version: 1},
		{Name: "search", Version: 1},
		{Name: "browse", Version: 1},
		{Name: "produce-sources", Version: 1},
		{Name: "refresh-availability", Version: 1},
	}}, nil
}

// CatalogueSync is refused: lazy providers have no index to enumerate.
func (s *Slot) CatalogueSync(_ context.Context, req *slotsv1.CatalogueSyncRequest) (*slotsv1.CatalogueSyncResponse, error) {
	if req.GetAccountId() == "" {
		return nil, fmt.Errorf("catalogue sync request: account_id is required")
	}
	return nil, fmt.Errorf("stremio: whole-catalogue sync is not a lazy-provider capability")
}

// SearchCatalog runs the addon's catalog search across its movie catalogs.
func (s *Slot) SearchCatalog(ctx context.Context, req *slotsv1.SearchCatalogRequest) (*slotsv1.SearchCatalogResponse, error) {
	if req.GetAccountId() == "" {
		return nil, fmt.Errorf("search catalog request: account_id is required")
	}
	if strings.TrimSpace(req.GetQuery()) == "" {
		return nil, fmt.Errorf("search catalog request: query is required")
	}
	m, err := s.manifestFor(ctx, req.GetAccountId())
	if err != nil {
		return nil, err
	}
	out := []*slotsv1.CatalogueItem{}
	for _, c := range m.Catalogs {
		q := url.Values{"search": {req.GetQuery()}}
		metas, err := s.getCatalog(ctx, m, c, q)
		if err != nil {
			return nil, err
		}
		for _, me := range metas {
			out = append(out, toCatalogueItem(c.Type, me))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GetNativeId() < out[j].GetNativeId() })
	return &slotsv1.SearchCatalogResponse{Items: out}, nil
}

// BrowseCatalog lists one catalog of the addon (default: first movie catalog).
func (s *Slot) BrowseCatalog(ctx context.Context, req *slotsv1.BrowseCatalogRequest) (*slotsv1.BrowseCatalogResponse, error) {
	if req.GetAccountId() == "" {
		return nil, fmt.Errorf("browse catalog request: account_id is required")
	}
	m, err := s.manifestFor(ctx, req.GetAccountId())
	if err != nil {
		return nil, err
	}
	if len(m.Catalogs) == 0 {
		return &slotsv1.BrowseCatalogResponse{}, nil
	}
	c := m.Catalogs[0]
	if p := req.GetPath(); p != "" {
		found := false
		for _, cc := range m.Catalogs {
			if cc.Type+"/"+cc.ID == p || cc.ID == p {
				c = cc
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("stremio: no catalog %q on this addon", p)
		}
	}
	metas, err := s.getCatalog(ctx, m, c, nil)
	if err != nil {
		return nil, err
	}
	out := []*slotsv1.CatalogueItem{}
	for _, me := range metas {
		out = append(out, toCatalogueItem(c.Type, me))
	}
	return &slotsv1.BrowseCatalogResponse{Items: out}, nil
}

// RefreshAvailability is the explicit, user-triggered pure lookup (§5.3/§5.4).
func (s *Slot) RefreshAvailability(ctx context.Context, req *slotsv1.RefreshAvailabilityRequest) (*slotsv1.RefreshAvailabilityResponse, error) {
	if req.GetAccountId() == "" {
		return nil, fmt.Errorf("refresh availability request: account_id is required")
	}
	m, err := s.manifestFor(ctx, req.GetAccountId())
	if err != nil {
		return nil, err
	}
	out := []*slotsv1.CatalogueItem{}
	for _, nativeID := range req.GetNativeIds() {
		typ, id, ok := splitNativeID(nativeID)
		if !ok {
			continue // malformed id: not a known provider item, treated as gone
		}
		var mr metaResponse
		if err := s.getJSON(ctx, fmt.Sprintf("%s/meta/%s/%s.json", m.base, typ, id), &mr); err != nil || mr.Meta.ID == "" {
			continue
		}
		out = append(out, toCatalogueItem(typ, mr.Meta))
	}
	return &slotsv1.RefreshAvailabilityResponse{Items: out}, nil
}

// ProduceSources resolves one item's streams to a manifest (§6.2).
func (s *Slot) ProduceSources(ctx context.Context, req *slotsv1.ProduceSourcesRequest) (*slotsv1.ProduceSourcesResponse, error) {
	if req.GetAccountId() == "" || req.GetNativeId() == "" {
		return nil, fmt.Errorf("produce sources request: account_id and native_id are required")
	}
	m, err := s.manifestFor(ctx, req.GetAccountId())
	if err != nil {
		return nil, err
	}
	typ, id, ok := splitNativeID(req.GetNativeId())
	if !ok {
		return nil, fmt.Errorf("stremio: native_id must be \"type:id\", got %q", req.GetNativeId())
	}
	var sr streamResponse
	if err := s.getJSON(ctx, fmt.Sprintf("%s/stream/%s/%s.json", m.base, typ, id), &sr); err != nil {
		return nil, err
	}
	if len(sr.Streams) == 0 {
		return nil, fmt.Errorf("stremio: addon has no streams for %q", req.GetNativeId())
	}
	tracks := []*corev1.Track{}
	for i, st := range sr.Streams {
		tracks = append(tracks, &corev1.Track{
			Id:       fmt.Sprintf("stream-%d", i),
			Media:    &corev1.Track_Video{Video: &corev1.VideoTrack{Codec: "h264"}},
			Delivery: &corev1.TrackDelivery{Locations: []string{st.URL}},
		})
	}
	return &slotsv1.ProduceSourcesResponse{Source: &corev1.MediaSource{
		Type:        corev1.MediaSourceType_MEDIA_SOURCE_TYPE_STATIC,
		Seekable:    corev1.Seekable_SEEKABLE_FULL,
		Addressable: corev1.Addressable_ADDRESSABLE_WHOLE_MUX,
		Tracks:      tracks,
	}}, nil
}

// manifestFor fetches and caches one account's manifest.
func (s *Slot) manifestFor(ctx context.Context, accountID string) (*manifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.man[accountID]; ok {
		return m, nil
	}
	acct, err := s.src.Lookup(ctx, accountID)
	if err != nil {
		return nil, err
	}
	var mr manifest
	if err := s.getJSON(ctx, acct.ManifestURL, &mr); err != nil {
		return nil, fmt.Errorf("stremio: manifest %s: %w", acct.ManifestURL, err)
	}
	idx := strings.LastIndex(acct.ManifestURL, "/")
	if idx <= 0 {
		return nil, fmt.Errorf("stremio: malformed manifest url %q", acct.ManifestURL)
	}
	mr.base = acct.ManifestURL[:idx]
	s.man[accountID] = &mr
	return &mr, nil
}

func (s *Slot) getCatalog(ctx context.Context, m *manifest, c catalogRecord, q url.Values) ([]metaRecord, error) {
	u := fmt.Sprintf("%s/catalog/%s/%s.json", m.base, c.Type, c.ID)
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var cr catalogResponse
	if err := s.getJSON(ctx, u, &cr); err != nil {
		return nil, err
	}
	return cr.Metas, nil
}

func (s *Slot) getJSON(ctx context.Context, u string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := s.opts.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("stremio: GET %s: status %d", u, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func toCatalogueItem(typ string, me metaRecord) *slotsv1.CatalogueItem {
	kind := slotsv1.ItemKind_ITEM_KIND_MOVIE
	if typ == "series" {
		kind = slotsv1.ItemKind_ITEM_KIND_SERIES
	}
	return &slotsv1.CatalogueItem{
		NativeId: typ + ":" + me.ID,
		Kind:     kind,
		Metadata: &corev1.TitleMetadata{Title: me.Name, Year: asUint32(me.Year)},
	}
}

func asUint32(v interface{}) uint32 {
	switch n := v.(type) {
	case nil:
		return 0
	case float64:
		return uint32(n)
	case string:
		var y int
		if _, err := fmt.Sscanf(n, "%d", &y); err != nil {
			return 0
		}
		return uint32(y)
	default:
		return 0
	}
}

func splitNativeID(id string) (typ, rest string, ok bool) {
	i := strings.Index(id, ":")
	if i <= 0 || i == len(id)-1 {
		return "", "", false
	}
	return id[:i], id[i+1:], true
}
