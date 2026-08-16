package drm

import (
	"context"
	"sync"
	"time"
)

// VaultStore is the persistence interface the vault caches keys in.
type VaultStore interface {
	// Get returns the cached value for key.
	Get(ctx context.Context, key string) ([]byte, bool, error)
	// Put stores value for key with an expiry.
	Put(ctx context.Context, key string, value []byte, expiresAt time.Time) error
}

// Vault wraps a KeyProvider with a TTL cache and singleflight so repeat key
// requests never hit the license server. It implements KeyProvider.
type Vault struct {
	store  VaultStore
	inner  KeyProvider
	ttl    time.Duration
	mu     sync.Mutex
	flight map[string]*vaultCall
}

type vaultCall struct {
	done chan struct{}
	keys map[KID]CEK
	err  error
}

// NewVault returns a Vault caching keys from inner in store.
func NewVault(store VaultStore, inner KeyProvider, ttl time.Duration) *Vault {
	if ttl <= 0 {
		ttl = licenseTTL
	}
	return &Vault{
		store:  store,
		inner:  inner,
		ttl:    ttl,
		flight: make(map[string]*vaultCall),
	}
}

// Scheme reports the underlying provider's scheme.
func (v *Vault) Scheme() Scheme {
	return v.inner.Scheme()
}

// GetKeys returns the requested keys, serving cache hits and coalescing
// concurrent misses for the same request.
func (v *Vault) GetKeys(ctx context.Context, req KeyRequest) (map[KID]CEK, error) {
	out := make(map[KID]CEK)

	// 1. Cache lookup for every requested KID.
	var missing []KID
	for _, kid := range req.KIDs {
		key, found, err := v.store.Get(ctx, v.key(req, kid))
		if err != nil {
			return nil, err
		}
		if found {
			var cek CEK
			cek = append(cek, key...)
			out[kid] = cek
		} else {
			missing = append(missing, kid)
		}
	}
	if len(missing) == 0 {
		return out, nil
	}

	// 2. Singleflight: coalesce concurrent misses for the same scope.
	fk := v.flightKey(req)
	req.KIDs = missing

	v.mu.Lock()
	call, ok := v.flight[fk]
	if !ok {
		call = &vaultCall{done: make(chan struct{})}
		v.flight[fk] = call
	}
	v.mu.Unlock()

	if !ok {
		// Leader: license the missing keys and write them back.
		call.keys, call.err = v.inner.GetKeys(ctx, req)
		v.mu.Lock()
		delete(v.flight, fk)
		v.mu.Unlock()
		close(call.done)
		if call.err != nil {
			return nil, call.err
		}
		now := time.Now()
		for kid, cek := range call.keys {
			out[kid] = cek
			_ = v.store.Put(ctx, v.key(req, kid), []byte(cek), now.Add(v.ttl))
		}
		return out, nil
	}

	// Follower: wait for the leader's result.
	select {
	case <-call.done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if call.err != nil {
		return nil, call.err
	}
	for kid, cek := range call.keys {
		out[kid] = cek
	}
	return out, nil
}

// key scopes a cache entry to provider + content + KID.
func (v *Vault) key(req KeyRequest, kid KID) string {
	return string(req.Scheme) + ":" + req.ProviderTag + ":" + req.ContentKey + ":" + kid.String()
}

// flightKey scopes in-flight license calls to provider + content (KIDs vary
// per request but all renditions of a title share the same scope).
func (v *Vault) flightKey(req KeyRequest) string {
	return string(req.Scheme) + ":" + req.ProviderTag + ":" + req.ContentKey
}
