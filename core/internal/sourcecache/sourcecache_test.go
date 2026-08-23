package sourcecache

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// fakeProvider serves canned pages; the last non-empty token page errors if
// failOnPage matches.
type fakeProvider struct {
	pages      []*slotsv1.CatalogueSyncResponse
	failOnPage int // 1-based; 0 never fails
}

func (f *fakeProvider) CatalogueSync(_ context.Context, req *slotsv1.CatalogueSyncRequest) (*slotsv1.CatalogueSyncResponse, error) {
	idx := 0
	if req.GetPageToken() != "" {
		if _, err := fmt.Sscanf(req.GetPageToken(), "p%d", &idx); err != nil {
			return nil, fmt.Errorf("bad page token")
		}
	}
	if idx >= len(f.pages) {
		return nil, fmt.Errorf("page out of range")
	}
	if f.failOnPage == idx+1 {
		return nil, fmt.Errorf("provider error")
	}
	return f.pages[idx], nil
}

func movie(id string) *slotsv1.CatalogueItem {
	return &slotsv1.CatalogueItem{
		NativeId: id,
		Kind:     slotsv1.ItemKind_ITEM_KIND_MOVIE,
		Metadata: &corev1.TitleMetadata{Title: "T " + id, Year: 2001},
		ExternalIds: []*slotsv1.ExternalId{
			{Namespace: "imdb", Value: "tt" + id},
		},
	}
}

func newSync(t *testing.T, client Client) (*Synchronizer, store.Store) {
	t.Helper()
	cache := store.NewInMemory()
	s, err := New("jellyfin", client, cache, slog.Default())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s, cache
}

func TestSyncAccountPagesAndPersistsWholeCatalogue(t *testing.T) {
	f := &fakeProvider{pages: []*slotsv1.CatalogueSyncResponse{
		{Items: []*slotsv1.CatalogueItem{movie("1"), movie("2")}, NextPageToken: "p1"},
		{Items: []*slotsv1.CatalogueItem{movie("3")}},
	}}
	s, _ := newSync(t, f)

	stats, err := s.SyncAccount(context.Background(), "primary")
	if err != nil {
		t.Fatalf("SyncAccount: %v", err)
	}
	if stats.Items != 3 || stats.Pages != 2 {
		t.Fatalf("stats = %+v, want 3 items across 2 pages", stats)
	}

	items, err := s.ListItems(context.Background(), "primary")
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("cached %d items, want 3", len(items))
	}

	m, ok, err := s.Manifest(context.Background(), "primary")
	if err != nil || !ok {
		t.Fatalf("Manifest: %v (ok=%v)", err, ok)
	}
	if m.Items != 3 || m.LastCompleteSync.IsZero() {
		t.Fatalf("manifest = %+v", m)
	}
}

func TestSyncEmptyLibraryRecordsCompleteManifest(t *testing.T) {
	f := &fakeProvider{pages: []*slotsv1.CatalogueSyncResponse{{}}}
	s, _ := newSync(t, f)

	stats, err := s.SyncAccount(context.Background(), "primary")
	if err != nil {
		t.Fatalf("SyncAccount: %v", err)
	}
	if stats.Items != 0 {
		t.Fatalf("items = %d, want 0", stats.Items)
	}
	m, ok, _ := s.Manifest(context.Background(), "primary")
	if !ok || m.Items != 0 {
		t.Fatalf("empty sync must still complete: manifest=%+v ok=%v", m, ok)
	}
}

func TestSyncAbortsOnContractViolationWithoutCompleting(t *testing.T) {
	f := &fakeProvider{pages: []*slotsv1.CatalogueSyncResponse{
		{Items: []*slotsv1.CatalogueItem{movie("1")}, NextPageToken: "p1"},
		// Second page violates the contract: item without native_id.
		{Items: []*slotsv1.CatalogueItem{{Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE, Metadata: &corev1.TitleMetadata{Title: "Broken"}}}},
	}}
	s, cache := newSync(t, f)

	_, err := s.SyncAccount(context.Background(), "primary")
	if err == nil {
		t.Fatal("contract violation accepted")
	}
	m, ok, _ := s.Manifest(context.Background(), "primary")
	if ok {
		t.Fatalf("manifest must not record an incomplete sync: %+v", m)
	}
	// Page 1 items may be present, but the broken page's invalid item must
	// not exist.
	if _, err := cache.Get(context.Background(), "jellyfin/primary/"); err == nil {
		t.Fatal("invalid item was written")
	}
}

func TestSyncAbortsOnProviderError(t *testing.T) {
	f := &fakeProvider{pages: []*slotsv1.CatalogueSyncResponse{{Items: []*slotsv1.CatalogueItem{movie("1")}, NextPageToken: "p1"}, {}}, failOnPage: 2}
	s, _ := newSync(t, f)

	if _, err := s.SyncAccount(context.Background(), "primary"); err == nil {
		t.Fatal("provider error swallowed")
	}
}

func TestNewRejectsMissingParts(t *testing.T) {
	cache := store.NewInMemory()
	if _, err := New("", &fakeProvider{}, cache, nil); err == nil {
		t.Fatal("empty provider accepted")
	}
	if _, err := New("jellyfin", nil, cache, nil); err == nil {
		t.Fatal("nil client accepted")
	}
	if _, err := New("jellyfin", &fakeProvider{}, nil, nil); err == nil {
		t.Fatal("nil cache accepted")
	}
}
