// The enrichment engine (TECHNICAL-DECISIONS §1.28): resolve one queued
// entry to a cached TitleMetadata record. Resolution order per entry —
// external IDs first through the metadata cache, then the catalogue text-
// lookup fallback scored by the identity gate; ties abstain. Never runs in
// a request path; the Worker drives it.
package enrichment

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/identity"
)

// EntryEvidence is what the core knows about an entry before enrichment.
type EntryEvidence struct {
	Kind     slotsv1.ItemKind
	Metadata *corev1.TitleMetadata
	// ExternalIDs are the entry's asserted identity claims — provider-
	// supplied IDs and any heuristic IDs already adopted (PLAN.md §5.3).
	ExternalIDs []*slotsv1.ExternalId
}

// EntrySource supplies evidence for queued entries; the library/registry
// wiring implements it (M3 slice 6).
type EntrySource interface {
	Evidence(ctx context.Context, entryID string) (EntryEvidence, bool, error)
}

// CatalogueClient is one catalogue slot's contract surface.
type CatalogueClient interface {
	LookupTitle(context.Context, *slotsv1.LookupTitleRequest) (*slotsv1.LookupTitleResponse, error)
	GetMetadata(context.Context, *slotsv1.GetMetadataRequest) (*slotsv1.GetMetadataResponse, error)
}

// MetadataStore is the slice of the metadata cache the engine needs.
type MetadataStore interface {
	Resolve(ctx context.Context, externalID string) (string, bool, error)
	GetRecord(ctx context.Context, ref string) (*corev1.TitleMetadata, bool, error)
	PutRecord(ctx context.Context, ref string, m *corev1.TitleMetadata) error
	LinkAlias(ctx context.Context, alias, ref string) error
}

// Catalogue pairs a slot instance id (TECHNICAL-DECISIONS §1.25) with its
// client. Order matters: earlier slots win Adopt ties by being tried first,
// though the identity gate, not order, does the deciding.
type Catalogue struct {
	Slot   string
	Client CatalogueClient
}

// Engine enriches entries into the global metadata cache.
type Engine struct {
	source   EntrySource
	store    MetadataStore
	catalogs []Catalogue
	logger   *slog.Logger
	now      func() time.Time
}

// NewEngine builds an engine. Catalogues may be empty — the instance then
// enriches nothing until operators enable slots.
func NewEngine(src EntrySource, st MetadataStore, catalogs []Catalogue, logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	return &Engine{source: src, store: st, catalogs: catalogs, logger: logger, now: time.Now}
}

