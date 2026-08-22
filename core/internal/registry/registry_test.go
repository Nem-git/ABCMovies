package registry_test

import (
	"context"
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/registry"
)

// testSlot is a minimal MetaService implementation for tests.
type testSlot struct {
	corev1.UnimplementedMetaServiceServer
	caps []*corev1.Capability
}

func (s *testSlot) CapabilityQuery(_ context.Context, _ *corev1.CapabilityQueryRequest) (*corev1.CapabilityQueryResponse, error) {
	return &corev1.CapabilityQueryResponse{Capabilities: s.caps}, nil
}

func TestRegistry_Admit_Success(t *testing.T) {
	r := registry.NewInProcess()
	defer r.Close()

	caps, err := r.Admit("test-slot", &testSlot{caps: []*corev1.Capability{
		{Name: "test", Version: 1},
	}})
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}
	if len(caps) != 1 {
		t.Fatalf("got %d capabilities, want 1", len(caps))
	}
	if caps[0].Name != "test" || caps[0].Version != 1 {
		t.Fatalf("capability = %+v, want {Name:test Version:1}", caps[0])
	}
}

func TestRegistry_Admit_Duplicate(t *testing.T) {
	r := registry.NewInProcess()
	defer r.Close()

	slot := &testSlot{caps: []*corev1.Capability{{Name: "a", Version: 1}}}
	_, err := r.Admit("slot-1", slot)
	if err != nil {
		t.Fatalf("first Admit: %v", err)
	}

	_, err = r.Admit("slot-1", slot)
	if err == nil {
		t.Fatal("expected error for duplicate slot name")
	}
}

func TestRegistry_Admit_InvalidCapability(t *testing.T) {
	r := registry.NewInProcess()
	defer r.Close()

	// Empty name should be rejected.
	_, err := r.Admit("bad-slot", &testSlot{caps: []*corev1.Capability{
		{Name: "", Version: 1},
	}})
	if err == nil {
		t.Fatal("expected error for empty capability name")
	}

	// Zero version should be rejected.
	_, err = r.Admit("bad-slot-2", &testSlot{caps: []*corev1.Capability{
		{Name: "valid", Version: 0},
	}})
	if err == nil {
		t.Fatal("expected error for zero capability version")
	}
}

func TestRegistry_Capabilities(t *testing.T) {
	r := registry.NewInProcess()
	defer r.Close()

	// Unknown slot.
	_, ok := r.Capabilities("no-such-slot")
	if ok {
		t.Fatal("expected false for unknown slot")
	}

	// Known slot.
	if _, err := r.Admit("my-slot", &testSlot{caps: []*corev1.Capability{
		{Name: "alpha", Version: 2},
		{Name: "beta", Version: 3},
	}}); err != nil {
		t.Fatalf("Admit my-slot: %v", err)
	}
	caps, ok := r.Capabilities("my-slot")
	if !ok {
		t.Fatal("expected true for known slot")
	}
	if len(caps) != 2 {
		t.Fatalf("got %d capabilities, want 2", len(caps))
	}
}

func TestRegistry_MultipleSlots(t *testing.T) {
	r := registry.NewInProcess()
	defer r.Close()

	_, err := r.Admit("slot-a", &testSlot{caps: []*corev1.Capability{{Name: "a", Version: 1}}})
	if err != nil {
		t.Fatalf("Admit slot-a: %v", err)
	}
	_, err = r.Admit("slot-b", &testSlot{caps: []*corev1.Capability{{Name: "b", Version: 2}}})
	if err != nil {
		t.Fatalf("Admit slot-b: %v", err)
	}

	capsA, ok := r.Capabilities("slot-a")
	if !ok || len(capsA) != 1 || capsA[0].Name != "a" {
		t.Fatalf("slot-a capabilities: %+v", capsA)
	}
	capsB, ok := r.Capabilities("slot-b")
	if !ok || len(capsB) != 1 || capsB[0].Name != "b" {
		t.Fatalf("slot-b capabilities: %+v", capsB)
	}
}
