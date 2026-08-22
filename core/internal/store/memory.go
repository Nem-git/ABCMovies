package store

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// InMemory is a Store backed by a Go map. Data lives in RAM and is lost on
// restart. Suitable for rebuildable caches (PLAN.md §2.4): account source
// cache, metadata cache, derived library cache, and job state in dev mode.
type InMemory struct {
	mu     sync.RWMutex
	data   map[string][]byte
	closed bool
}

// NewInMemory returns a ready-to-use in-memory store.
func NewInMemory() *InMemory {
	return &InMemory{data: make(map[string][]byte)}
}

func (m *InMemory) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, fmt.Errorf("store: closed")
	}
	v, ok := m.data[key]
	if !ok {
		return nil, ErrKeyNotFound
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out, nil
}

func (m *InMemory) Put(_ context.Context, key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return fmt.Errorf("store: closed")
	}
	cp := make([]byte, len(value))
	copy(cp, value)
	m.data[key] = cp
	return nil
}

func (m *InMemory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return fmt.Errorf("store: closed")
	}
	delete(m.data, key)
	return nil
}

func (m *InMemory) List(_ context.Context, prefix string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, fmt.Errorf("store: closed")
	}
	var keys []string
	for k := range m.data {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (m *InMemory) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return fmt.Errorf("store: already closed")
	}
	m.closed = true
	m.data = nil
	return nil
}
