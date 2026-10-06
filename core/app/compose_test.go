package app

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/registry"
	"github.com/nem-git/abcmovies/core/internal/store"
)

type fakeInvalidator struct {
	calls    [][2]string
	failWith error
}

func (f *fakeInvalidator) InvalidateAccount(provider, accountID string) error {
	f.calls = append(f.calls, [2]string{provider, accountID})
	return f.failWith
}

// fakePublisher records what the mux hands to the event path, so the tests
// observe the mux itself rather than the bus's audience fan-out rules.
type fakePublisher struct {
	published []*corev1.EventEnvelope
}

func (f *fakePublisher) Publish(env *corev1.EventEnvelope) {
	f.published = append(f.published, env)
}

func availabilityEnvelope(id, provider, accountID string) *corev1.EventEnvelope {
	return &corev1.EventEnvelope{
		Id:        id,
		Type:      corev1.EventType_EVENT_TYPE_AVAILABILITY_CHANGED,
		Audience:  corev1.EventAudience_EVENT_AUDIENCE_ACCOUNT,
		AccountId: accountID,
		Payload: &corev1.EventEnvelope_Availability{
			Availability: &corev1.AvailabilityEvent{
				AccountId: accountID,
				Provider:  provider,
			},
		},
	}
}

func TestMuxForwardsAvailabilityToBusAndInvalidator(t *testing.T) {
	pub := &fakePublisher{}
	inv := &fakeInvalidator{}
	mux := &eventMux{bus: pub, lib: inv, log: slog.Default()}

	mux.Publish(availabilityEnvelope("e1", "slot-a", "acct-1"))

	if len(pub.published) != 1 || pub.published[0].GetId() != "e1" {
		t.Fatalf("bus received %v, want exactly the forwarded envelope", pub.published)
	}
	if len(inv.calls) != 1 || inv.calls[0] != [2]string{"slot-a", "acct-1"} {
		t.Fatalf("invalidation calls = %v, want one (slot-a, acct-1)", inv.calls)
	}
}

func TestMuxWithoutLibraryStillForwards(t *testing.T) {
	pub := &fakePublisher{}
	mux := &eventMux{bus: pub, log: slog.Default()}

	mux.Publish(availabilityEnvelope("e2", "slot-b", "acct-2"))
	if len(pub.published) != 1 || pub.published[0].GetId() != "e2" {
		t.Fatalf("bus received %v, want the forwarded envelope", pub.published)
	}
}

// Events without an availability payload must reach neither consumer — the
// availability filter is the mux's own routing rule.
func TestMuxDropsNonAvailabilityEvents(t *testing.T) {
	pub := &fakePublisher{}
	inv := &fakeInvalidator{}
	mux := &eventMux{bus: pub, lib: inv, log: slog.Default()}

	mux.Publish(&corev1.EventEnvelope{Id: "e3"})

	if len(pub.published) != 0 || len(inv.calls) != 0 {
		t.Fatalf("non-availability event leaked: bus=%v invalidations=%v", pub.published, inv.calls)
	}
}

// An invalidation failure is logged and tolerated; the event still reaches
// the bus rather than being lost to a cache problem.
func TestMuxInvalidationFailureDoesNotDropEvent(t *testing.T) {
	pub := &fakePublisher{}
	inv := &fakeInvalidator{failWith: errors.New("cache unavailable")}
	mux := &eventMux{bus: pub, lib: inv, log: slog.Default()}

	mux.Publish(availabilityEnvelope("e4", "slot-c", "acct-3"))

	if len(pub.published) != 1 {
		t.Fatalf("bus received %v, want the forwarded envelope despite the invalidation failure", pub.published)
	}
}

