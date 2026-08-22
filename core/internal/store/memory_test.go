package store

import "testing"

func TestInMemory(t *testing.T) {
	RunStoreSuite(t, func(t *testing.T) Store {
		t.Helper()
		return NewInMemory()
	})
}

func TestInMemory_ClosedStoreReturnsErrors(t *testing.T) {
	s := NewInMemory()
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	ctx := t.Context()
	if err := s.Put(ctx, "k", []byte("v")); err == nil {
		t.Fatal("expected error on Put after Close")
	}
	if _, err := s.Get(ctx, "k"); err == nil {
		t.Fatal("expected error on Get after Close")
	}
	if err := s.Delete(ctx, "k"); err == nil {
		t.Fatal("expected error on Delete after Close")
	}
	if _, err := s.List(ctx, ""); err == nil {
		t.Fatal("expected error on List after Close")
	}
}