// Enrich resolves one entry to a cached record. A missing entry or an
// abstained verdict is not an error — both mean "nothing to do this tick".
func (e *Engine) Enrich(ctx context.Context, entryID string) error {
	ev, found, err := e.source.Evidence(ctx, entryID)
	if err != nil {
		return fmt.Errorf("enrichment: evidence for %q: %w", entryID, err)
	}
	if !found {
		e.logger.Info("enrichment skipped: entry vanished", "entry", entryID)
		return nil
	}

	// 1. External IDs first: any asserted claim already resolved in the
	// cache ends the work — the record exists under its canonical ref.
	for _, id := range ev.ExternalIDs {
		ref, ok, rerr := e.store.Resolve(ctx, extKey(id))
		if rerr != nil {
			return fmt.Errorf("enrichment: resolve %q: %w", extKey(id), rerr)
		}
		if !ok {
			continue
		}
		if _, ok, _ := e.store.GetRecord(ctx, ref); ok {
			return nil // already enriched under ref
		}
	}

	// 2. Text-lookup fallback across enabled catalogue slots.
	entryItem := identity.Item{Kind: ev.Kind, ExternalIDs: ev.ExternalIDs, Metadata: ev.Metadata}
	var pool []pooledCandidate
	for _, cat := range e.catalogs {
		resp, lerr := cat.Client.LookupTitle(ctx, &slotsv1.LookupTitleRequest{
			Query: ev.Metadata.GetTitle(),
			Kind:  ev.Kind,
			Year:  ev.Metadata.GetYear(),
		})
		if lerr != nil {
			e.logger.Warn("enrichment: catalogue lookup failed",
				"slot", cat.Slot, "entry", entryID, "error", lerr)
			continue // best-effort across slots
		}
		for _, c := range resp.GetCandidates() {
			pool = append(pool, pooledCandidate{cat: cat, candidate: c, item: candidateItem(c)})
		}
	}

	// 3. Hard gates over summaries; details decide the rest. Summaries
	// carry no corroborating signals, so survivors get their full records
	// fetched and Adopt adjudicates on those (TECHNICAL-DECISIONS §1.28).
	survivors := identity.Screen(entryItem, candidateItems(pool))
	if len(survivors) == 0 {
		e.logger.Info("enrichment abstained: no catalogue candidate passed screening",
			"entry", entryID, "candidates", len(pool))
		return nil
	}
	var full []fullItem
	for _, idx := range survivors {
		c := pool[idx]
		mdResp, gerr := c.cat.Client.GetMetadata(ctx, &slotsv1.GetMetadataRequest{Ref: c.candidate.GetRef()})
		if gerr != nil {
			e.logger.Warn("enrichment: detail fetch failed",
				"slot", c.cat.Slot, "ref", c.candidate.GetRef(), "error", gerr)
			continue // best-effort across survivors
		}
		full = append(full, fullItem{
			cat:      c.cat,
			ref:      c.candidate.GetRef(),
			response: mdResp,
			item: identity.Item{
				Kind:        c.candidate.GetKind(),
				ExternalIDs: append(claimedIDs(c.candidate), mdResp.GetExternalIds()...),
				Metadata:    mdResp.GetMetadata(),
			},
		})
	}
	if len(full) == 0 {
		// Screening found candidates but every decisive fetch failed —
		// report it so the drain tick counts as failed rather than
		// pretending nothing was there.
		return fmt.Errorf("enrichment: every detail fetch failed for entry %q (%d survivors)",
			entryID, len(survivors))
	}
	picked, ok := identity.Adopt(entryItem, fullItems(full))
	if !ok {
		e.logger.Info("enrichment abstained: no unambiguous catalogue match",
			"entry", entryID, "survivors", len(full))
		return nil
	}
	win := full[picked]

	// 4. Fold the winning record into the cache under the canonical ref.
	cur, _, _ := e.store.GetRecord(ctx, win.ref)
	merged, merr := Merge(cur, Contribution{
		Slot:     win.cat.Slot,
		Kind:     SourceCatalogue,
		Metadata: win.response.GetMetadata(),
	}, e.now())
	if merr != nil {
		return fmt.Errorf("enrichment: merge %q: %w", win.ref, merr)
	}
	if perr := e.store.PutRecord(ctx, win.ref, merged); perr != nil {
		return fmt.Errorf("enrichment: store record %q: %w", win.ref, perr)
	}
	// Every asserted ID becomes an alias to the canonical record.
	aliases := map[string]struct{}{win.ref: {}}
	for _, id := range win.response.GetExternalIds() {
		if key := extKey(id); key != "" {
			aliases[key] = struct{}{}
		}
	}
	for alias := range aliases {
		if alias == win.ref {
			continue // canonical resolves without an alias row
		}
		if lerr := e.store.LinkAlias(ctx, alias, win.ref); lerr != nil {
			e.logger.Warn("enrichment: alias link failed",
				"alias", alias, "ref", win.ref, "error", lerr)
		}
	}
	return nil
}

// candidateItem adapts a lookup summary to identity evidence. When the
// display title differs but the original title matches the entry, the
// original title stands in — that is exactly the localization case the
// field exists for (contract comment on TitleCandidate.original_title;
// matching semantics in TECHNICAL-DECISIONS.md §1.29).
func candidateItem(c *slotsv1.TitleCandidate) identity.Item {
	title := c.GetTitle()
	alt := c.GetOriginalTitle()
	item := identity.Item{
		Kind:        c.GetKind(),
		ExternalIDs: claimedIDs(c),
		Metadata:    &corev1.TitleMetadata{Title: title, Year: c.GetYear()},
	}
	if alt != "" && alt != title {
		item.AltTitles = []string{alt}
	}
	return item
}

// claimedIDs renders a candidate's own assertions, including its canonical
// ref parsed back into namespace:value form.
func claimedIDs(c *slotsv1.TitleCandidate) []*slotsv1.ExternalId {
	ids := append([]*slotsv1.ExternalId{}, c.GetExternalIds()...)
	if ns, val, ok := strings.Cut(c.GetRef(), ":"); ok && ns != "" && val != "" {
		ids = append(ids, &slotsv1.ExternalId{Namespace: ns, Value: val})
	}
	return ids
}

func extKey(id *slotsv1.ExternalId) string {
	if id == nil || id.GetNamespace() == "" || id.GetValue() == "" {
		return ""
	}
	return id.GetNamespace() + ":" + id.GetValue()
}

// pooledCandidate is one lookup hit plus its adapter-side evidence.
type pooledCandidate struct {
	cat       Catalogue
	candidate *slotsv1.TitleCandidate
	item      identity.Item
}

// candidateItems projects the pooled candidates to identity evidence.
func candidateItems(pool []pooledCandidate) []identity.Item {
	out := make([]identity.Item, len(pool))
	for i, p := range pool {
		out[i] = p.item
	}
	return out
}

// fullItem is a detail-fetched survivor.
type fullItem struct {
	cat      Catalogue
	ref      string
	response *slotsv1.GetMetadataResponse
	item     identity.Item
}

func fullItems(fs []fullItem) []identity.Item {
	out := make([]identity.Item, len(fs))
	for i, f := range fs {
		out[i] = f.item
	}
	return out
}
