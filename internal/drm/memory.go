package drm

import (
	"context"
	"sync"
	"time"
)

// memoryEntry is one cached key with its expiry.
type memoryEntry struct {
	value     []byte
	expiresAt time.Time
}

// MemoryVaultStore is an in-memory VaultStore with a background cleanup loop.
// Keys are scoped by the vault (provider + content + KID), so a single store
// can back multiple providers.
type MemoryVaultStore struct {
	mu      sync.RWMutex
	entries map[string]memoryEntry
}

// NewMemoryVaultStore returns an empty in-memory key store.
func NewMemoryVaultStore() *MemoryVaultStore {
	return &MemoryVaultStore{entries: make(map[string]memoryEntry)}
}

// Get returns the cached value for key if present and not expired.
func (s *MemoryVaultStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[key]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false, nil
	}
	return e.value, true, nil
}

// Put stores value for key with an expiry.
func (s *MemoryVaultStore) Put(_ context.Context, key string, value []byte, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = memoryEntry{value: value, expiresAt: expiresAt}
	return nil
}

// Cleanup removes expired entries. Called periodically via CleanupLoop.
func (s *MemoryVaultStore) Cleanup() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, e := range s.entries {
		if now.After(e.expiresAt) {
			delete(s.entries, k)
		}
	}
}
