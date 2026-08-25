// Package metadatacache is the global per-title cache (PLAN.md §2.4, §5.2):
// one TitleMetadata record per title plus an external-ID-to-record lookup,
// shared across all users and accounts. Records live under their canonical
// external ID (`namespace:value`); any other known ID for the same title is
// an alias resolving to that record. The class is durable and rebuildable
// from catalogue slots and provider adapters — loss costs fetches, never
// correctness. Enrichment (M3) is this cache's writer; identity never reads
// coverage from here, only records via refs.
package metadatacache

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/store"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	recordPrefix = "meta/record/"
	aliasPrefix  = "meta/alias/"
)

// Cache fronts the metadata-cache store class.
type Cache struct {
	st     store.Store
	logger *slog.Logger
}

// New returns a cache over st.
func New(st store.Store, logger *slog.Logger) (*Cache, error) {
	if st == nil {
		return nil, fmt.Errorf("metadatacache: store is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Cache{st: st, logger: logger}, nil
}

// GetRecord returns the TitleMetadata stored under ref (a canonical external
// ID). A missing or undecodable record is a miss — the cache is rebuildable,
// so corruption downgrades to absence rather than failing reads.
func (c *Cache) GetRecord(ctx context.Context, ref string) (*corev1.TitleMetadata, bool, error) {
	if err := validateExternalID(ref); err != nil {
		return nil, false, err
	}
	blob, err := c.st.Get(ctx, recordPrefix+ref)
	if err == store.ErrKeyNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("metadatacache: read record: %w", err)
	}
	m := &corev1.TitleMetadata{}
	if err := protojson.Unmarshal(blob, m); err != nil {
		c.logger.Warn("metadatacache: corrupt record ignored; enrichment can rebuild it",
			"ref", ref, "error", err)
		return nil, false, nil
	}
	return m, true, nil
}

// PutRecord stores m under ref, overwriting any previous record.
func (c *Cache) PutRecord(ctx context.Context, ref string, m *corev1.TitleMetadata) error {
	if err := validateExternalID(ref); err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("metadatacache: record %q: nil metadata", ref)
	}
	blob, err := protojson.Marshal(m)
	if err != nil {
		return fmt.Errorf("metadatacache: encode record %q: %w", ref, err)
	}
	if err := c.st.Put(ctx, recordPrefix+ref, blob); err != nil {
		return fmt.Errorf("metadatacache: write record %q: %w", ref, err)
	}
	return nil
}

// Resolve maps any known external ID to its record ref. A canonical ID with
// a stored record resolves to itself without an explicit alias; aliases
// resolve through the lookup they were linked with.
func (c *Cache) Resolve(ctx context.Context, externalID string) (string, bool, error) {
	if err := validateExternalID(externalID); err != nil {
		return "", false, err
	}
	blob, err := c.st.Get(ctx, aliasPrefix+externalID)
	if err == nil {
		ref := string(blob)
		if err := validateExternalID(ref); err != nil {
			c.logger.Warn("metadatacache: corrupt alias ignored", "alias", externalID, "error", err)
			return "", false, nil
		}
		return ref, true, nil
	}
	if err != store.ErrKeyNotFound {
		return "", false, fmt.Errorf("metadatacache: read alias: %w", err)
	}
	// Canonical fallback: an ID with a stored record needs no alias row.
	if _, ok, err := c.GetRecord(ctx, externalID); err != nil || !ok {
		return "", false, err
	} else if ok {
		return externalID, true, nil
	}
	return "", false, nil
}

// LinkAlias points alias at the record stored under ref. The record must
// exist — dangling lookups would send readers chasing ghosts.
func (c *Cache) LinkAlias(ctx context.Context, alias, ref string) error {
	if err := validateExternalID(alias); err != nil {
		return err
	}
	if err := validateExternalID(ref); err != nil {
		return err
	}
	if _, ok, err := c.GetRecord(ctx, ref); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("metadatacache: alias %q: no record under %q", alias, ref)
	}
	if err := c.st.Put(ctx, aliasPrefix+alias, []byte(ref)); err != nil {
		return fmt.Errorf("metadatacache: link alias %q: %w", alias, err)
	}
	return nil
}

// DeleteRecord removes the record under ref and every alias pointing at it
// (PLAN.md §5.3: heuristic-resolved IDs are purgeable). Deleting an absent
// record is a no-op.
func (c *Cache) DeleteRecord(ctx context.Context, ref string) error {
	if err := validateExternalID(ref); err != nil {
		return err
	}
	aliases, err := c.st.List(ctx, aliasPrefix)
	if err != nil {
		return fmt.Errorf("metadatacache: list aliases: %w", err)
	}
	for _, key := range aliases {
		blob, err := c.st.Get(ctx, key)
		if err != nil {
			continue // raced away; nothing to purge
		}
		if string(blob) != ref {
			continue
		}
		if err := c.st.Delete(ctx, key); err != nil {
			return fmt.Errorf("metadatacache: purge alias %q: %w", strings.TrimPrefix(key, aliasPrefix), err)
		}
	}
	if err := c.st.Delete(ctx, recordPrefix+ref); err != nil {
		return fmt.Errorf("metadatacache: delete record %q: %w", ref, err)
	}
	return nil
}

// validateExternalID pins the `namespace:value` shape every key in this class
// derives from — the same discipline scoped keys follow everywhere else
// (TECHNICAL-DECISIONS.md §1.25).
func validateExternalID(id string) error {
	ns, val, ok := strings.Cut(id, ":")
	if !ok || ns == "" || val == "" {
		return fmt.Errorf("metadatacache: external id %q is not namespace:value", id)
	}
	return nil
}
