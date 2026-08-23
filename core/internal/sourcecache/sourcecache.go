// Package sourcecache maintains the account source cache (PLAN.md §5.4): the
// core-owned index of what each linked streaming-provider account carries.
// The cache is rebuilt by paging a provider slot's whole-catalogue sync
// surface; every page is validated against the contract before anything is
// written, and an invalid page aborts the sync without updating the account's
// completion marker (reject, never downgrade — PLAN.md §2.5).
package sourcecache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/schema"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// maxPages bounds one sync run so a misbehaving provider cannot spin the
// synchronizer forever; a healthy catalogue of any realistic size fits far
// below it.
const maxPages = 10000

// Client is the slice of a provider slot the synchronizer needs. It is
// satisfied both by an admitted in-process slot and by a transport client.
type Client interface {
	CatalogueSync(ctx context.Context, req *slotsv1.CatalogueSyncRequest) (*slotsv1.CatalogueSyncResponse, error)
}

// Stats reports what one account sync did.
type Stats struct {
	Items int
	Pages int
}

// manifest is the per-account completion marker. Consumers read it to know
// whether the cached index is complete and how fresh it is.
type manifest struct {
	LastCompleteSync time.Time `json:"last_complete_sync"`
	Items            int       `json:"items"`
}

// Synchronizer rebuilds one provider's accounts into the cache.
type Synchronizer struct {
	provider string // capability namespace, e.g. "jellyfin"
	client   Client
	cache    store.Store
	logger   *slog.Logger
}

// New builds a synchronizer for one provider slot.
func New(provider string, client Client, cache store.Store, logger *slog.Logger) (*Synchronizer, error) {
	if provider == "" || client == nil || cache == nil {
		return nil, fmt.Errorf("sourcecache: provider, client, and cache are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Synchronizer{provider: provider, client: client, cache: cache, logger: logger}, nil
}

// itemKey namespaces a native id under its provider and account, mirroring
// the coverage-row addressing of LibraryEntry (provider:id).
func (s *Synchronizer) itemKey(accountID, nativeID string) string {
	return s.provider + "/" + accountID + "/" + nativeID
}

const manifestSuffix = "/_sync-manifest"

func (s *Synchronizer) manifestKey(accountID string) string {
	return s.provider + "/" + accountID + manifestSuffix
}

// Manifest returns the stored completion marker for an account, or false when
// no complete sync has landed yet.
func (s *Synchronizer) Manifest(ctx context.Context, accountID string) (manifest, bool, error) {
	blob, err := s.cache.Get(ctx, s.manifestKey(accountID))
	if err != nil {
		if err == store.ErrKeyNotFound {
			return manifest{}, false, nil
		}
		return manifest{}, false, fmt.Errorf("sourcecache: manifest: %w", err)
	}
	var m manifest
	if err := json.Unmarshal(blob, &m); err != nil {
		return manifest{}, false, fmt.Errorf("sourcecache: manifest corrupt: %w", err)
	}
	return m, true, nil
}

// SyncAccount pages the provider until exhaustion and upserts every item.
// Items removed upstream are not deleted here in M1 — deletion reconciliation
// arrives with identity work (IMPLEMENTATION.md §3); the manifest records the
// completion boundary regardless.
func (s *Synchronizer) SyncAccount(ctx context.Context, accountID string) (Stats, error) {
	stats := Stats{}
	token := ""
	for {
		if stats.Pages >= maxPages {
			return stats, fmt.Errorf("sourcecache: %s/%s: exceeded %d pages", s.provider, accountID, maxPages)
		}
		resp, err := s.client.CatalogueSync(ctx, &slotsv1.CatalogueSyncRequest{
			AccountId: accountID,
			PageToken: token,
		})
		if err != nil {
			return stats, fmt.Errorf("sourcecache: page %d: %w", stats.Pages+1, err)
		}
		// Reject, never downgrade: an invalid page aborts the run before any
		// of its items land, and the manifest stays at the last good sync.
		if err := schema.ValidateCatalogueSyncResponse(resp); err != nil {
			return stats, fmt.Errorf("sourcecache: page %d: contract violation: %w", stats.Pages+1, err)
		}
		for _, item := range resp.GetItems() {
			blob, err := protojson.Marshal(item)
			if err != nil {
				return stats, fmt.Errorf("sourcecache: encode %q: %w", item.GetNativeId(), err)
			}
			if err := s.cache.Put(ctx, s.itemKey(accountID, item.GetNativeId()), blob); err != nil {
				return stats, fmt.Errorf("sourcecache: write %q: %w", item.GetNativeId(), err)
			}
			stats.Items++
		}
		stats.Pages++
		token = resp.GetNextPageToken()
		if token == "" {
			break
		}
	}

	m := manifest{LastCompleteSync: time.Now().UTC(), Items: stats.Items}
	blob, err := json.Marshal(m)
	if err != nil {
		return stats, err
	}
	if err := s.cache.Put(ctx, s.manifestKey(accountID), blob); err != nil {
		return stats, fmt.Errorf("sourcecache: write manifest: %w", err)
	}
	s.logger.Info("source cache synced",
		"provider", s.provider, "account", accountID, "items", stats.Items, "pages", stats.Pages)
	return stats, nil
}

// ListItems returns every cached CatalogueItem for an account.
func (s *Synchronizer) ListItems(ctx context.Context, accountID string) ([]*slotsv1.CatalogueItem, error) {
	keys, err := s.cache.List(ctx, s.provider+"/"+accountID+"/")
	if err != nil {
		return nil, fmt.Errorf("sourcecache: list: %w", err)
	}
	out := make([]*slotsv1.CatalogueItem, 0, len(keys))
	for _, key := range keys {
		if strings.HasSuffix(key, manifestSuffix) {
			continue // bookkeeping, not catalogue content
		}
		blob, err := s.cache.Get(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("sourcecache: read %q: %w", key, err)
		}
		item := &slotsv1.CatalogueItem{}
		if err := protojson.Unmarshal(blob, item); err != nil {
			return nil, fmt.Errorf("sourcecache: decode %q: %w", key, err)
		}
		out = append(out, item)
	}
	return out, nil
}
