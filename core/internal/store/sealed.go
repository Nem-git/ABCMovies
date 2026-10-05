package store

import (
	"context"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"log/slog"
	"sync"
)

// SealedStore seals every value with a caller-provided AEAD cipher before it
// reaches the inner store. Use it for store classes whose contents are secret
// but live in an unencrypted backend (the users store holds password hashes
// and wrapped keys; the vault class already encrypts itself, so it does not
// need this wrapper).
//
// The cipher is built from the instance key, so a record written with one key
// never opens under another — whether the change is an operator typo or a
// deliberate rotation. When that happens the read fails closed, and the first
// failure per process logs a warning naming it, so a key mismatch cannot pass
// for an empty store or a wrong password.
type SealedStore struct {
	inner  Store
	aead   cipher.AEAD
	logger *slog.Logger

	warnOnce sync.Once
}

// NewSealed wraps inner so its bytes are AEAD-encrypted on Put. A nil logger
// disables the key-mismatch warning.
func NewSealed(inner Store, aead cipher.AEAD, logger *slog.Logger) (*SealedStore, error) {
	if inner == nil {
		return nil, fmt.Errorf("sealed store: backing store is required")
	}
	if aead == nil {
		return nil, fmt.Errorf("sealed store: cipher is required")
	}
	return &SealedStore{inner: inner, aead: aead, logger: logger}, nil
}

func (s *SealedStore) Get(ctx context.Context, key string) ([]byte, error) {
	raw, err := s.inner.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if len(raw) < s.aead.NonceSize() {
		s.noteOpenFailure(key)
		return nil, fmt.Errorf("sealed store: record %q is not sealed", key)
	}
	nonce, ciphertext := raw[:s.aead.NonceSize()], raw[s.aead.NonceSize():]
	plain, err := s.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		s.noteOpenFailure(key)
		return nil, fmt.Errorf("sealed store: open %q: %w", key, err)
	}
	return plain, nil
}

func (s *SealedStore) Put(ctx context.Context, key string, value []byte) error {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("sealed store: generate nonce: %w", err)
	}
	blob := s.aead.Seal(nonce, nonce, value, nil)
	return s.inner.Put(ctx, key, blob)
}

func (s *SealedStore) Delete(ctx context.Context, key string) error {
	return s.inner.Delete(ctx, key)
}

func (s *SealedStore) List(ctx context.Context, prefix string) ([]string, error) {
	return s.inner.List(ctx, prefix)
}

func (s *SealedStore) Close() error {
	return s.inner.Close()
}

// noteOpenFailure logs once per process: after a key rotation or a mistyped
// vault key, every must-not-lose record fails the same way, and the first
// warning is what points at the cause.
func (s *SealedStore) noteOpenFailure(key string) {
	if s.logger == nil {
		return
	}
	s.warnOnce.Do(func() {
		s.logger.Warn("sealed store: cannot open a record with the configured instance key — the record was written with a different key; re-check stores.vault-key", "key", key)
	})
}
