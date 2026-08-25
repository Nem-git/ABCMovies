// Package sourcecache maintains the account source cache (PLAN.md §5.4): the
// core-owned index of what each linked streaming-provider account carries.
// The cache is rebuilt by paging a provider slot's whole-catalogue sync
// surface; every page is validated against the contract before anything is
// written, and an invalid page aborts the sync without updating the account's
// completion marker (reject, never downgrade — PLAN.md §2.5).
package sourcecache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
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
	// Removed counts items the provider no longer reports that deletion
	// reconciliation dropped from the cache.
	Removed int
}

// EntryLookup resolves a provider item to its library entry. It is satisfied
// by *itemregistry.Registry.
type EntryLookup interface {
	Lookup(ctx context.Context, provider, nativeID string) (string, bool, error)
}

// EventSink receives event envelopes for routing. It is satisfied by the
// in-memory bus.
type EventSink interface {
	Publish(*corev1.EventEnvelope)
}

// ItemResolver performs identity resolution for a synced item. It is
// satisfied by *itemregistry.Registry (through a narrow adapter). When wired,
// every synced item is resolved behind the run's success boundary, so
// mappings exist by the time deletion reconciliation reports arrivals and
// departures.
type ItemResolver interface {
	Resolve(ctx context.Context, provider string, item *slotsv1.CatalogueItem) error
}

// Option configures optional synchronizer collaborators.
type Option func(*Synchronizer)

// WithEntryLookup wires the item registry so removed items can be reported
// with their library-entry id.
func WithEntryLookup(l EntryLookup) Option {
	return func(s *Synchronizer) { s.entries = l }
}

// WithEventsSink wires an event sink so removals emit account-scoped
// availability-changed events.
func WithEventsSink(k EventSink) Option {
	return func(s *Synchronizer) { s.sink = k }
}

// WithItemResolver wires identity resolution into the sync path: each synced
// item is resolved after its page validates. A resolver failure aborts the
// run before the completion marker advances — a sync either lands whole or
// changes nothing identity-related.
func WithItemResolver(res ItemResolver) Option {
	return func(s *Synchronizer) { s.resolver = res }
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
	entries  EntryLookup  // optional
	sink     EventSink    // optional
	resolver ItemResolver // optional
}

