package store

import (
	"bytes"
	"context"
	"testing"
)

// The login store holds password hashes; its bytes must never appear in the
// backing store, only under the instance key.
func TestSealedStore_InnerNeverHoldsPlaintext(t *testing.T) {
	inner := NewInMemory()
	s, err := NewSealed(inner, testAEAD(t), nil)
	if err != nil {
		t.Fatalf("NewSealed: %v", err)
	}
	plain := []byte(`{"PasswordHash":"needle"}`)
	if err := s.Put(context.Background(), "user:alice", plain); err != nil {
		t.Fatalf("Put: %v", err)
	}
	raw, err := inner.Get(context.Background(), "user:alice")
	if err != nil {
		t.Fatalf("inner Get: %v", err)
	}
	if bytes.Contains(raw, []byte("needle")) {
		t.Fatal("inner store holds plaintext")
	}
	got, err := s.Get(context.Background(), "user:alice")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatal("sealed round trip did not return the plaintext")
	}
}

// A record sealed under one key cannot open under another: this is what
// turns a wrong stores.vault-key into a fail-closed boot notice instead of
// a silently empty account table.
func TestSealedStore_WrongKeyFailsClosed(t *testing.T) {
	inner := NewInMemory()
	s1, err := NewSealed(inner, testAEAD(t), nil)
	if err != nil {
		t.Fatalf("NewSealed: %v", err)
	}
	if err := s1.Put(context.Background(), "user:alice", []byte("v")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	s2, err := NewSealed(inner, testAEADDifferentKey(t), nil)
	if err != nil {
		t.Fatalf("NewSealed: %v", err)
	}
	if _, err := s2.Get(context.Background(), "user:alice"); err == nil {
		t.Fatal("record opened under a different key")
	}
}
