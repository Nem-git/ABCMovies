// Package mocklazy is M7's built-in lazy streaming-service provider: a
// scriptable in-process slot. It models the streaming-service shape PLAN.md
// §5.4 describes — no whole-catalogue index, browse/search as the only
// catalogue probe, produce-sources at click/play — so the lazy-refresh
// fixtures are deterministic. A real (Archive.org) adapter exercises the same
// contract over HTTP.
package mocklazy

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// Item is one catalogue entry the mock serves.
type Item struct {
	NativeID  string
	Title     string
	Kind      slotsv1.ItemKind
	Year      int
	Available bool // gone from the "provider" (delisted) when false
}

// Slot implements the meta handshake and the lazy provider capability set.
type Slot struct {
	corev1.UnimplementedMetaServiceServer
	slotsv1.UnimplementedProviderServiceServer

	mu       sync.Mutex
	items    map[string]Item
	calls    map[string]int
	degraded bool
	streams  atomic.Int64
}

// New builds the slot from a fixture catalogue.
func New(items []Item) *Slot {
	m := make(map[string]Item, len(items))
	for _, it := range items {
		m[it.NativeID] = it
	}
	return &Slot{items: m, calls: map[string]int{}}
}

// Calls reports how many times one RPC has been invoked — the no-background-
// probing assertion reads this.
func (s *Slot) Calls(rpc string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[rpc]
}

// SetDegraded marks the provider unusable (PLAN.md §4): RefreshAvailability
// fails and no background probing may start.
func (s *Slot) SetDegraded(d bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.degraded = d
}

// AddItem inserts a new item the provider now carries.
func (s *Slot) AddItem(it Item) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[it.NativeID] = it
}

// RemoveItem delists an item from the provider.
func (s *Slot) RemoveItem(nativeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if it, ok := s.items[nativeID]; ok {
		it.Available = false
		s.items[nativeID] = it
	}
}

func (s *Slot) record(rpc string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls[rpc]++
}

// CapabilityQuery declares the lazy capability set — never "browse" as
// whole-catalogue sync: a lazy provider has no such operation.
func (s *Slot) CapabilityQuery(_ context.Context, _ *corev1.CapabilityQueryRequest) (*corev1.CapabilityQueryResponse, error) {
	return &corev1.CapabilityQueryResponse{
		Capabilities: []*corev1.Capability{
			{Name: "meta", Version: 1},
			{Name: "search", Version: 1},
			{Name: "browse", Version: 1},
			{Name: "produce-sources", Version: 1},
			{Name: "refresh-availability", Version: 1},
		},
	}, nil
}

// CatalogueSync a lazy provider refuses outright (PLAN.md §5.4): it has no
// index to enumerate. A core that calls it has misclassified the provider —
// reject, never downgrade.
func (s *Slot) CatalogueSync(_ context.Context, req *slotsv1.CatalogueSyncRequest) (*slotsv1.CatalogueSyncResponse, error) {
	s.record("CatalogueSync")
	if req.GetAccountId() == "" {
		return nil, fmt.Errorf("catalogue sync request: account_id is required")
	}
	return nil, fmt.Errorf("mocklazy: whole-catalogue sync is not a lazy-provider capability")
}

// SearchCatalog returns items whose title contains the query (case-insensitive).
func (s *Slot) SearchCatalog(_ context.Context, req *slotsv1.SearchCatalogRequest) (*slotsv1.SearchCatalogResponse, error) {
	s.record("SearchCatalog")
	if req.GetAccountId() == "" {
		return nil, fmt.Errorf("search catalog request: account_id is required")
	}
	if req.GetQuery() == "" {
		return nil, fmt.Errorf("search catalog request: query is required")
	}
	out := []*slotsv1.CatalogueItem{}
	for _, it := range s.snapshot() {
		if it.Available && strings.Contains(strings.ToLower(it.Title), strings.ToLower(req.GetQuery())) {
			out = append(out, toWire(it))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GetNativeId() < out[j].GetNativeId() })
	return &slotsv1.SearchCatalogResponse{Items: out}, nil
}

// BrowseCatalog returns the full available catalogue (a "landing" listing).
func (s *Slot) BrowseCatalog(_ context.Context, req *slotsv1.BrowseCatalogRequest) (*slotsv1.BrowseCatalogResponse, error) {
	s.record("BrowseCatalog")
	if req.GetAccountId() == "" {
		return nil, fmt.Errorf("browse catalog request: account_id is required")
	}
	out := []*slotsv1.CatalogueItem{}
	for _, it := range s.snapshot() {
		if it.Available {
			out = append(out, toWire(it))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GetNativeId() < out[j].GetNativeId() })
	return &slotsv1.BrowseCatalogResponse{Items: out}, nil
}

// RefreshAvailability is the explicit, user-triggered availability check
// (PLAN.md §5.4): a pure lookup that changes presence only. While degraded it
// fails rather than probing.
func (s *Slot) RefreshAvailability(_ context.Context, req *slotsv1.RefreshAvailabilityRequest) (*slotsv1.RefreshAvailabilityResponse, error) {
	s.record("RefreshAvailability")
	if req.GetAccountId() == "" {
		return nil, fmt.Errorf("refresh availability request: account_id is required")
	}
	s.mu.Lock()
	degraded := s.degraded
	s.mu.Unlock()
	if degraded {
		return nil, fmt.Errorf("mocklazy: provider degraded; no probing")
	}
	out := []*slotsv1.CatalogueItem{}
	for _, id := range req.GetNativeIds() {
		if it, ok := s.lookup(id); ok && it.Available {
			out = append(out, toWire(it))
		}
	}
	return &slotsv1.RefreshAvailabilityResponse{Items: out}, nil
}

// ProduceSources resolves one item to a manifest — the click/play availability
// confirmation (PLAN.md §5.4).
func (s *Slot) ProduceSources(_ context.Context, req *slotsv1.ProduceSourcesRequest) (*slotsv1.ProduceSourcesResponse, error) {
	s.record("ProduceSources")
	if req.GetAccountId() == "" || req.GetNativeId() == "" {
		return nil, fmt.Errorf("produce sources request: account_id and native_id are required")
	}
	it, ok := s.lookup(req.GetNativeId())
	if !ok || !it.Available {
		return nil, fmt.Errorf("mocklazy: %q is not available on this provider", req.GetNativeId())
	}
	s.streams.Add(1)
	return &slotsv1.ProduceSourcesResponse{Source: &corev1.MediaSource{
		Type:        corev1.MediaSourceType_MEDIA_SOURCE_TYPE_STATIC,
		Seekable:    corev1.Seekable_SEEKABLE_FULL,
		Addressable: corev1.Addressable_ADDRESSABLE_WHOLE_MUX,
		Tracks: []*corev1.Track{{
			Id:    "main",
			Media: &corev1.Track_Video{Video: &corev1.VideoTrack{Codec: "h264"}},
			Delivery: &corev1.TrackDelivery{Locations: []string{
				fmt.Sprintf("https://mock.test/%s.m3u8", it.NativeID),
			}},
		}},
	}}, nil
}

func (s *Slot) snapshot() []Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Item, 0, len(s.items))
	for _, it := range s.items {
		out = append(out, it)
	}
	return out
}

func (s *Slot) lookup(id string) (Item, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.items[id]
	return it, ok
}

func toWire(it Item) *slotsv1.CatalogueItem {
	return &slotsv1.CatalogueItem{
		NativeId: it.NativeID,
		Kind:     it.Kind,
		Metadata: &corev1.TitleMetadata{
			Title: it.Title,
			Year:  uint32(it.Year),
		},
	}
}