// Regression fixture for the production wiring: before it existed,
// availability events reached the bus while no cached library was ever
// dropped, and every test passed because each test wired its own invalidator.
// Composing the slots for real must connect availability events to derived
// library invalidation, and the production entitlement predicate must keep an
// unknown account from leaking to subscribers.
func TestComposeSlotsAvailabilityEventInvalidatesCache(t *testing.T) {
	ctx := context.Background()
	reg := registry.NewInProcess()
	defer reg.Close()
	sourceCache := store.NewInMemory()
	cache := store.NewInMemory()
	bus := apiserver.NewInMemoryBus()
	rt, err := ComposeSlots(ctx, config.SlotsConfig{}, config.EnrichmentConfig{}, reg,
		sourceCache, store.NewInMemory(), store.NewInMemory(), cache, bus, slog.Default())
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	defer rt.Bus.Close()
	if rt.eventMux == nil {
		t.Fatal("production wiring did not expose the event mux")
	}

	// A derived-library entry as if a user had already read their library.
	if err := cache.Put(ctx, "lib/u/alice", []byte(`{"entries":[]}`)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	sub := bus.Subscribe("sub-1", "alice")
	defer bus.Unsubscribe("sub-1")

	rt.eventMux.Publish(availabilityEnvelope("e1", "jellyfin", "acct-1"))

	if _, err := cache.Get(ctx, "lib/u/alice"); err != store.ErrKeyNotFound {
		t.Fatalf("expected cached library dropped, got err=%v", err)
	}

	select {
	case e := <-sub:
		t.Fatalf("unknown account leaked to subscriber: %v", e.GetType())
	case <-time.After(100 * time.Millisecond):
	}
}

func TestComposeSlotsDerivedCacheLivesInTheCacheStore(t *testing.T) {
	reg := registry.NewInProcess()
	defer reg.Close()
	sourceCache := store.NewInMemory()
	cache := store.NewInMemory()

	rt, err := ComposeSlots(context.Background(), config.SlotsConfig{}, config.EnrichmentConfig{}, reg,
		sourceCache, store.NewInMemory(), store.NewInMemory(), cache, apiserver.NewInMemoryBus(), slog.Default())
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	defer rt.Bus.Close()

	if _, err := rt.Library.Library(context.Background(), "u1"); err != nil {
		t.Fatalf("derive library: %v", err)
	}
	libKeys, err := cache.List(context.Background(), "lib/u/")
	if err != nil {
		t.Fatalf("list cache store: %v", err)
	}
	if len(libKeys) == 0 {
		t.Fatal("derived library did not land in the cache store")
	}
	leaked, err := sourceCache.List(context.Background(), "lib/u/")
	if err != nil {
		t.Fatalf("list source cache: %v", err)
	}
	if len(leaked) != 0 {
		t.Fatalf("derived library leaked into the source cache: %v", strings.Join(leaked, ", "))
	}
}

func TestComposeSlotsEmptyConfig(t *testing.T) {
	reg := registry.NewInProcess()
	defer reg.Close()

	rt, err := ComposeSlots(context.Background(), config.SlotsConfig{}, config.EnrichmentConfig{}, reg,
		store.NewInMemory(), store.NewInMemory(), store.NewInMemory(), store.NewInMemory(), apiserver.NewInMemoryBus(), slog.Default())
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	defer rt.Bus.Close()

	if rt.Library == nil || rt.ItemRegistry == nil || rt.Bus == nil || rt.Queue == nil {
		t.Fatal("composed runtime has nil components")
	}
	// The enrichment drain always runs; with no slots configured its queue
	// simply stays empty (TECHNICAL-DECISIONS.md §1.28).
	if len(rt.Jobs) != 1 || rt.Jobs[0].Name != "enrichment-drain" {
		t.Fatalf("empty config jobs = %v, want only the enrichment drain", rt.Jobs)
	}
	// Enrichment is off by default: no configured catalogue slot means the
	// engine has nothing to consult (TECHNICAL-DECISIONS.md §1.25).
	if len(rt.Catalogues) != 0 {
		t.Fatalf("empty config enabled %d catalogue slots, want 0", len(rt.Catalogues))
	}
	entries, err := rt.Library.Library(context.Background(), "someone")
	if err != nil || len(entries) != 0 {
		t.Fatalf("fresh library = (%v, %v), want empty without error", entries, err)
	}
}
