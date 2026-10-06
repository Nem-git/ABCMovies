// Package store provides persistence backends for the system's storage classes
// (PLAN.md §2.4). Every store class — caches, vault, watch history, jobs — is
// backed by a Store implementation chosen at startup.
//
// The Store interface is deliberately simple: a key-value container operating on
// raw bytes. Callers own protobuf serialization; the store owns persistence and,
// where required, encryption.
package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrKeyNotFound is returned by Get when the key does not exist.
var ErrKeyNotFound = errors.New("store: key not found")

// ensureParentDir creates the directory holding path when the path is nested,
// so a fresh checkout can point a file-backed store at a location that does
// not exist yet instead of failing with an opaque SQLite open error.
func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	return nil
}

// Store is a key-value persistence layer. Keys are opaque strings; values are
// raw bytes. Implementations differ in durability, encryption, and who may read
// (PLAN.md §2.4), but the interface is uniform.
type Store interface {
	// Get retrieves the value for key. Returns ErrKeyNotFound if the key does
	// not exist.
	Get(ctx context.Context, key string) ([]byte, error)

	// Put stores value under key, overwriting any existing value.
	Put(ctx context.Context, key string, value []byte) error

	// Delete removes the key. Deleting a non-existent key is a no-op.
	Delete(ctx context.Context, key string) error

	// List returns all keys that match the given prefix. An empty prefix
	// returns all keys. The returned keys are not sorted.
	List(ctx context.Context, prefix string) ([]string, error)

	// Close releases resources held by the store. After Close, all other
	// methods return errors.
	Close() error
}
