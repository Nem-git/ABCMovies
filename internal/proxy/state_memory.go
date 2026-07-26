package proxy

import (
	"context"
	"sync"
	"time"
)

// MemoryStore is an in-memory implementation of StateStore.
type MemoryStore struct {
	mu      sync.RWMutex
	entries map[string]*StreamMeta
	ttl     time.Duration
}

func NewMemoryStore(ttl time.Duration) *MemoryStore {
	s := &MemoryStore{
		entries: make(map[string]*StreamMeta),
		ttl:     ttl,
	}
	go s.cleanupLoop()
	return s
}

func (s *MemoryStore) Put(_ context.Context, key string, meta StreamMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = &meta
	return nil
}

func (s *MemoryStore) Get(_ context.Context, key string) (StreamMeta, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	meta, ok := s.entries[key]
	if !ok {
		return StreamMeta{}, false, nil
	}
	if time.Now().After(meta.ExpiresAt) {
		return StreamMeta{}, false, nil
	}
	return *meta, true, nil
}

func (s *MemoryStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, key)
	return nil
}

func (s *MemoryStore) cleanupLoop() {
	ticker := time.NewTicker(s.ttl)
	defer ticker.Stop()
	for range ticker.C {
		s.Cleanup(context.Background())
	}
}

func (s *MemoryStore) Cleanup(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, v := range s.entries {
		if now.After(v.ExpiresAt) {
			delete(s.entries, k)
		}
	}
	return nil
}
