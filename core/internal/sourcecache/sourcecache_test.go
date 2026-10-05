package sourcecache

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/itemregistry"
	"github.com/nem-git/abcmovies/core/internal/schema"
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

// captureSink records every published envelope.
type captureSink struct{ envs []*corev1.EventEnvelope }

func (c *captureSink) Publish(e *corev1.EventEnvelope) { c.envs = append(c.envs, e) }

// stubLookup answers from a fixed native-id → entry-id map.
type stubLookup struct{ entries map[string]string }

func (s *stubLookup) Lookup(_ context.Context, _, nativeID string) (string, bool, error) {
	id, ok := s.entries[nativeID]
	return id, ok, nil
}

var fullCatalogue = []*slotsv1.CatalogueSyncResponse{
	{Items: []*slotsv1.CatalogueItem{movie("1"), movie("2")}, NextPageToken: "p1"},
	{Items: []*slotsv1.CatalogueItem{movie("3")}},
}

func TestDeletionReconcilesAfterCompleteSync(t *testing.T) {
	f := &fakeProvider{pages: fullCatalogue}
	sink := &captureSink{}
	reg := &stubLookup{entries: map[string]string{"2": "le_e2", "3": "le_e3"}}
	cache := store.NewInMemory()
	s, err := New("jellyfin", f, cache, slog.Default(), WithEntryLookup(reg), WithEventsSink(sink))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()

	if _, err := s.SyncAccount(ctx, "primary"); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	items, _ := s.ListItems(ctx, "primary")
	if len(items) != 3 {
		t.Fatalf("setup: cached %d items, want 3", len(items))
	}
	// The setup sync reported arrivals for the mapped items; item 1 is
	// deliberately absent from the stub to exercise the skip path.
	if len(sink.envs) != 2 {
		t.Fatalf("setup published %d events, want one per mapped arrival", len(sink.envs))
	}
	sink.envs = nil

	// The provider now reports only item 1.
	f.pages = []*slotsv1.CatalogueSyncResponse{{Items: []*slotsv1.CatalogueItem{movie("1")}}}
	stats, err := s.SyncAccount(ctx, "primary")
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if stats.Items != 1 || stats.Removed != 2 {
		t.Fatalf("stats = %+v, want items=1 removed=2", stats)
	}
	items, err = s.ListItems(ctx, "primary")
	if err != nil || len(items) != 1 || items[0].GetNativeId() != "1" {
		t.Fatalf("cache after reconciliation = %d items (err=%v)", len(items), err)
	}

	if len(sink.envs) != 2 {
		t.Fatalf("published %d events, want one per removal", len(sink.envs))
	}
	var gone *corev1.AvailabilityEvent
	for _, env := range sink.envs {
		if err := schema.ValidateEventEnvelope(env); err != nil {
			t.Fatalf("published envelope invalid: %v", err)
		}
		av := env.GetAvailability()
		if av.GetPresent() {
			t.Fatalf("removal must report present=false: %+v", av)
		}
		if env.GetAudience() != corev1.EventAudience_EVENT_AUDIENCE_ACCOUNT || env.GetAccountId() != "primary" {
			t.Fatalf("event must be account-scoped: %+v", env)
		}
		if av.GetEntryId() == "le_e3" {
			gone = av
		}
	}
	if gone == nil {
		t.Fatal("no event carried the registry-resolved entry id")
	}
	if gone.GetProvider() != "jellyfin" {
		t.Fatalf("availability event names wrong provider: %+v", gone)
	}
}

func TestRemovedItemWithoutMappingSkipsEvent(t *testing.T) {
	f := &fakeProvider{pages: fullCatalogue}
	sink := &captureSink{}
	cache := store.NewInMemory()
	s, err := New("jellyfin", f, cache, slog.Default(), WithEntryLookup(&stubLookup{}), WithEventsSink(sink))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	if _, err := s.SyncAccount(ctx, "primary"); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	f.pages = []*slotsv1.CatalogueSyncResponse{{Items: []*slotsv1.CatalogueItem{movie("1")}}}
	stats, err := s.SyncAccount(ctx, "primary")
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if stats.Removed != 2 {
		t.Fatalf("removed = %d, want 2", stats.Removed)
	}
	if len(sink.envs) != 0 {
		t.Fatalf("unmapped removals must not emit events with blank entries: %d published", len(sink.envs))
	}
}

