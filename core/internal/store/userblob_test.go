package store_test

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/nem-git/abcmovies/core/internal/store"
)

func testDEK(t *testing.T) []byte {
	t.Helper()
	dek := make([]byte, 32)
	if _, err := rand.Read(dek); err != nil {
		t.Fatalf("generate DEK: %v", err)
	}
	return dek
}

func TestUserBlobStore_PutGetRoundTrip(t *testing.T) {
	inner := store.NewInMemory()
	blob := store.NewUserBlobStore(inner)
	dek := testDEK(t)
	ctx := store.ContextWithUserBlob(context.Background(), "user:alice", dek)

	if err := blob.Put(ctx, "key1", []byte("hello")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := blob.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("Get = %q, want %q", got, "hello")
	}
}

func TestUserBlobStore_EncryptedAtRest(t *testing.T) {
	inner := store.NewInMemory()
	blob := store.NewUserBlobStore(inner)
	dek := testDEK(t)
	ctx := store.ContextWithUserBlob(context.Background(), "user:alice", dek)

	plaintext := "sensitive watch history"
	if err := blob.Put(ctx, "key1", []byte(plaintext)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Read raw from the inner store — ciphertext should not contain plaintext.
	raw, err := inner.Get(ctx, "user:user:alice:key1")
	if err != nil {
		t.Fatalf("inner Get: %v", err)
	}
	if string(raw) == plaintext {
		t.Fatal("raw value on disk contains plaintext — not encrypted")
	}
}

func TestUserBlobStore_UserIsolation(t *testing.T) {
	inner := store.NewInMemory()
	blob := store.NewUserBlobStore(inner)

	dekA := testDEK(t)
	dekB := testDEK(t)
	ctxA := store.ContextWithUserBlob(context.Background(), "user:alice", dekA)
	ctxB := store.ContextWithUserBlob(context.Background(), "user:bob", dekB)

	// Alice stores a value.
	if err := blob.Put(ctxA, "history", []byte("alice-data")); err != nil {
		t.Fatalf("Put alice: %v", err)
	}

	// Bob stores a value under the same key.
	if err := blob.Put(ctxB, "history", []byte("bob-data")); err != nil {
		t.Fatalf("Put bob: %v", err)
	}

	// Alice reads her own data.
	got, err := blob.Get(ctxA, "history")
	if err != nil {
		t.Fatalf("Get alice: %v", err)
	}
	if string(got) != "alice-data" {
		t.Fatalf("alice got %q, want %q", got, "alice-data")
	}

	// Bob reads his own data.
	got, err = blob.Get(ctxB, "history")
	if err != nil {
		t.Fatalf("Get bob: %v", err)
	}
	if string(got) != "bob-data" {
		t.Fatalf("bob got %q, want %q", got, "bob-data")
	}

	// Bob cannot read Alice's data with his DEK.
	_, err = blob.Get(ctxB, "user:user:alice:history")
	if err == nil {
		t.Fatal("bob should not be able to read alice's data")
	}
}

func TestUserBlobStore_Delete(t *testing.T) {
	inner := store.NewInMemory()
	blob := store.NewUserBlobStore(inner)
	dek := testDEK(t)
	ctx := store.ContextWithUserBlob(context.Background(), "user:alice", dek)

	if err := blob.Put(ctx, "key1", []byte("val")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := blob.Delete(ctx, "key1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := blob.Get(ctx, "key1")
	if err == nil {
		t.Fatal("expected error after Delete")
	}
}

func TestUserBlobStore_List(t *testing.T) {
	inner := store.NewInMemory()
	blob := store.NewUserBlobStore(inner)
	dek := testDEK(t)
	ctx := store.ContextWithUserBlob(context.Background(), "user:alice", dek)

	if err := blob.Put(ctx, "history:movie1", []byte("v1")); err != nil {
		t.Fatalf("Put history:movie1: %v", err)
	}
	if err := blob.Put(ctx, "history:movie2", []byte("v2")); err != nil {
		t.Fatalf("Put history:movie2: %v", err)
	}
	if err := blob.Put(ctx, "prefs:lang", []byte("en")); err != nil {
		t.Fatalf("Put prefs:lang: %v", err)
	}

	keys, err := blob.List(ctx, "history:")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("List returned %d keys, want 2: %v", len(keys), keys)
	}
}

func TestUserBlobStore_NoDEKInContext(t *testing.T) {
	inner := store.NewInMemory()
	blob := store.NewUserBlobStore(inner)
	ctx := context.Background()

	err := blob.Put(ctx, "key1", []byte("val"))
	if err == nil {
		t.Fatal("expected error when no DEK in context")
	}
}

func TestUserBlobStore_Close(t *testing.T) {
	inner := store.NewInMemory()
	blob := store.NewUserBlobStore(inner)
	if err := blob.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
