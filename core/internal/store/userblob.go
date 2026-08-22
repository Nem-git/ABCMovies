package store

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"strings"
)

// UserBlobStore is a Store decorator that encrypts and decrypts values using
// the current user's DEK, fetched from the request context (PLAN.md §2.4,
// IMPLEMENTATION.md §1.3). It implements the per-user encrypted blob storage
// class: data is encrypted at rest with a per-user key, and users cannot read
// each other's blobs.
//
// Keys are stored with a "user:<id>:" prefix so that different users' data is
// isolated even when sharing the same underlying store. Key names are stored in
// cleartext; only values are encrypted.
type UserBlobStore struct {
	inner Store
}

// NewUserBlobStore returns a UserBlobStore wrapping the given store.
func NewUserBlobStore(inner Store) *UserBlobStore {
	return &UserBlobStore{inner: inner}
}

func (s *UserBlobStore) Get(ctx context.Context, key string) ([]byte, error) {
	dek := dekFromContext(ctx)
	if dek == nil {
		return nil, fmt.Errorf("userblob: no DEK in context")
	}
	raw, err := s.inner.Get(ctx, userPrefix(ctx)+key)
	if err != nil {
		return nil, err
	}
	return decrypt(raw, dek)
}

func (s *UserBlobStore) Put(ctx context.Context, key string, value []byte) error {
	dek := dekFromContext(ctx)
	if dek == nil {
		return fmt.Errorf("userblob: no DEK in context")
	}
	enc, err := encrypt(value, dek)
	if err != nil {
		return err
	}
	return s.inner.Put(ctx, userPrefix(ctx)+key, enc)
}

func (s *UserBlobStore) Delete(ctx context.Context, key string) error {
	dek := dekFromContext(ctx)
	if dek == nil {
		return fmt.Errorf("userblob: no DEK in context")
	}
	return s.inner.Delete(ctx, userPrefix(ctx)+key)
}

func (s *UserBlobStore) List(ctx context.Context, prefix string) ([]string, error) {
	up := userPrefix(ctx)
	keys, err := s.inner.List(ctx, up+prefix)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = strings.TrimPrefix(k, up)
	}
	return out, nil
}

func (s *UserBlobStore) Close() error {
	return s.inner.Close()
}

// userPrefix returns the key prefix for the current user.
func userPrefix(ctx context.Context) string {
	uid, _ := ctx.Value(UserBlobUserIDKey).(string)
	if uid == "" {
		return ""
	}
	return "user:" + uid + ":"
}

// dekFromContext extracts the DEK from the context.
func dekFromContext(ctx context.Context) []byte {
	dek, _ := ctx.Value(UserBlobDEKKey).([]byte)
	return dek
}

// encrypt encrypts plaintext using AES-GCM with a random nonce.
func encrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("userblob: aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("userblob: gcm: %w", err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("userblob: generate nonce: %w", err)
	}
	return aead.Seal(nonce, nonce, plaintext, nil), nil
}

// decrypt decrypts ciphertext using AES-GCM with the nonce prepended.
func decrypt(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("userblob: aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("userblob: gcm: %w", err)
	}
	nonceSize := aead.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("userblob: ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return aead.Open(nil, nonce, ciphertext, nil)
}

// Context keys for the user blob store. Exported so the auth interceptor can
// set them in the request context.
type UserBlobCtxKey string

const (
	UserBlobUserIDKey UserBlobCtxKey = "userblob_user_id"
	UserBlobDEKKey    UserBlobCtxKey = "userblob_dek"
)

// ContextWithUserBlob returns a context carrying the user ID and DEK for
// per-user blob encryption. Call this in tests or internal code that needs
// to interact with a UserBlobStore outside the normal gRPC interceptor path.
func ContextWithUserBlob(ctx context.Context, userID string, dek []byte) context.Context {
	ctx = context.WithValue(ctx, UserBlobUserIDKey, userID)
	ctx = context.WithValue(ctx, UserBlobDEKKey, dek)
	return ctx
}