func TestFailedSyncNeverReconcilesDeletions(t *testing.T) {
	f := &fakeProvider{pages: fullCatalogue}
	cache := store.NewInMemory()
	s, err := New("jellyfin", f, cache, slog.Default())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	if _, err := s.SyncAccount(ctx, "primary"); err != nil {
		t.Fatalf("setup sync: %v", err)
	}
	before, ok, _ := s.Manifest(ctx, "primary")
	if !ok {
		t.Fatal("setup manifest missing")
	}

	// A provider that dies mid-run must not be read as a mass deletion.
	broken := &fakeProvider{
		pages:      []*slotsv1.CatalogueSyncResponse{{Items: []*slotsv1.CatalogueItem{movie("1")}, NextPageToken: "p1"}, {}},
		failOnPage: 2,
	}
	s2, err := New("jellyfin", broken, cache, slog.Default())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := s2.SyncAccount(ctx, "primary"); err == nil {
		t.Fatal("provider error swallowed")
	}
	items, err := s.ListItems(ctx, "primary")
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("failed run deleted items: %d remain", len(items))
	}
	after, ok, _ := s.Manifest(ctx, "primary")
	if !ok || !after.LastCompleteSync.Equal(before.LastCompleteSync) {
		t.Fatalf("failed run advanced the completion marker: %+v -> %+v", before, after)
	}
}

func TestRemovalWithoutCollaboratorsStillDeletes(t *testing.T) {
	f := &fakeProvider{pages: fullCatalogue}
	s, _ := newSync(t, f)
	ctx := context.Background()
	if _, err := s.SyncAccount(ctx, "primary"); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	f.pages = []*slotsv1.CatalogueSyncResponse{{Items: []*slotsv1.CatalogueItem{movie("1"), movie("2")}}}
	stats, err := s.SyncAccount(ctx, "primary")
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if stats.Removed != 1 {
		t.Fatalf("removed = %d, want 1 even without registry/sink wired", stats.Removed)
	}
	items, _ := s.ListItems(ctx, "primary")
	if len(items) != 2 {
		t.Fatalf("cached %d items after deletion, want 2", len(items))
	}
}

// resolvingProvider adapts *itemregistry.Registry to the ItemResolver
// interface for tests that exercise the full sync → identity → event path.
type resolveInto struct{ r *itemregistry.Registry }

func (a resolveInto) Resolve(ctx context.Context, provider string, item *slotsv1.CatalogueItem) error {
	_, err := a.r.Resolve(ctx, provider, item)
	return err
}

