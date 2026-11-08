package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestAdminLifecycle(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	n, err := s.AdminCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("AdminCount = %d, want 0", n)
	}

	if err := s.CreateAdmin(ctx, "admin", "hash"); err != nil {
		t.Fatal(err)
	}
	n, err = s.AdminCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("AdminCount = %d, want 1", n)
	}

	admin, err := s.GetAdmin(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if admin.Hash != "hash" {
		t.Fatalf("Hash = %q, want %q", admin.Hash, "hash")
	}

	if err := s.CreateAdmin(ctx, "admin", "hash"); err == nil {
		t.Fatal("duplicate CreateAdmin unexpectedly succeeded")
	}
}

func TestSessionLifecycle(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	if err := s.CreateAdmin(ctx, "admin", "hash"); err != nil {
		t.Fatal(err)
	}

	sess, err := s.CreateSession(ctx, "admin", 7*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.ValidateSession(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "admin" {
		t.Fatalf("Username = %q, want admin", got.Username)
	}

	if err := s.DeleteSession(ctx, sess.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ValidateSession(ctx, sess.ID); err == nil {
		t.Fatal("ValidateSession after DeleteSession unexpectedly succeeded")
	}
}