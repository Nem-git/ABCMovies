// Package contentkeycache maintains the content-key cache (PLAN.md §2.4,
// §6.6): a global, durable, safe-to-rebuild store of DRM content keys. The
// cache is keyed by content — (provider, contentId, keyId) — because a title's
// key is the same for every user who decrypts it on that provider. Entries
// carry only key material (encrypted at rest) and validity (TTL = license
// validity).
//
// The cache is a hint, not a guarantee (PLAN.md §6.6): on a decrypt failure
// the entry is dropped and acquire-keys is re-run (fail-fast re-license).
// Credential-set rotation never purges the cache — blocking a burned device
// stops acquisition, not decryption with keys already obtained.
package contentkeycache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// Cache is the content-key cache for one provider. A title's key is the same
// for every user who decrypts it on that provider, so the cache is global
// across all users and accounts (PLAN.md §2.4).
type Cache struct {
	provider string
	cache    store.Store
	logger   *slog.Logger
}

// New builds a content-key cache for one provider slot.
func New(provider string, cache store.Store, logger *slog.Logger) (*Cache, error) {
	if provider == "" || cache == nil {
		return nil, fmt.Errorf("contentkeycache: provider and cache are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Cache{provider: provider, cache: cache, logger: logger}, nil
}

// Provider returns the capability namespace this cache stores keys for.
func (c *Cache) Provider() string { return c.provider }

// key builds the store key for one content key: provider/contentId/keyId.
func (c *Cache) key(contentID, keyID string) string {
	return c.provider + "/" + contentID + "/" + keyID
}

// GetKeys retrieves all cached keys for a content id. Returns an empty slice
// (not an error) when nothing is cached — the caller decides whether to
// re-license. Expired entries are dropped on read (fail-fast: a stale key is
// worse than no key, because it produces a decrypt failure that could have
// been avoided by re-licensing).
func (c *Cache) GetKeys(ctx context.Context, contentID string) ([]*slotsv1.ContentKey, error) {
	prefix := c.provider + "/" + contentID + "/"
	keys, err := c.cache.List(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("contentkeycache: list: %w", err)
	}
	var out []*slotsv1.ContentKey
	for _, k := range keys {
		blob, err := c.cache.Get(ctx, k)
		if err != nil {
			return nil, fmt.Errorf("contentkeycache: read %q: %w", k, err)
		}
		var entry cacheEntry
		if err := json.Unmarshal(blob, &entry); err != nil {
			return nil, fmt.Errorf("contentkeycache: decode %q: %w", k, err)
		}
		if time.Now().UTC().After(entry.ExpiresAt) {
			// Fail-fast: drop expired entries on read so the caller re-licenses
			// instead of using a stale key that will fail at decrypt time.
			if err := c.cache.Delete(ctx, k); err != nil {
				c.logger.Warn("contentkeycache: failed to drop expired entry",
					"key", k, "err", err)
			}
			continue
		}
		out = append(out, entry.Key)
	}
	return out, nil
}

// PutKeys stores keys for a content id. Existing keys for the same content id
// are overwritten (same key id → new material + new expiry). The TTL is the
// license validity carried by each key.
func (c *Cache) PutKeys(ctx context.Context, contentID string, keys []*slotsv1.ContentKey) error {
	for _, k := range keys {
		if k.GetKeyId() == "" {
			return fmt.Errorf("contentkeycache: key_id is required")
		}
		if len(k.GetKeyMaterial()) == 0 {
			return fmt.Errorf("contentkeycache: key_material is required for key %q", k.GetKeyId())
		}
		entry := cacheEntry{
			Key:       k,
			ExpiresAt: time.Now().UTC().Add(k.GetValidity().AsDuration()),
		}
		blob, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("contentkeycache: encode %q: %w", k.GetKeyId(), err)
		}
		storeKey := c.key(contentID, k.GetKeyId())
		if err := c.cache.Put(ctx, storeKey, blob); err != nil {
			return fmt.Errorf("contentkeycache: write %q: %w", storeKey, err)
		}
	}
	return nil
}

// DropKeys removes all cached keys for a content id. Called on decrypt
// failure (fail-fast re-license — PLAN.md §6.6): the cached key is wrong or
// stale, so it is dropped and acquire-keys is re-run.
func (c *Cache) DropKeys(ctx context.Context, contentID string) error {
	prefix := c.provider + "/" + contentID + "/"
	keys, err := c.cache.List(ctx, prefix)
	if err != nil {
		return fmt.Errorf("contentkeycache: list for drop: %w", err)
	}
	for _, k := range keys {
		if err := c.cache.Delete(ctx, k); err != nil {
			return fmt.Errorf("contentkeycache: delete %q: %w", k, err)
		}
	}
	return nil
}

// DropKey removes one cached key by key id. Called on decrypt failure when
// only one key in a multi-key set is bad.
func (c *Cache) DropKey(ctx context.Context, contentID, keyID string) error {
	return c.cache.Delete(ctx, c.key(contentID, keyID))
}

// cacheEntry is the stored form of a content key: the key itself plus its
// expiry. The key material is encrypted at rest by the store's encryption
// layer (PLAN.md §2.4 — the content-key cache is a durable, safe-to-rebuild
// store class).
type cacheEntry struct {
	Key       *slotsv1.ContentKey `json:"key"`
	ExpiresAt time.Time           `json:"expires_at"`
}
