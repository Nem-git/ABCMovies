package contentkeycache

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/store"
)

func newCache(t *testing.T) *Cache {
	t.Helper()
	c, err := New("widevine", store.NewInMemory(), slog.Default())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func testKey(id, material string) *slotsv1.ContentKey {
	return &slotsv1.ContentKey{
		KeyId:       id,
		KeyMaterial: []byte(material),
		Validity:    durationpb.New(time.Hour),
	}
}

func TestPutAndGetKeys(t *testing.T) {
	ctx := context.Background()
	c := newCache(t)

	keys := []*slotsv1.ContentKey{testKey("kid1", "material-one"), testKey("kid2", "material-two")}
	if err := c.PutKeys(ctx, "movie123", keys); err != nil {
		t.Fatalf("PutKeys: %v", err)
	}

	got, err := c.GetKeys(ctx, "movie123")
	if err != nil {
		t.Fatalf("GetKeys: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d keys, want 2", len(got))
	}
	if got[0].GetKeyId() != "kid1" || string(got[0].GetKeyMaterial()) != "material-one" {
		t.Fatalf("key[0] = %v", got[0])
	}
	if got[1].GetKeyId() != "kid2" || string(got[1].GetKeyMaterial()) != "material-two" {
		t.Fatalf("key[1] = %v", got[1])
	}
}

func TestGetKeysEmpty(t *testing.T) {
	ctx := context.Background()
	c := newCache(t)

	got, err := c.GetKeys(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetKeys: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d keys, want 0", len(got))
	}
}

func TestDropKeys(t *testing.T) {
	ctx := context.Background()
	c := newCache(t)

	keys := []*slotsv1.ContentKey{testKey("kid1", "material-one")}
	if err := c.PutKeys(ctx, "movie123", keys); err != nil {
		t.Fatalf("PutKeys: %v", err)
	}
	if err := c.DropKeys(ctx, "movie123"); err != nil {
		t.Fatalf("DropKeys: %v", err)
	}
	got, err := c.GetKeys(ctx, "movie123")
	if err != nil {
		t.Fatalf("GetKeys: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d keys after drop, want 0", len(got))
	}
}

func TestDropKey(t *testing.T) {
	ctx := context.Background()
	c := newCache(t)

	keys := []*slotsv1.ContentKey{testKey("kid1", "material-one"), testKey("kid2", "material-two")}
	if err := c.PutKeys(ctx, "movie123", keys); err != nil {
		t.Fatalf("PutKeys: %v", err)
	}
	if err := c.DropKey(ctx, "movie123", "kid1"); err != nil {
		t.Fatalf("DropKey: %v", err)
	}
	got, err := c.GetKeys(ctx, "movie123")
	if err != nil {
		t.Fatalf("GetKeys: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d keys after drop, want 1", len(got))
	}
	if got[0].GetKeyId() != "kid2" {
		t.Fatalf("remaining key = %q, want kid2", got[0].GetKeyId())
	}
}

func TestExpiredKeysDropped(t *testing.T) {
	ctx := context.Background()
	c := newCache(t)

	expired := &slotsv1.ContentKey{
		KeyId:       "kid1",
		KeyMaterial: []byte("material-one"),
		Validity:    durationpb.New(-time.Hour),
	}
	if err := c.PutKeys(ctx, "movie123", []*slotsv1.ContentKey{expired}); err != nil {
		t.Fatalf("PutKeys: %v", err)
	}
	got, err := c.GetKeys(ctx, "movie123")
	if err != nil {
		t.Fatalf("GetKeys: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d keys, want 0 (expired)", len(got))
	}
}

func TestPutKeysValidation(t *testing.T) {
	ctx := context.Background()
	c := newCache(t)

	if err := c.PutKeys(ctx, "movie123", []*slotsv1.ContentKey{{KeyMaterial: []byte("m")}}); err == nil {
		t.Fatal("expected error for missing key_id")
	}
	if err := c.PutKeys(ctx, "movie123", []*slotsv1.ContentKey{{KeyId: "k"}}); err == nil {
		t.Fatal("expected error for missing key_material")
	}
}

func TestProvider(t *testing.T) {
	c := newCache(t)
	if c.Provider() != "widevine" {
		t.Fatalf("Provider = %q, want widevine", c.Provider())
	}
}
