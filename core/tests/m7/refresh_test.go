// Refresh composition for lazy providers (M7 slice): RefreshItems applies the
// explicit provider lookup to the source cache and fans out the same
// account-scoped availability events a catalogue sync emits.
package m7_test

import (
	"context"
	"log/slog"
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/schema"
	"github.com/nem-git/abcmovies/core/internal/sourcecache"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// fakeLazySpeaksBoth is the client fixture: catalogue sync is refused, the
// lazy refresh path answers from a mutable item map.
type fakeLazySpeaksBoth struct {
	*fakeRefresh
}

func (f *fakeLazySpeaksBoth) CatalogueSync(_ context.Context, _ *slotsv1.CatalogueSyncRequest) (*slotsv1.CatalogueSyncResponse, error) {
	return nil, nil // lazy providers never catalogue-sync; unreachable in these fixtures
}

func (f *fakeLazySpeaksBoth) RefreshAvailability(_ context.Context, req *slotsv1.RefreshAvailabilityRequest) (*slotsv1.RefreshAvailabilityResponse, error) {
	out := []*slotsv1.CatalogueItem{}
	for _, id := range req.GetNativeIds() {
		if it, ok := f.items[id]; ok {
			out = append(out, it)
		}
	}
	return &slotsv1.RefreshAvailabilityResponse{Items: out}, nil
}

type fakeRefresh struct {
	items map[string]*slotsv1.CatalogueItem
}

type eventCapture struct{ envs []*corev1.EventEnvelope }

func (c *eventCapture) Publish(env *corev1.EventEnvelope) { c.envs = append(c.envs, env) }

type entryLookup struct{}

func (entryLookup) Lookup(_ context.Context, _, nativeID string) (string, bool, error) {
	return "entry-" + nativeID, true, nil
}

func availItems() map[string]*slotsv1.CatalogueItem {
	return map[string]*slotsv1.CatalogueItem{
		"movie:tt1": {NativeId: "movie:tt1", Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE, Metadata: &corev1.TitleMetadata{Title: "One"}},
		"movie:tt2": {NativeId: "movie:tt2", Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE, Metadata: &corev1.TitleMetadata{Title: "Two"}},
	}
}

// The refresh upserts carried items, drops unanswered ones, and emits one
// account-scoped availability event per arrival and departure.
func TestRefreshItemsAppliesPresenceAndEmits(t *testing.T) {
	cache := store.NewInMemory()
	sink := &eventCapture{}
	client := &fakeLazySpeaksBoth{&fakeRefresh{items: availItems()}}
	s, err := sourcecache.New("stremio", client, cache, slog.Default(),
		sourcecache.WithRefreshClient(client),
		sourcecache.WithEventsSink(sink),
		sourcecache.WithEntryLookup(entryLookup{}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	stats, err := s.RefreshItems(context.Background(), "acct", []string{"movie:tt1"})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Items != 1 || stats.Removed != 0 {
		t.Fatalf("stats = %+v", stats)
	}
	if len(sink.envs) != 1 || !sink.envs[0].GetAvailability().GetPresent() {
		t.Fatalf("events = %v", sink.envs)
	}
	if _, err := cache.Get(context.Background(), "stremio/acct/movie:tt1"); err != nil {
		t.Fatal("item not cached")
	}

	// tt1 delisted, tt2 appears: one departure event, one arrival event.
	delete(client.items, "movie:tt1")
	client.items["movie:tt2"] = &slotsv1.CatalogueItem{NativeId: "movie:tt2", Kind: slotsv1.ItemKind_ITEM_KIND_MOVIE, Metadata: &corev1.TitleMetadata{Title: "Two"}}
	stats, err = s.RefreshItems(context.Background(), "acct", []string{"movie:tt1", "movie:tt2"})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Items != 1 || stats.Removed != 1 {
		t.Fatalf("stats = %+v", stats)
	}
	if _, err := cache.Get(context.Background(), "stremio/acct/movie:tt1"); err == nil {
		t.Fatal("tt1 must be gone from cache")
	}
	if _, err := cache.Get(context.Background(), "stremio/acct/movie:tt2"); err != nil {
		t.Fatal("tt2 must be cached")
	}
	var departures, arrivals int
	for _, env := range sink.envs {
		if a := env.GetAvailability(); a != nil {
			if a.GetPresent() {
				arrivals++
			} else {
				departures++
			}
		}
	}
	if departures != 1 || arrivals != 2 {
		t.Fatalf("events: departures=%d arrivals=%d, want 1/2", departures, arrivals)
	}
}

// An invalid refresh response is a contract violation: nothing is written,
// nothing is deleted (§2.5: reject, never downgrade).
func TestRefreshItemsRejectsContractViolations(t *testing.T) {
	cache := store.NewInMemory()
	client := &fakeLazySpeaksBoth{&fakeRefresh{items: map[string]*slotsv1.CatalogueItem{
		"bad": {NativeId: "bad"}, // missing kind and metadata
	}}}
	s, err := sourcecache.New("stremio", client, cache, slog.Default(),
		sourcecache.WithRefreshClient(client),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := s.RefreshItems(context.Background(), "acct", []string{"bad"}); err == nil {
		t.Fatal("expected contract violation")
	}
}

// Referenced so the import stays warm for future validation-helper changes.
var _ = schema.ValidateCatalogueItem
