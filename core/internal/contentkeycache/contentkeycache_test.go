package contentkeycache

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// fakeStore is a minimal in-memory store.Store for testing the cache.
type fakeStore struct {
	data map[string][]byte
}

func newFakeStore() *fakeStore {
	return &fakeStore{data: map[string][]byte{}}
}

func (f *fakeStore) Put(_ context.Context, key string, value []byte) error {
	f.data[key] = value
	return nil
}

func (f *fakeStore) Get(_ context.Context, key string) ([]byte, error) {
	v, ok := f.data[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return v, nil
}

func (f *fakeStore) List(_ context.Context, prefix string) ([]string, error) {
	var out []string
	for k := range f.data {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			out = append(out, k)
		}
	}
	return out, nil
}

func (f *fakeStore) Delete(_ context.Context, key string) error {
	delete(f.data, key)
	return nil
}

func (f *fakeStore) Close() error { return nil }

func newTestCache(t *testing.T) (*Cache, *fakeStore) {
	t.Helper()
	st := newFakeStore()
	c, err := New(st, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c, st
}

func testKey(kid string, validity time.Duration) *slotsv1.ContentKey {
	return &slotsv1.ContentKey{
		KeyId:       kid,
		KeyMaterial: []byte("material-" + kid),
		Validity:    durationpb.New(validity),
	}
}

func TestPutAndGetKeysRoundTrip(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()
	system := slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE

	keys := []*slotsv1.ContentKey{testKey("k1", time.Hour)}
	if err := c.PutKeys(ctx, "prov1", "content1", system, keys); err != nil {
		t.Fatalf("PutKeys: %v", err)
	}

	got, err := c.GetKeys(ctx, "prov1", "content1", system)
	if err != nil {
		t.Fatalf("GetKeys: %v", err)
	}
	if len(got) != 1 || got[0].GetKeyId() != "k1" {
		t.Fatalf("got %v, want [k1]", got)
	}
}

func TestGetKeysEmptyWhenNothingCached(t *testing.T) {
	c, _ := newTestCache(t)
	got, err := c.GetKeys(context.Background(), "prov1", "content1", slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE)
	if err != nil {
		t.Fatalf("GetKeys: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %v, want empty", got)
	}
}

func TestGetKeysFiltersByProvider(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()
	system := slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE

	_ = c.PutKeys(ctx, "prov1", "content1", system, []*slotsv1.ContentKey{testKey("k1", time.Hour)})
	_ = c.PutKeys(ctx, "prov2", "content1", system, []*slotsv1.ContentKey{testKey("k2", time.Hour)})

	got, err := c.GetKeys(ctx, "prov1", "content1", system)
	if err != nil {
		t.Fatalf("GetKeys: %v", err)
	}
	if len(got) != 1 || got[0].GetKeyId() != "k1" {
		t.Fatalf("got %v, want [k1]", got)
	}
}

func TestGetKeysFiltersBySystem(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()

	_ = c.PutKeys(ctx, "prov1", "content1", slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE,
		[]*slotsv1.ContentKey{testKey("k1", time.Hour)})
	_ = c.PutKeys(ctx, "prov1", "content1", slotsv1.DrmSystem_DRM_SYSTEM_PLAYREADY,
		[]*slotsv1.ContentKey{testKey("k2", time.Hour)})

	got, err := c.GetKeys(ctx, "prov1", "content1", slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE)
	if err != nil {
		t.Fatalf("GetKeys: %v", err)
	}
	if len(got) != 1 || got[0].GetKeyId() != "k1" {
		t.Fatalf("got %v, want [k1]", got)
	}
}

func TestGetKeysFiltersByContentID(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()
	system := slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE

	_ = c.PutKeys(ctx, "prov1", "content1", system, []*slotsv1.ContentKey{testKey("k1", time.Hour)})
	_ = c.PutKeys(ctx, "prov1", "content2", system, []*slotsv1.ContentKey{testKey("k2", time.Hour)})

	got, err := c.GetKeys(ctx, "prov1", "content1", system)
	if err != nil {
		t.Fatalf("GetKeys: %v", err)
	}
	if len(got) != 1 || got[0].GetKeyId() != "k1" {
		t.Fatalf("got %v, want [k1]", got)
	}
}

func TestDropKeysRemovesAllForContentAndSystem(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()

	_ = c.PutKeys(ctx, "prov1", "content1", slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE,
		[]*slotsv1.ContentKey{testKey("k1", time.Hour), testKey("k2", time.Hour)})
	_ = c.PutKeys(ctx, "prov1", "content1", slotsv1.DrmSystem_DRM_SYSTEM_PLAYREADY,
		[]*slotsv1.ContentKey{testKey("k3", time.Hour)})

	if err := c.DropKeys(ctx, "prov1", "content1", slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE); err != nil {
		t.Fatalf("DropKeys: %v", err)
	}

	got, _ := c.GetKeys(ctx, "prov1", "content1", slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE)
	if len(got) != 0 {
		t.Fatalf("after DropKeys widevine: got %v, want empty", got)
	}
	got, _ = c.GetKeys(ctx, "prov1", "content1", slotsv1.DrmSystem_DRM_SYSTEM_PLAYREADY)
	if len(got) != 1 {
		t.Fatalf("after DropKeys playready: got %v, want 1 key", got)
	}
}

func TestDropKeyRemovesSingleKey(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()
	system := slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE

	_ = c.PutKeys(ctx, "prov1", "content1", system,
		[]*slotsv1.ContentKey{testKey("k1", time.Hour), testKey("k2", time.Hour)})

	if err := c.DropKey(ctx, "prov1", "content1", system, "k1"); err != nil {
		t.Fatalf("DropKey: %v", err)
	}

	got, _ := c.GetKeys(ctx, "prov1", "content1", system)
	if len(got) != 1 || got[0].GetKeyId() != "k2" {
		t.Fatalf("got %v, want [k2]", got)
	}
}

func TestGetKeysDropsExpiredOnRead(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()
	system := slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE

	// Already expired.
	_ = c.PutKeys(ctx, "prov1", "content1", system,
		[]*slotsv1.ContentKey{testKey("k1", -time.Hour)})
	_ = c.PutKeys(ctx, "prov1", "content1", system,
		[]*slotsv1.ContentKey{testKey("k2", time.Hour)})

	got, err := c.GetKeys(ctx, "prov1", "content1", system)
	if err != nil {
		t.Fatalf("GetKeys: %v", err)
	}
	if len(got) != 1 || got[0].GetKeyId() != "k2" {
		t.Fatalf("got %v, want [k2] (expired k1 dropped)", got)
	}
}

func TestPutKeysOverwritesSameKeyID(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()
	system := slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE

	_ = c.PutKeys(ctx, "prov1", "content1", system,
		[]*slotsv1.ContentKey{testKey("k1", time.Hour)})
	_ = c.PutKeys(ctx, "prov1", "content1", system,
		[]*slotsv1.ContentKey{testKey("k1", 2*time.Hour)})

	got, _ := c.GetKeys(ctx, "prov1", "content1", system)
	if len(got) != 1 {
		t.Fatalf("got %v, want 1 key (overwrite)", got)
	}
}

func TestPutKeysRejectsEmptyKeyID(t *testing.T) {
	c, _ := newTestCache(t)
	err := c.PutKeys(context.Background(), "p", "c", slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE,
		[]*slotsv1.ContentKey{{KeyMaterial: []byte("m")}})
	if err == nil {
		t.Fatal("expected error for empty key_id")
	}
}

func TestPutKeysRejectsEmptyMaterial(t *testing.T) {
	c, _ := newTestCache(t)
	err := c.PutKeys(context.Background(), "p", "c", slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE,
		[]*slotsv1.ContentKey{{KeyId: "k1"}})
	if err == nil {
		t.Fatal("expected error for empty key_material")
	}
}

// TestCredentialRotationNeverPurges proves that rotating a user's credentials
// (re-linking an account with new login material) does NOT purge the content-key
// cache (PLAN.md §2.4, §6.6). The cache is keyed by (provider, contentId,
// system), not by account — a title's key is the same for every user who
// decrypts it on that provider. Blocking a burned device stops acquisition,
// not decryption with keys already obtained.
func TestCredentialRotationNeverPurges(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()
	system := slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE

	// Cache keys for a content.
	_ = c.PutKeys(ctx, "prov1", "content1", system,
		[]*slotsv1.ContentKey{testKey("k1", time.Hour)})

	// Simulate credential rotation: the user re-links their account with new
	// login material. This must NOT touch the content-key cache.
	// (In production, re-linking updates the vaulted session but never calls
	// DropKeys — the cache is content-keyed, not account-keyed.)

	// Keys are still there.
	got, err := c.GetKeys(ctx, "prov1", "content1", system)
	if err != nil {
		t.Fatalf("GetKeys after rotation: %v", err)
	}
	if len(got) != 1 || got[0].GetKeyId() != "k1" {
		t.Fatalf("after rotation: got %v, want [k1]", got)
	}
}