// New builds a synchronizer for one provider slot.
func New(provider string, client Client, cache store.Store, logger *slog.Logger, opts ...Option) (*Synchronizer, error) {
	if provider == "" || client == nil || cache == nil {
		return nil, fmt.Errorf("sourcecache: provider, client, and cache are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	s := &Synchronizer{provider: provider, client: client, cache: cache, logger: logger}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

const manifestSuffix = "/_sync-manifest"

func (s *Synchronizer) manifestKey(accountID string) string {
	return s.provider + "/" + accountID + manifestSuffix
}

// Provider returns the capability namespace this synchronizer caches for.
func (s *Synchronizer) Provider() string { return s.provider }

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

// SyncAccount pages the provider until exhaustion, upserts every item, and —
// behind the run's success boundary — reconciles the account's cache against
// what the provider reported: departed items are dropped and both departures
// and arrivals surface as account-scoped availability-changed events when a
// registry and event sink are wired (PLAN.md §5.4). An aborted or invalid run
// reconciles nothing: deletions happen only after every page validated.
func (s *Synchronizer) SyncAccount(ctx context.Context, accountID string) (Stats, error) {
	stats := Stats{}
	prefix := s.provider + "/" + accountID + "/"
	prevKeys, err := s.cache.List(ctx, prefix)
	if err != nil {
		return stats, fmt.Errorf("sourcecache: snapshot: %w", err)
	}
	seen := make(map[string]struct{})
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
			if err := s.cache.Put(ctx, prefix+item.GetNativeId(), blob); err != nil {
				return stats, fmt.Errorf("sourcecache: write %q: %w", item.GetNativeId(), err)
			}
			if s.resolver != nil {
				if err := s.resolver.Resolve(ctx, s.provider, item); err != nil {
					return stats, fmt.Errorf("sourcecache: resolve %q: %w", item.GetNativeId(), err)
				}
			}
			seen[item.GetNativeId()] = struct{}{}
			stats.Items++
		}
		stats.Pages++
		token = resp.GetNextPageToken()
		if token == "" {
			break
		}
	}

	removed, err := s.reconcile(ctx, accountID, prevKeys, seen)
	if err != nil {
		return stats, err
	}
	stats.Removed = removed

	m := manifest{LastCompleteSync: time.Now().UTC(), Items: stats.Items}
	blob, err := json.Marshal(m)
	if err != nil {
		return stats, err
	}
	if err := s.cache.Put(ctx, s.manifestKey(accountID), blob); err != nil {
		return stats, fmt.Errorf("sourcecache: write manifest: %w", err)
	}
	s.logger.Info("source cache synced",
		"provider", s.provider, "account", accountID, "items", stats.Items,
		"pages", stats.Pages, "removed", stats.Removed)
	return stats, nil
}

// reconcile diffs the completed sync against the previous cache state and
// emits one availability-changed envelope per arrival and departure whose
// library entry is known (PLAN.md §5.4: the event reports a title leaving or
// joining). It runs only after every page validated, so a partial catalogue
// can never masquerade as a deletion wave.
func (s *Synchronizer) reconcile(ctx context.Context, accountID string, prevKeys []string, seen map[string]struct{}) (int, error) {
	prefix := s.provider + "/" + accountID + "/"
	prev := make(map[string]struct{}, len(prevKeys))
	for _, key := range prevKeys {
		if strings.HasSuffix(key, manifestSuffix) {
			continue // bookkeeping, not catalogue content
		}
		prev[strings.TrimPrefix(key, prefix)] = struct{}{}
	}

	removed := 0
	for nativeID := range prev {
		if _, ok := seen[nativeID]; ok {
			continue
		}
		if err := s.cache.Delete(ctx, prefix+nativeID); err != nil {
			return removed, fmt.Errorf("sourcecache: delete %q: %w", nativeID, err)
		}
		removed++
		s.notifyAvailability(ctx, accountID, nativeID, false)
	}
	for nativeID := range seen {
		if _, ok := prev[nativeID]; ok {
			continue
		}
		s.notifyAvailability(ctx, accountID, nativeID, true)
	}
	return removed, nil
}

// notifyAvailability publishes an ACCOUNT-scoped availability-changed
// envelope for an arriving or departing item. Without a wired registry or
// sink it is a no-op; an item with no registry mapping is logged and skipped
// rather than emitted with a blank entry id.
func (s *Synchronizer) notifyAvailability(ctx context.Context, accountID, nativeID string, present bool) {
	if s.sink == nil || s.entries == nil {
		return
	}
	entryID, ok, err := s.entries.Lookup(ctx, s.provider, nativeID)
	if err != nil || !ok {
		s.logger.Warn("source cache: availability change for unmapped item; event skipped",
			"provider", s.provider, "account", accountID, "native_id", nativeID,
			"present", present, "err", err)
		return
	}
	env := &corev1.EventEnvelope{
		Id:        newEventID(),
		Type:      corev1.EventType_EVENT_TYPE_AVAILABILITY_CHANGED,
		Audience:  corev1.EventAudience_EVENT_AUDIENCE_ACCOUNT,
		AccountId: accountID,
		Payload: &corev1.EventEnvelope_Availability{
			Availability: &corev1.AvailabilityEvent{
				AccountId: accountID,
				Provider:  s.provider,
				EntryId:   entryID,
				Present:   present,
			},
		},
		EmittedAt: timestamppb.Now(),
	}
	// The registry owns entry ids and this package owns keys; validate the
	// composed envelope before handing it to the bus.
	if err := schema.ValidateEventEnvelope(env); err != nil {
		s.logger.Error("source cache: refusing to publish invalid availability event", "err", err)
		return
	}
	s.sink.Publish(env)
}

func newEventID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
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
