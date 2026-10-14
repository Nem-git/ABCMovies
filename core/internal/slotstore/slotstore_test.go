package slotstore

import (
	"context"
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/store"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(store.NewInMemory())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func providerSlot(id string) *corev1.Slot {
	s := &corev1.Slot{
		Id:          id,
		Kind:        corev1.SlotKind_SLOT_KIND_PROVIDER,
		OwnerUserId: "",
	}
	s.Config = &corev1.Slot_Provider{
		Provider: &corev1.ProviderConfig{
			Adapter: "jellyfin",
			Server:  "https://jf.example.com",
		},
	}
	return s
}

func sinkSlot(id string) *corev1.Slot {
	s := &corev1.Slot{
		Id:   id,
		Kind: corev1.SlotKind_SLOT_KIND_SINK,
	}
	s.Config = &corev1.Slot_Sink{Sink: &corev1.SinkConfig{Adapter: "relay"}}
	return s
}

func catalogueSlot(id string) *corev1.Slot {
	s := &corev1.Slot{
		Id:   id,
		Kind: corev1.SlotKind_SLOT_KIND_CATALOGUE,
	}
	s.Config = &corev1.Slot_Catalogue{
		Catalogue: &corev1.CatalogueConfig{Adapter: "tmdb", TokenEnv: "TMDB_TOKEN"},
	}
	return s
}

func TestPutGetRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.Put(ctx, providerSlot("jf")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := s.Get(ctx, corev1.SlotKind_SLOT_KIND_PROVIDER, "", "jf")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.GetId() != "jf" || got.GetProvider().GetServer() != "https://jf.example.com" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.GetProvider().GetAdapter() != "jellyfin" {
		t.Errorf("adapter = %q, want jellyfin", got.GetProvider().GetAdapter())
	}
}

func TestGetAbsentReturnsNil(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	got, err := s.Get(ctx, corev1.SlotKind_SLOT_KIND_PROVIDER, "", "nope")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Fatalf("Get absent = %+v, want nil", got)
	}
}

func TestDeleteRemoves(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.Put(ctx, providerSlot("jf")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := s.Delete(ctx, corev1.SlotKind_SLOT_KIND_PROVIDER, "", "jf"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, err := s.Get(ctx, corev1.SlotKind_SLOT_KIND_PROVIDER, "", "jf")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Fatalf("Get after delete = %+v, want nil", got)
	}
}

// TestPutRejectsCrossOwnerDuplicateID pins that the slot id is the identity
// namespace (PLAN.md §1.25): it is globally unique. A second owner claiming
// an id another slot already holds is refused, and the first record is left
// untouched.
func TestPutRejectsCrossOwnerDuplicateID(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	alice := providerSlot("jf")
	alice.OwnerUserId = "user:alice"
	bob := providerSlot("jf")
	bob.OwnerUserId = "user:bob"
	if err := s.Put(ctx, alice); err != nil {
		t.Fatalf("Put alice: %v", err)
	}
	if err := s.Put(ctx, bob); err == nil {
		t.Fatal("Put bob should refuse an id alice already holds")
	}
	got, err := s.Get(ctx, corev1.SlotKind_SLOT_KIND_PROVIDER, "user:alice", "jf")
	if err != nil {
		t.Fatalf("Get alice: %v", err)
	}
	if got.GetOwnerUserId() != "user:alice" {
		t.Errorf("owner = %q, want user:alice", got.GetOwnerUserId())
	}
}

// TestPutRejectsCrossKindDuplicateID pins that the id is unique across kinds
// too: a provider and a catalogue may not share an id.
func TestPutRejectsCrossKindDuplicateID(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	prov := providerSlot("shared")
	cat := catalogueSlot("shared")
	if err := s.Put(ctx, prov); err != nil {
		t.Fatalf("Put provider: %v", err)
	}
	if err := s.Put(ctx, cat); err == nil {
		t.Fatal("Put catalogue should refuse an id the provider holds")
	}
}

func TestListByKindFiltersOwner(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	alice := providerSlot("jf-alice")
	alice.OwnerUserId = "user:alice"
	bob := providerSlot("jf-bob")
	bob.OwnerUserId = "user:bob"
	sink := sinkSlot("dev")
	for _, slot := range []*corev1.Slot{alice, bob, sink} {
		if err := s.Put(ctx, slot); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}
	all, err := s.ListByKind(ctx, corev1.SlotKind_SLOT_KIND_PROVIDER, "")
	if err != nil {
		t.Fatalf("ListByKind: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("all providers = %d, want 2", len(all))
	}
	aliceSlots, err := s.ListByKind(ctx, corev1.SlotKind_SLOT_KIND_PROVIDER, "user:alice")
	if err != nil {
		t.Fatalf("ListByKind alice: %v", err)
	}
	if len(aliceSlots) != 1 {
		t.Errorf("alice providers = %d, want 1", len(aliceSlots))
	}
	sinks, err := s.ListByKind(ctx, corev1.SlotKind_SLOT_KIND_SINK, "")
	if err != nil {
		t.Fatalf("ListByKind sinks: %v", err)
	}
	if len(sinks) != 1 {
		t.Errorf("sinks = %d, want 1", len(sinks))
	}
}

func TestPutRejectsInvalidSlot(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	bad := providerSlot("jf")
	bad.GetProvider().Server = ""
	if err := s.Put(ctx, bad); err == nil {
		t.Fatal("Put should reject a provider without a server")
	}
}
