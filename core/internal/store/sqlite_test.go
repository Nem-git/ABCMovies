package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"path/filepath"
	"testing"
)

func TestSQLite(t *testing.T) {
	RunStoreSuite(t, func(t *testing.T) Store {
		t.Helper()
		return newTestSQLite(t)
	})
}

func TestSQLite_CreatesMissingParentDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "deeper", "test.db")
	s, err := NewSQLite(t.Context(), path)
	if err != nil {
		t.Fatalf("NewSQLite with missing parents: %v", err)
	}
	defer func() { _ = s.Close() }()
	if err := s.Put(t.Context(), "k", []byte("v")); err != nil {
		t.Fatalf("Put: %v", err)
	}
}

func TestVault_CreatesMissingParentDirs(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	gcm, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(gcm)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "nested", "vault.db")
	v, err := NewVault(t.Context(), path, aead)
	if err != nil {
		t.Fatalf("NewVault with missing parents: %v", err)
	}
	defer func() { _ = v.Close() }()
	if err := v.Put(t.Context(), "k", []byte("v")); err != nil {
		t.Fatalf("Put: %v", err)
	}
}

func TestSQLite_ClosedStoreReturnsErrors(t *testing.T) {
	s := newTestSQLite(t)
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

func TestSQLite_PersistenceAcrossReopen(t *testing.T) {
	path := t.TempDir() + "/test.db"
	ctx := t.Context()

	s, err := NewSQLite(ctx, path)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	if err := s.Put(ctx, "persist-key", []byte("persist-value")); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	s2, err := NewSQLite(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = s2.Close() }()

	got, err := s2.Get(ctx, "persist-key")
	if err != nil {
		t.Fatalf("get after reopen: %v", err)
	}
	if string(got) != "persist-value" {
		t.Fatalf("got %q, want %q", got, "persist-value")
	}
}

func newTestSQLite(t *testing.T) *SQLite {
	t.Helper()
	path := t.TempDir() + "/store.db"
	s, err := NewSQLite(t.Context(), path)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
