package auth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nem-git/abcmovies/core/internal/store"
)

// TokenStore persists session token data.
type TokenStore interface {
	Save(ctx context.Context, key string, value []byte) error
	Load(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

// DEKCache holds per-user DEKs for the session lifetime.
type DEKCache interface {
	StoreDEK(userID string, dek []byte)
	GetDEK(userID string) []byte
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

// StoreDEKCache adapts a store.Store to the DEKCache interface.
// Keys are prefixed with "dek:" to avoid collisions.
type StoreDEKCache struct {
	store store.Store
}

// NewStoreDEKCache returns a DEKCache backed by the given store.Store.
func NewStoreDEKCache(s store.Store) *StoreDEKCache {
	return &StoreDEKCache{store: s}
}

func (a *StoreDEKCache) StoreDEK(userID string, dek []byte) {
	_ = a.store.Put(context.TODO(), "dek:"+userID, dek)
}

func (a *StoreDEKCache) GetDEK(userID string) []byte {
	val, err := a.store.Get(context.TODO(), "dek:"+userID)
	if err != nil {
		return nil
	}
	return val
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
