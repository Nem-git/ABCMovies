package auth_test

import (
	"context"
	"testing"

	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/store"
)

func TestStoreTokenStore_RoundTrip(t *testing.T) {
	s := store.NewInMemory()
	ts := auth.NewStoreTokenStore(s)
	ctx := context.Background()

	if err := ts.Save(ctx, "tok:abc", []byte("value")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := ts.Load(ctx, "tok:abc")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(got) != "value" {
		t.Fatalf("Load = %q, want %q", got, "value")
	}

	if err := ts.Delete(ctx, "tok:abc"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = ts.Load(ctx, "tok:abc")
	if err == nil {
		t.Fatal("expected error after Delete")
	}
}

func TestStoreTokenStore_Load_NotFound(t *testing.T) {
	s := store.NewInMemory()
	ts := auth.NewStoreTokenStore(s)

	_, err := ts.Load(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestStoreDEKCache_RoundTrip(t *testing.T) {
	s := store.NewInMemory()
	dc := auth.NewStoreDEKCache(s)

	dek := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	dc.StoreDEK("user:user:alice", dek)

	got := dc.GetDEK("user:user:alice")
	if len(got) == 0 {
		t.Fatal("GetDEK should return the stored DEK")
	}
	if len(got) != len(dek) {
		t.Fatalf("GetDEK length = %d, want %d", len(got), len(dek))
	}
	for i := range got {
		if got[i] != dek[i] {
			t.Fatalf("GetDEK[%d] = %d, want %d", i, got[i], dek[i])
		}
	}
}

func TestStoreDEKCache_GetDEK_Missing(t *testing.T) {
	s := store.NewInMemory()
	dc := auth.NewStoreDEKCache(s)

	got := dc.GetDEK("nonexistent")
	if got != nil {
		t.Fatalf("GetDEK for missing user should return nil, got %v", got)
	}
}

func TestStoreUserStore_RoundTrip(t *testing.T) {
	s := store.NewInMemory()
	us := auth.NewStoreUserStore(s)

	data := &auth.UserData{
		Salt:            []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		PasswordHash:    []byte("password-kek"),
		WrappedDEK:      []byte("wrapped-dek"),
		WrappedRecovery: []byte("wrapped-recovery"),
	}
	if err := us.PutUser("alice", data); err != nil {
		t.Fatalf("PutUser: %v", err)
	}

	got, err := us.GetUser("alice")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if len(got.Salt) != len(data.Salt) {
		t.Fatalf("Salt length = %d, want %d", len(got.Salt), len(data.Salt))
	}
	if string(got.PasswordHash) != string(data.PasswordHash) {
		t.Fatalf("PasswordHash = %q, want %q", got.PasswordHash, data.PasswordHash)
	}
	if string(got.WrappedDEK) != string(data.WrappedDEK) {
		t.Fatalf("WrappedDEK = %q, want %q", got.WrappedDEK, data.WrappedDEK)
	}
	if string(got.WrappedRecovery) != string(data.WrappedRecovery) {
		t.Fatalf("WrappedRecovery = %q, want %q", got.WrappedRecovery, data.WrappedRecovery)
	}
}

func TestStoreUserStore_GetUser_NotFound(t *testing.T) {
	s := store.NewInMemory()
	us := auth.NewStoreUserStore(s)

	_, err := us.GetUser("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

func TestStoreUserStore_PutUser_Corrupt(t *testing.T) {
	s := store.NewInMemory()
	us := auth.NewStoreUserStore(s)

	// Write raw garbage to the underlying store.
	if err := s.Put(context.Background(), "user:alice", []byte("not-json")); err != nil {
		t.Fatalf("direct Put: %v", err)
	}

	_, err := us.GetUser("alice")
	if err == nil {
		t.Fatal("expected error for corrupted user data")
	}
}
