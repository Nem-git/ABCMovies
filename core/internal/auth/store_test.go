package auth_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
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

func testAEAD(t *testing.T) cipher.AEAD {
	t.Helper()
	block, err := aes.NewCipher(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("NewGCM: %v", err)
	}
	return aead
}

func TestMemoryDEKCache_RoundTrip(t *testing.T) {
	dc := auth.NewMemoryDEKCache()

	dek := bytes.Repeat([]byte{1}, 32)
	if err := dc.StoreDEK("dek:session-a", dek); err != nil {
		t.Fatalf("StoreDEK: %v", err)
	}

	got := dc.GetDEK("dek:session-a")
	if !bytes.Equal(got, dek) {
		t.Fatalf("GetDEK = %v, want %v", got, dek)
	}

	// Mutating the returned slice must not corrupt the cached entry.
	got[0] = 0xff
	if again := dc.GetDEK("dek:session-a"); again[0] != 0x01 {
		t.Fatal("mutated caller slice leaked into the cached DEK")
	}
}

func TestMemoryDEKCache_GetDEK_Missing(t *testing.T) {
	dc := auth.NewMemoryDEKCache()

	if got := dc.GetDEK("nonexistent"); got != nil {
		t.Fatalf("GetDEK for missing session should return nil, got %v", got)
	}
}

func TestMemoryDEKCache_DeleteDEK(t *testing.T) {
	dc := auth.NewMemoryDEKCache()

	if err := dc.StoreDEK("dek:session-a", []byte("key")); err != nil {
		t.Fatalf("StoreDEK: %v", err)
	}
	// Deleting a missing key is a no-op.
	if err := dc.DeleteDEK("nonexistent"); err != nil {
		t.Fatalf("DeleteDEK missing: %v", err)
	}
	if err := dc.DeleteDEK("dek:session-a"); err != nil {
		t.Fatalf("DeleteDEK: %v", err)
	}
	if got := dc.GetDEK("dek:session-a"); got != nil {
		t.Fatalf("GetDEK after DeleteDEK = %v, want nil", got)
	}
}

func TestSealedDEKCache_RoundTrip(t *testing.T) {
	s := store.NewInMemory()
	dc, err := auth.NewSealedDEKCache(s, testAEAD(t))
	if err != nil {
		t.Fatalf("NewSealedDEKCache: %v", err)
	}

	dek := bytes.Repeat([]byte{2}, 32)
	if err := dc.StoreDEK("session-a", dek); err != nil {
		t.Fatalf("StoreDEK: %v", err)
	}

	// The stored blob must not contain the plaintext key material.
	blob, err := s.Get(context.Background(), "dek:session-a")
	if err != nil {
		t.Fatalf("raw Get: %v", err)
	}
	if bytes.Contains(blob, dek) {
		t.Fatal("sealed cache wrote plaintext DEK to the backing store")
	}

	if got := dc.GetDEK("session-a"); !bytes.Equal(got, dek) {
		t.Fatalf("GetDEK = %v, want %v", got, dek)
	}
}

func TestSealedDEKCache_GetDEK_Missing(t *testing.T) {
	dc, err := auth.NewSealedDEKCache(store.NewInMemory(), testAEAD(t))
	if err != nil {
		t.Fatalf("NewSealedDEKCache: %v", err)
	}

	if got := dc.GetDEK("nonexistent"); got != nil {
		t.Fatalf("GetDEK for missing session should return nil, got %v", got)
	}
}

func TestSealedDEKCache_DeleteDEK(t *testing.T) {
	s := store.NewInMemory()
	dc, err := auth.NewSealedDEKCache(s, testAEAD(t))
	if err != nil {
		t.Fatalf("NewSealedDEKCache: %v", err)
	}

	if err := dc.StoreDEK("session-a", []byte("key")); err != nil {
		t.Fatalf("StoreDEK: %v", err)
	}
	if err := dc.DeleteDEK("session-a"); err != nil {
		t.Fatalf("DeleteDEK: %v", err)
	}
	if _, err := s.Get(context.Background(), "dek:session-a"); err == nil {
		t.Fatal("sealed entry still present after DeleteDEK")
	}
}

func TestSealedDEKCache_RequiresStoreAndCipher(t *testing.T) {
	if _, err := auth.NewSealedDEKCache(nil, testAEAD(t)); err == nil {
		t.Fatal("expected error without backing store")
	}
	if _, err := auth.NewSealedDEKCache(store.NewInMemory(), nil); err == nil {
		t.Fatal("expected error without cipher")
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
