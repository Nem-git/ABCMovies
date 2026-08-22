package auth

import (
	"bytes"
	"context"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/nem-git/abcmovies/core/internal/store"
)

// TokenStore persists session token data.
type TokenStore interface {
	Save(ctx context.Context, key string, value []byte) error
	Load(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

// StoreTokenStore adapts a store.Store to the TokenStore interface.
type StoreTokenStore struct {
	store store.Store
}

// NewStoreTokenStore returns a TokenStore backed by the given store.Store.
func NewStoreTokenStore(s store.Store) *StoreTokenStore {
	return &StoreTokenStore{store: s}
}

func (a *StoreTokenStore) Save(ctx context.Context, key string, value []byte) error {
	return a.store.Put(ctx, key, value)
}

func (a *StoreTokenStore) Load(ctx context.Context, key string) ([]byte, error) {
	return a.store.Get(ctx, key)
}

func (a *StoreTokenStore) Delete(ctx context.Context, key string) error {
	return a.store.Delete(ctx, key)
}

// DEKCache holds unwrapped data-encryption keys for the lifetime of a
// session. Keys are opaque session keys — derived from the session's bearer
// token by SessionHandler — never user IDs: each live session holds exactly
// its own entry, so revocation or expiry drops one session's material
// without touching any other (IMPLEMENTATION.md §1.3).
type DEKCache interface {
	// StoreDEK stores the DEK under the given session key.
	StoreDEK(key string, dek []byte) error

	// GetDEK returns the DEK stored under the given session key, or nil if
	// no entry exists.
	GetDEK(key string) []byte

	// DeleteDEK removes the entry under the given session key. Deleting a
	// missing key is a no-op.
	DeleteDEK(key string) error
}

// MemoryDEKCache is the default DEKCache: an in-memory map. Unwrapped key
// material never reaches any store; it lives only for the process lifetime,
// and a restart simply requires users to log in again.
type MemoryDEKCache struct {
	mu   sync.RWMutex
	deks map[string][]byte
}

// NewMemoryDEKCache returns an empty in-memory DEK cache.
func NewMemoryDEKCache() *MemoryDEKCache {
	return &MemoryDEKCache{deks: make(map[string][]byte)}
}

func (c *MemoryDEKCache) StoreDEK(key string, dek []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Clone so callers cannot mutate the cached key material through their
	// own slice (in-memory-only discipline: one buffer per entry).
	c.deks[key] = bytes.Clone(dek)
	return nil
}

func (c *MemoryDEKCache) GetDEK(key string) []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if dek, ok := c.deks[key]; ok {
		return bytes.Clone(dek)
	}
	return nil
}

func (c *MemoryDEKCache) DeleteDEK(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.deks, key)
	return nil
}

// SealedDEKCache is the opt-in persistent DEK cache for instances that must
// survive restarts without re-login. Entries are sealed with the instance's
// vault cipher before they reach the backing store and are opened on read:
// even this path never writes plaintext key material to disk (PLAN.md §7.6,
// IMPLEMENTATION.md §1.3). With the default ephemeral vault key, sealed
// entries become unreadable across restarts — memory-equivalent semantics;
// with a pinned vault key they persist with the sessions store.
type SealedDEKCache struct {
	store store.Store
	aead  cipher.AEAD
}

// NewSealedDEKCache returns a DEKCache that seals every entry with aead
// before writing it to s under a "dek:"-prefixed key.
func NewSealedDEKCache(s store.Store, aead cipher.AEAD) (*SealedDEKCache, error) {
	if s == nil {
		return nil, fmt.Errorf("sealed dek cache: backing store is required")
	}
	if aead == nil {
		return nil, fmt.Errorf("sealed dek cache: cipher is required")
	}
	return &SealedDEKCache{store: s, aead: aead}, nil
}

func (c *SealedDEKCache) StoreDEK(key string, dek []byte) error {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("sealed dek cache: generate nonce: %w", err)
	}
	blob := c.aead.Seal(nonce, nonce, dek, nil)
	if err := c.store.Put(context.TODO(), "dek:"+key, blob); err != nil {
		return fmt.Errorf("sealed dek cache: store %q: %w", key, err)
	}
	return nil
}

func (c *SealedDEKCache) GetDEK(key string) []byte {
	blob, err := c.store.Get(context.TODO(), "dek:"+key)
	if err != nil || len(blob) < c.aead.NonceSize() {
		return nil
	}
	nonce, ciphertext := blob[:c.aead.NonceSize()], blob[c.aead.NonceSize():]
	dek, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// Unreadable entry (e.g. vault key rotated): treat as absent rather
		// than failing every request carrying that session.
		return nil
	}
	return dek
}

func (c *SealedDEKCache) DeleteDEK(key string) error {
	return c.store.Delete(context.TODO(), "dek:"+key)
}

// StoreUserStore adapts a store.Store to the UserStore interface.
// UserData is JSON-serialized for persistence. Key names are stored in
// cleartext; only the values (which contain auth material) are encrypted
// at rest when the underlying store is a vault.
type StoreUserStore struct {
	store store.Store
}

// NewStoreUserStore returns a UserStore backed by the given store.Store.
func NewStoreUserStore(s store.Store) *StoreUserStore {
	return &StoreUserStore{store: s}
}

func (a *StoreUserStore) GetUser(username string) (*UserData, error) {
	raw, err := a.store.Get(context.TODO(), "user:"+username)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}
	var data UserData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("user data corrupted: %w", err)
	}
	return &data, nil
}

func (a *StoreUserStore) PutUser(username string, data *UserData) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal user data: %w", err)
	}
	return a.store.Put(context.TODO(), "user:"+username, raw)
}
