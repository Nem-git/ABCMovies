// Package library derives the per-user merged library (PLAN.md §5.1): every
// browse surface is built from the accounts the user can actually reach,
// resolved onto canonical LibraryEntries through the provider item registry.
// The derived result is cached per user and invalidated by account-scoped
// availability events; a missed event costs only staleness until the next
// rebuild, never wrongness beyond what the source caches already carry
// (PLAN.md §5.1, §5.4).
//
// The registry carries proof only; coverage lives here. Coverage rows are
// display-only by construction (PLAN.md §5.3): a row records that a provider
// item is present and how much identity backs it — never the reverse.
package library

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/itemregistry"
	"github.com/nem-git/abcmovies/core/internal/metadatacache"
	"github.com/nem-git/abcmovies/core/internal/sourcecache"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// Reach names one reachable provider account feeding every user's library.
// Until sharing lands (member scoping is M6), each linked account is reachable
// by every host user, which is exactly PLAN.md §5.1's rule for this milestone.
type Reach struct {
	Sync      *sourcecache.Synchronizer
	AccountID string
}

// Service derives and caches per-user libraries over a Store.
type Service struct {
	reaches []Reach
	reg     *itemregistry.Registry
	cache   store.Store
	logger  *slog.Logger

	// Enrichment seams (nil until WithEnrichment): resolve asserted IDs to
	// cached records while deriving, and hand unresolvable entries to the
	// background drain (TECHNICAL-DECISIONS.md §1.28).
	meta *metadatacache.Cache
	mark func(entryID string)
}

// MetaResolver is the slice of the metadata cache the derivation needs.
type MetaResolver = *metadatacache.Cache

type Option func(*Service)

// WithEnrichment wires the enrichment trigger. During every rebuild each
// entry's asserted external IDs are resolved through the cache: a hit fills
// LibraryEntry.metadata_ref, an all-miss marks the entry for the background
// worker via mark. Both seams are optional; without them the derivation
// behaves exactly as before enrichment existed.
func WithEnrichment(resolver MetaResolver, mark func(entryID string)) Option {
	return func(s *Service) { s.meta = resolver; s.mark = mark }
}