func TestArrivalEventsCarryResolvedEntryIDs(t *testing.T) {
	f := &fakeProvider{pages: fullCatalogue}
	sink := &captureSink{}
	cache := store.NewInMemory()
	reg, err := itemregistry.New(cache, "operator")
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	s, err := New("jellyfin", f, cache, slog.Default(),
		WithEntryLookup(reg), WithEventsSink(sink), WithItemResolver(resolveInto{reg}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	stats, err := s.SyncAccount(context.Background(), "primary")
	if err != nil {
		t.Fatalf("first sync: %v", err)
	}
	// The first sync is three arrivals; every item resolved during paging,
	// so each arrival carries its entry id.
	if stats.Items != 3 || len(sink.envs) != 3 {
		t.Fatalf("stats=%+v events=%d, want 3 arrivals reported", stats, len(sink.envs))
	}
	for _, env := range sink.envs {
		if err := schema.ValidateEventEnvelope(env); err != nil {
			t.Fatalf("arrival envelope invalid: %v", err)
		}
		av := env.GetAvailability()
		if !av.GetPresent() || av.GetEntryId() == "" || av.GetAccountId() != "primary" {
			t.Fatalf("arrival event = %+v", av)
		}
	}

	// A steady-state resync changes nothing and emits nothing.
	sink.envs = nil
	stats, err = s.SyncAccount(context.Background(), "primary")
	if err != nil || stats.Removed != 0 || len(sink.envs) != 0 {
		t.Fatalf("steady state: stats=%+v events=%d err=%v", stats, len(sink.envs), err)
	}
}

func TestSwapSyncEmitsArrivalAndDeparture(t *testing.T) {
	f := &fakeProvider{pages: fullCatalogue}
	sink := &captureSink{}
	cache := store.NewInMemory()
	reg, _ := itemregistry.New(cache, "")
	s, _ := New("jellyfin", f, cache, slog.Default(),
		WithEntryLookup(reg), WithEventsSink(sink), WithItemResolver(resolveInto{reg}))
	ctx := context.Background()
	if _, err := s.SyncAccount(ctx, "primary"); err != nil {
		t.Fatalf("setup: %v", err)
	}
	sink.envs = nil

	f.pages = []*slotsv1.CatalogueSyncResponse{{Items: []*slotsv1.CatalogueItem{movie("1"), movie("4")}}}
	stats, err := s.SyncAccount(ctx, "primary")
	if err != nil {
		t.Fatalf("swap sync: %v", err)
	}
	// Items 2 and 3 left, item 4 arrived.
	if stats.Items != 2 || stats.Removed != 2 || len(sink.envs) != 3 {
		t.Fatalf("stats=%+v events=%d, want two departures and one arrival", stats, len(sink.envs))
	}
	presents := map[bool]bool{}
	for _, env := range sink.envs {
		presents[env.GetAvailability().GetPresent()] = true
	}
	if !presents[true] || !presents[false] {
		t.Fatalf("expected both a present=true and a present=false event: %v", presents)
	}
}

func TestResolverFailureAbortsRunWithoutCompleting(t *testing.T) {
	f := &fakeProvider{pages: fullCatalogue}
	cache := store.NewInMemory()
	reg, _ := itemregistry.New(cache, "")
	sink := &captureSink{}
	resolver := failingResolver{}
	s, _ := New("jellyfin", f, cache, slog.Default(),
		WithEntryLookup(reg), WithEventsSink(sink), WithItemResolver(resolver))
	ctx := context.Background()

	if _, err := s.SyncAccount(ctx, "primary"); err == nil {
		t.Fatal("resolver failure swallowed")
	}
	// Page items land before resolution runs (mirroring contract-violation
	// semantics), but the run aborts before completing anything.
	items, err := s.ListItems(ctx, "primary")
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 1 || items[0].GetNativeId() != "1" {
		t.Fatalf("failed run must stop at the failing item: %d items", len(items))
	}
	if _, ok, _ := s.Manifest(ctx, "primary"); ok {
		t.Fatal("failed run advanced the completion marker")
	}
	if len(sink.envs) != 0 {
		t.Fatalf("failed run emitted events: %d", len(sink.envs))
	}
}

// failingResolver rejects the configured native id.
type failingResolver struct{}

func (failingResolver) Resolve(_ context.Context, _ string, item *slotsv1.CatalogueItem) error {
	return fmt.Errorf("identity service unavailable")
}

// TestListAccountsEnumeratesSynchronizedAccounts proves the read-only
// enumeration returns each distinct account that has any cached material
// under this provider, with no duplicates across item rows and manifests.
func TestListAccountsEnumeratesSynchronizedAccounts(t *testing.T) {
	ctx := context.Background()
	f := &fakeProvider{pages: []*slotsv1.CatalogueSyncResponse{
		{Items: []*slotsv1.CatalogueItem{movie("1"), movie("2")}},
	}}
	s, _ := newSync(t, f)

	if got, err := s.ListAccounts(ctx); err != nil || len(got) != 0 {
		t.Fatalf("accounts before any sync = %v err=%v, want none", got, err)
	}

	if _, err := s.SyncAccount(ctx, "primary"); err != nil {
		t.Fatalf("SyncAccount primary: %v", err)
	}
	if _, err := s.SyncAccount(ctx, "secondary"); err != nil {
		t.Fatalf("SyncAccount secondary: %v", err)
	}

	accounts, err := s.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	set := map[string]bool{}
	for _, a := range accounts {
		if set[a] {
			t.Fatalf("duplicate account %q in %v", a, accounts)
		}
		set[a] = true
	}
	if len(set) != 2 || !set["primary"] || !set["secondary"] {
		t.Fatalf("accounts = %v, want primary and secondary, exactly once", accounts)
	}
}