// NewService builds the library service. reg resolves provider items to
// entries; reaches lists every linked account's synchronizer.
func NewService(reaches []Reach, reg *itemregistry.Registry, cache store.Store, logger *slog.Logger, opts ...Option) (*Service, error) {
	if reg == nil || cache == nil {
		return nil, fmt.Errorf("library: registry and cache are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	s := &Service{reaches: reaches, reg: reg, cache: cache, logger: logger}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

const userPrefix = "lib/u/"

// Reaches returns a copy of the reachable account list this service derives
// from. Exposed for the observability surface (which linked accounts feed the
// library).
func (s *Service) Reaches() []Reach {
	out := make([]Reach, len(s.reaches))
	copy(out, s.reaches)
	return out
}

func (s *Service) userKey(userID string) string {
	return userPrefix + url.PathEscape(userID)
}

// Library returns the user's derived library, rebuilding it when no cached
// version exists (lazy first derivation; PLAN.md §5.1).
func (s *Service) Library(ctx context.Context, userID string) ([]*corev1.LibraryEntry, error) {
	blob, err := s.cache.Get(ctx, s.userKey(userID))
	if err == nil {
		var cached struct {
			Entries []*corev1.LibraryEntry `json:"entries"`
		}
		if err := json.Unmarshal(blob, &cached); err == nil {
			return cached.Entries, nil
		}
		// A corrupt blob falls through to a rebuild rather than failing the
		// user's whole library view; the derivation is deterministic.
		s.logger.Warn("library: corrupt cached library; rebuilding", "user", userID)
	} else if err != store.ErrKeyNotFound {
		return nil, fmt.Errorf("library: read cache: %w", err)
	}
	if err := s.RebuildUser(ctx, userID); err != nil {
		return nil, err
	}
	blob, err = s.cache.Get(ctx, s.userKey(userID))
	if err != nil {
		return nil, fmt.Errorf("library: read rebuilt cache: %w", err)
	}
	var cached struct {
		Entries []*corev1.LibraryEntry `json:"entries"`
	}
	if err := json.Unmarshal(blob, &cached); err != nil {
		return nil, fmt.Errorf("library: decode rebuilt cache: %w", err)
	}
	return cached.Entries, nil
}

// RebuildUser recomputes the user's library from every reachable account's
// source cache plus the item registry, and stores the result.
func (s *Service) RebuildUser(ctx context.Context, userID string) error {
	type build struct {
		entry     *corev1.LibraryEntry
		claimKeys map[string]bool // namespace\x00value assertions backing this entry
		observers map[string][]string
	}
	order := make([]string, 0)
	entries := make(map[string]*build)

	for _, reach := range s.reaches {
		items, err := reach.Sync.ListItems(ctx, reach.AccountID)
		if err != nil {
			return fmt.Errorf("library: list %s/%s: %w", reach.Sync.Provider(), reach.AccountID, err)
		}
		for _, item := range items {
			provider := reach.Sync.Provider()
			nativeID := item.GetNativeId()
			entryID, ok, err := s.reg.Lookup(ctx, provider, nativeID)
			if err != nil {
				return fmt.Errorf("library: lookup %s:%s: %w", provider, nativeID, err)
			}
			if !ok {
				// No mapping yet (identity work has not run for this item);
				// it contributes nothing until a sync resolves it.
				continue
			}
			canon, ok, err := s.reg.Canonical(ctx, entryID)
			if err != nil {
				return fmt.Errorf("library: canonical %s: %w", entryID, err)
			}
			if !ok {
				continue
			}
			b, ok := entries[entryID]
			if !ok {
				b = &build{
					entry: &corev1.LibraryEntry{
						Id:       entryID,
						Kind:     entryKind(canon.Kind),
						Coverage: map[string]*corev1.CoverageRow{},
						// metadata_ref is filled below from the metadata
						// cache; it names the canonical record this entry's
						// display data comes from.
					},
					claimKeys: map[string]bool{},
					observers: map[string][]string{},
				}
				var metaRef string
				for _, c := range canon.Claims {
					// Every claim on an entry came from a provider-supplied
					// assertion, which is exactly the corroborated case the
					// contract defines (library_entry.proto IdentityVerdict).
					b.entry.ExternalIdentities = append(b.entry.ExternalIdentities, &corev1.ExternalIdentity{
						Namespace:  c.Namespace,
						Value:      c.Value,
						Verdict:    corev1.IdentityVerdict_IDENTITY_VERDICT_CORROBORATED,
						Provenance: strings.Join(c.Suppliers, ","),
					})
					b.claimKeys[c.Namespace+"\x00"+c.Value] = true
					if metaRef == "" && s.meta != nil {
						ref, ok, err := s.meta.Resolve(ctx, c.Namespace+":"+c.Value)
						if err != nil {
							// Enrichment is best-effort display data; a
							// cache hiccup must not fail the derivation.
							s.logger.Warn("library: metadata resolve failed",
								"entry", entryID, "claim", c.Namespace+":"+c.Value, "error", err)
						} else if ok {
							metaRef = ref
						}
					}
				}
				b.entry.MetadataRef = metaRef
				entries[entryID] = b
				order = append(order, entryID)
			}

			corroborated := false
			for _, id := range item.GetExternalIds() {
				if b.claimKeys[id.GetNamespace()+"\x00"+id.GetValue()] {
					corroborated = true
					break
				}
			}
			verdict := corev1.CoverageVerdict_COVERAGE_VERDICT_PRESENCE_ONLY
			if corroborated {
				verdict = corev1.CoverageVerdict_COVERAGE_VERDICT_CORROBORATED
			}
			// Coverage keys are scoped "provider:nativeId" (PLAN.md §5.3).
			// Several linked accounts of one slot can carry the same item;
			// every observation is retained (CoverageRow.via is repeated —
			// provenance retention is mandatory) and rows are finalized
			// after all reaches, with sorted via elements so rebuilds are
			// deterministic.
			coverKey := provider + ":" + nativeID
			via := reach.AccountID + ":" + provider + ":host"
			if !contains(b.observers[coverKey], via) {
				b.observers[coverKey] = append(b.observers[coverKey], via)
			}
			if _, ok := b.entry.Coverage[coverKey]; !ok {
				b.entry.Coverage[coverKey] = &corev1.CoverageRow{Present: true, Verdict: verdict, LastVerified: timestamppb.Now()}
			}
		}
	}

	for _, b := range entries {
		for key, row := range b.entry.Coverage {
			vias := append([]string(nil), b.observers[key]...)
			sort.Strings(vias)
			row.Via = vias
		}
	}

	sort.Strings(order)
	out := make([]*corev1.LibraryEntry, 0, len(order))
	for _, id := range order {
		out = append(out, entries[id].entry)
	}
	blob, err := json.Marshal(struct {
		Entries []*corev1.LibraryEntry `json:"entries"`
	}{Entries: out})
	if err != nil {
		return fmt.Errorf("library: encode: %w", err)
	}
	if err := s.cache.Put(ctx, s.userKey(userID), blob); err != nil {
		return fmt.Errorf("library: write cache: %w", err)
	}
	// T1 trigger (TECHNICAL-DECISIONS.md §1.28): every entry whose claims
	// resolved to no cached record is handed to the background drain. The
	// queue coalesces repeats, so re-marking on each rebuild is the
	// self-healing mechanism, not a leak.
	if s.mark != nil {
		for _, id := range order {
			if entries[id].entry.GetMetadataRef() == "" {
				s.mark(id)
			}
		}
	}
	s.logger.Info("library derived", "user", userID, "entries", len(out))
	return nil
}

// InvalidateAccount drops every cached user library after one account's
// availability changed. Until member scoping exists every user reaches every
// linked account, so the precise recipient set is "everyone"; the next read
// rebuilds lazily (PLAN.md §5.1: a missed event leaves a slightly stale cache
// until the next periodic rebuild — acceptable).
func (s *Service) InvalidateAccount(_ string, _ string) error {
	keys, err := s.cache.List(context.Background(), userPrefix)
	if err != nil {
		return fmt.Errorf("library: list cached users: %w", err)
	}
	for _, k := range keys {
		if err := s.cache.Delete(context.Background(), k); err != nil {
			return fmt.Errorf("library: invalidate %q: %w", k, err)
		}
	}
	return nil
}

// Publish implements sourcecache.EventSink: account-scoped availability
// events invalidate the affected users' derived caches.
func (s *Service) Publish(env *corev1.EventEnvelope) {
	if av := env.GetAvailability(); av != nil {
		if err := s.InvalidateAccount(av.GetProvider(), av.GetAccountId()); err != nil {
			s.logger.Warn("library: invalidation failed; caches stay until next rebuild", "error", err)
		}
	}
}

func entryKind(k slotsv1.ItemKind) corev1.EntryKind {
	if k == slotsv1.ItemKind_ITEM_KIND_SERIES {
		return corev1.EntryKind_ENTRY_KIND_SERIES
	}
	return corev1.EntryKind_ENTRY_KIND_MOVIE
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
