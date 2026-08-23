// Package itemregistry implements the provider item registry (PLAN.md §5.3):
// the durable `(provider, nativeId) -> { entryId, proof }` mapping that turns
// provider-native catalogue items into canonical LibraryEntries. The registry
// carries identity proof only, never coverage; an availability refresh is a
// pure lookup here, and identity work runs only on first-seen, proof change,
// or corroboration need. Merges are unions, never ID destruction: superseded
// mappings stay resolvable through the alias table, and a recycled provider
// ID becomes a new instance plus a merge-conflict event rather than a silent
// remap.
package itemregistry

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/identity"
	"github.com/nem-git/abcmovies/core/internal/schema"
	"github.com/nem-git/abcmovies/core/internal/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Status describes what a Resolve call did.
type Status int

const (
	// StatusCreated means the item was first-seen and nothing merged into it:
	// a new LibraryEntry was minted.
	StatusCreated Status = iota
	// StatusAttached means the item merged into an existing entry, either by
	// a matching provider-supplied external ID or by the conservative
	// heuristic rule.
	StatusAttached
	// StatusUnchanged means a mapping existed and its proof still matches:
	// a pure lookup, with no identity work (PLAN.md §5.3).
	StatusUnchanged
	// StatusUpdated means a mapping existed and its proof evolved (e.g. the
	// provider began asserting another external ID, or supplied richer
	// signal material) while identity stayed on the same entry. The stored
	// material accumulates the new assertions; nothing is remapped and
	// nothing is destroyed.
	StatusUpdated
)

func (s Status) String() string {
	switch s {
	case StatusCreated:
		return "created"
	case StatusAttached:
		return "attached"
	case StatusUnchanged:
		return "unchanged"
	case StatusUpdated:
		return "updated"
	default:
		return fmt.Sprintf("Status(%d)", int(s))
	}
}

// Outcome reports what a single Resolve did, including any events the caller
// should publish on the ephemeral event bus.
type Outcome struct {
	// EntryID is the LibraryEntry the item now belongs to.
	EntryID string
	Status  Status
	// Recycled is true when a previous mapping for this (provider, nativeId)
	// was superseded because its proof no longer matches and re-resolution
	// landed on a different entry (PLAN.md §5.3, recycled provider IDs). The
	// old entry keeps resolving through the alias table.
	Recycled bool
	// SupersededEntryID is the entry the superseded mapping pointed to.
	SupersededEntryID string
	// Events carries envelopes for the caller to publish. Empty unless a
	// mapping moved to a different entry and the registry was constructed
	// with an owner ID.
	Events []*corev1.EventEnvelope
}

// mappingRecord is one (provider, nativeId) slot's current identity state.
type mappingRecord struct {
	EntryID    string         `json:"entryId"`
	Generation uint64         `json:"generation"`
	Proof      identity.Proof `json:"proof"`
}

// entryRecord is the canonical identity material of one LibraryEntry: what
// matching compares against. Title, year and kind are fixed at creation;
// external-ID assertions and corroborating signals accumulate as items merge
// in — merging is a union onto the canonical entry (PLAN.md §5.3). Every
// claim keeps its provenance: which providers supplied it (PLAN.md §5.3,
// provenance retention is mandatory).
type entryRecord struct {
	ID               string           `json:"id"`
	Kind             slotsv1.ItemKind `json:"kind"`
	Title            string           `json:"title"`
	Year             uint32           `json:"year"`
	Directors        []string         `json:"directors,omitempty"`
	Cast             []string         `json:"cast,omitempty"`
	OriginalLanguage string           `json:"originalLanguage,omitempty"`
	RuntimeMinutes   uint32           `json:"runtimeMinutes,omitempty"`
	Claims           []idClaim        `json:"claims"`
}

// idClaim is one external-identity assertion on an entry plus its provenance:
// the set of providers that supplied it. A claim asserted by several
// providers is the same title confirmed by independent catalogues.
type idClaim struct {
	Namespace string   `json:"ns"`
	Value     string   `json:"value"`
	Suppliers []string `json:"suppliers,omitempty"`
}

func (c idClaim) key() string { return c.Namespace + "\x00" + c.Value }

// IdentityClaim is the public view of one accumulated identity assertion.
type IdentityClaim struct {
	// Namespace and value of the claim, e.g. imdb / tt0133093.
	Namespace string
	Value     string
	// Suppliers lists the providers that have asserted this claim.
	Suppliers []string
}

// Canonical is an entry's stable identity surface: what it is and which
// external identities back that. Coverage never appears here — the registry
// carries proof only, never coverage (IMPLEMENTATION.md M2).
type Canonical struct {
	Kind   slotsv1.ItemKind
	Title  string
	Year   uint32
	Claims []IdentityClaim
}

// Canonical returns the entry's canonical identity material. The second
// return is false for unknown ids.
func (r *Registry) Canonical(ctx context.Context, entryID string) (Canonical, bool, error) {
	if entryID == "" {
		return Canonical{}, false, fmt.Errorf("itemregistry: entry id is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, err := r.getEntry(ctx, entryID)
	if err != nil || rec == nil {
		return Canonical{}, false, err
	}
	out := Canonical{Kind: rec.Kind, Title: rec.Title, Year: rec.Year}
	for _, c := range rec.Claims {
		suppliers := append([]string(nil), c.Suppliers...)
		sort.Strings(suppliers)
		out.Claims = append(out.Claims, IdentityClaim{Namespace: c.Namespace, Value: c.Value, Suppliers: suppliers})
	}
	return out, true, nil
}

// item renders the entry's accumulated identity material for comparison.
func (e *entryRecord) item() identity.Item {
	md := &corev1.TitleMetadata{Title: e.Title, Year: e.Year, Directors: e.Directors, Cast: e.Cast, OriginalLanguage: e.OriginalLanguage}
	if e.RuntimeMinutes > 0 {
		md.KindSpecific = &corev1.TitleMetadata_Movie{Movie: &corev1.MovieSpecific{RuntimeMinutes: e.RuntimeMinutes}}
	}
	ids := make([]*slotsv1.ExternalId, 0, len(e.Claims))
	for _, c := range e.Claims {
		ids = append(ids, &slotsv1.ExternalId{Namespace: c.Namespace, Value: c.Value})
	}
	return identity.Item{Kind: e.Kind, ExternalIDs: ids, Metadata: md}
}

// Registry is the provider item registry over a Store (PLAN.md §2.4 identity
// store row: durable, host-read). It is safe for concurrent use; resolution
// is serialized so concurrent syncs cannot double-create entries.
type Registry struct {
	st      store.Store
	ownerID string
	mu      sync.Mutex
}

// New returns a registry backed by st. ownerID, when non-empty, is placed on
// OWNER-audience merge-conflict events so the operator sees recycled IDs;
// leave it empty to suppress emission.
func New(st store.Store, ownerID string) (*Registry, error) {
	if st == nil {
		return nil, fmt.Errorf("itemregistry: nil store")
	}
	return &Registry{st: st, ownerID: ownerID}, nil
}

// Resolve maps one provider-native catalogue item onto a LibraryEntry,
// creating or attaching per the merge rule (PLAN.md §5.3). The item must
// pass ValidateCatalogueItem. When a mapping exists and its proof still
// matches, this is a pure lookup: nothing else happens.
func (r *Registry) Resolve(ctx context.Context, provider string, item *slotsv1.CatalogueItem) (*Outcome, error) {
	if err := schema.ValidateCatalogueItem(item); err != nil {
		return nil, fmt.Errorf("itemregistry: %w", err)
	}
	if provider == "" {
		return nil, fmt.Errorf("itemregistry: provider is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	nativeID := item.GetNativeId()
	proof := identity.ProofOf(item.GetKind(), item.GetMetadata(), item.GetExternalIds())

	cur, err := r.loadMapping(ctx, provider, nativeID)
	if err != nil {
		return nil, err
	}
	if cur != nil && identity.SameProof(cur.Proof, proof) {
		return &Outcome{EntryID: cur.EntryID, Status: StatusUnchanged}, nil
	}

	out := &Outcome{}
	target, err := r.findTarget(ctx, item)
	if err != nil {
		return nil, err
	}
	switch target {
	case "":
		id, err := newEntryID()
		if err != nil {
			return nil, err
		}
		// The founding item contributes its full identity material:
		// proof fields fix the canonical title/year/kind, and its
		// corroborating signals are stored so later siblings can
		// heuristically merge against them.
		md := item.GetMetadata()
		if err := r.putEntry(ctx, &entryRecord{
			ID:               id,
			Kind:             proof.Kind,
			Title:            proof.Title,
			Year:             proof.Year,
			Directors:        unionStrings(nil, md.GetDirectors()),
			Cast:             unionStrings(nil, md.GetCast()),
			OriginalLanguage: md.GetOriginalLanguage(),
			RuntimeMinutes:   md.GetMovie().GetRuntimeMinutes(),
			Claims:           claimsFromIDs(proof.ExternalIDs, provider),
		}); err != nil {
			return nil, err
		}
		out.EntryID, out.Status = id, StatusCreated
	default:
		if err := r.absorb(ctx, target, item, provider); err != nil {
			return nil, err
		}
		out.EntryID, out.Status = target, StatusAttached
	}

	next := mappingRecord{EntryID: out.EntryID, Proof: proof}
	if cur == nil {
		next.Generation = 1
	} else {
		// The live item's identity material drifted from the stored proof.
		// Re-resolution landed on out.EntryID; the old mapping generation is
		// retained as an alias, and a move to a different entry surfaces as a
		// merge conflict instead of a silent remap (PLAN.md §5.3).
		if out.EntryID == cur.EntryID {
			out.Status = StatusUpdated
		} else {
			out.Recycled, out.SupersededEntryID = true, cur.EntryID
			if r.ownerID != "" {
				out.Events = append(out.Events, conflictEvent(provider, nativeID, cur.EntryID, r.ownerID))
			}
		}
		if err := r.st.Put(ctx, aliasKey(provider, nativeID, cur.Generation), []byte(cur.EntryID)); err != nil {
			return nil, err
		}
		next.Generation = cur.Generation + 1
	}
	if err := r.st.Put(ctx, mappingKey(provider, nativeID), marshal(next)); err != nil {
		return nil, err
	}
	return out, nil
}

// Lookup returns the entry a provider item currently maps to. This is the
// pure lookup availability refreshes rely on: it performs no identity work
// (PLAN.md §5.3).
func (r *Registry) Lookup(ctx context.Context, provider, nativeID string) (string, bool, error) {
	if provider == "" || nativeID == "" {
		return "", false, fmt.Errorf("itemregistry: provider and native id are required")
	}
	rec, err := r.loadMapping(ctx, provider, nativeID)
	if err != nil {
		return "", false, err
	}
	if rec == nil {
		return "", false, nil
	}
	return rec.EntryID, true, nil
}

// Alias returns the entry a historical generation of a provider-item mapping
// pointed to. Aliases are never destroyed (PLAN.md §2.3): history, playlists
// and downloads keep resolving.
func (r *Registry) Alias(ctx context.Context, provider, nativeID string, generation uint64) (string, bool, error) {
	b, err := r.st.Get(ctx, aliasKey(provider, nativeID, generation))
	if err == store.ErrKeyNotFound {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return string(b), true, nil
}

// findTarget searches indexed candidates — entries sharing a provider-supplied
// external ID first (identity assertions outrank heuristics), then entries
// carrying the same normalized title and kind — and returns the first entry
// the merge rule merges into.
func (r *Registry) findTarget(ctx context.Context, item *slotsv1.CatalogueItem) (string, error) {
	live := identity.Item{
		Kind:        item.GetKind(),
		ExternalIDs: item.GetExternalIds(),
		Metadata:    item.GetMetadata(),
	}
	seen := map[string]bool{}
	var order []string
	for _, id := range item.GetExternalIds() {
		if id == nil || id.GetNamespace() == "" || id.GetValue() == "" {
			continue
		}
		keys, err := r.st.List(ctx, extIndexPrefix(id.GetNamespace(), id.GetValue()))
		if err != nil {
			return "", err
		}
		order = appendUniqueIDs(order, seen, keys)
	}
	for _, id := range order {
		ok, err := r.mergesInto(ctx, id, live)
		if err != nil {
			return "", err
		}
		if ok {
			return id, nil
		}
	}

	order = order[:0]
	clear(seen)
	keys, err := r.st.List(ctx, titleIndexPrefix(identity.NormalizeTitle(item.GetMetadata().GetTitle()), item.GetKind()))
	if err != nil {
		return "", err
	}
	order = appendUniqueIDs(order, seen, keys)
	for _, id := range order {
		ok, err := r.mergesInto(ctx, id, live)
		if err != nil {
			return "", err
		}
		if ok {
			return id, nil
		}
	}
	return "", nil
}

// mergesInto reports whether the live item merges into the entry with the
// given id.
func (r *Registry) mergesInto(ctx context.Context, entryID string, live identity.Item) (bool, error) {
	rec, err := r.getEntry(ctx, entryID)
	if err != nil {
		return false, err
	}
	if rec == nil {
		return false, nil // dangling index row; treat as absent
	}
	return identity.Decide(live, rec.item()).Merge, nil
}

// absorb unions the item's provider-supplied external IDs and corroborating
// signals into the entry's accumulated identity material (and the external-ID
// index), so later items can corroborate against them. The supplying provider
// is recorded on every claim the item asserts.
func (r *Registry) absorb(ctx context.Context, entryID string, item *slotsv1.CatalogueItem, supplier string) error {
	rec, err := r.getEntry(ctx, entryID)
	if err != nil {
		return err
	}
	if rec == nil {
		return fmt.Errorf("itemregistry: entry %q vanished during absorb", entryID)
	}
	md := item.GetMetadata()
	rec.Claims = unionClaims(rec.Claims, item.GetExternalIds(), supplier)
	rec.Directors = unionStrings(rec.Directors, md.GetDirectors())
	rec.Cast = unionStrings(rec.Cast, md.GetCast())
	if rec.OriginalLanguage == "" {
		rec.OriginalLanguage = md.GetOriginalLanguage()
	}
	if rec.RuntimeMinutes == 0 {
		rec.RuntimeMinutes = md.GetMovie().GetRuntimeMinutes()
	}
	return r.putEntry(ctx, rec)
}

// putEntry persists the record and maintains its indexes: normalized-title
// and external-ID index rows.
func (r *Registry) putEntry(ctx context.Context, rec *entryRecord) error {
	blob, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("itemregistry: encode entry %q: %w", rec.ID, err)
	}
	if err := r.st.Put(ctx, entryKey(rec.ID), blob); err != nil {
		return err
	}
	if norm := identity.NormalizeTitle(rec.Title); norm != "" {
		if err := r.st.Put(ctx, titleIndexPrefix(norm, rec.Kind)+rec.ID, []byte{}); err != nil {
			return err
		}
	}
	for _, c := range rec.Claims {
		if c.Namespace == "" || c.Value == "" {
			continue
		}
		if err := r.st.Put(ctx, extIndexPrefix(c.Namespace, c.Value)+rec.ID, []byte{}); err != nil {
			return err
		}
	}
	return nil
}

func (r *Registry) getEntry(ctx context.Context, id string) (*entryRecord, error) {
	blob, err := r.st.Get(ctx, entryKey(id))
	if err == store.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rec entryRecord
	if err := json.Unmarshal(blob, &rec); err != nil {
		return nil, fmt.Errorf("itemregistry: decode entry %q: %w", id, err)
	}
	return &rec, nil
}

func (r *Registry) loadMapping(ctx context.Context, provider, nativeID string) (*mappingRecord, error) {
	blob, err := r.st.Get(ctx, mappingKey(provider, nativeID))
	if err == store.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rec mappingRecord
	if err := json.Unmarshal(blob, &rec); err != nil {
		return nil, fmt.Errorf("itemregistry: decode mapping %s/%s: %w", provider, nativeID, err)
	}
	return &rec, nil
}

// --- keys -----------------------------------------------------------------

func esc(s string) string { return url.PathEscape(s) }

func mappingKey(provider, nativeID string) string {
	return "reg/m/" + esc(provider) + "/" + esc(nativeID)
}

func aliasKey(provider, nativeID string, generation uint64) string {
	return fmt.Sprintf("reg/a/%s/%s/%016x", esc(provider), esc(nativeID), generation)
}

func entryKey(id string) string { return "reg/e/" + id }

func titleIndexPrefix(normTitle string, kind slotsv1.ItemKind) string {
	return "reg/it/" + esc(normTitle) + "/" + strconv.Itoa(int(kind)) + "/"
}

func extIndexPrefix(namespace, value string) string {
	return "reg/ix/" + esc(namespace) + "/" + esc(value) + "/"
}

// pathTail returns the last path segment of an index key: the entry id.
func pathTail(key string) string {
	if i := strings.LastIndex(key, "/"); i >= 0 {
		return key[i+1:]
	}
	return key
}

// --- small helpers --------------------------------------------------------

func newEntryID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("itemregistry: mint entry id: %w", err)
	}
	return "le_" + hex.EncodeToString(b[:]), nil
}

func marshal(rec mappingRecord) []byte {
	b, _ := json.Marshal(rec) // deterministic plain-Go struct; cannot fail
	return b
}

// claimsFromIDs seeds provenance-carrying claims from one provider's
// assertions.
func claimsFromIDs(ids []*slotsv1.ExternalId, supplier string) []idClaim {
	return unionClaims(nil, ids, supplier)
}

// unionClaims merges a provider's assertions into the claim set: known claims
// gain the supplier, unknown ones are appended.
func unionClaims(base []idClaim, add []*slotsv1.ExternalId, supplier string) []idClaim {
	out := base
	index := make(map[string]int, len(base)+len(add))
	for i, c := range out {
		index[c.key()] = i
	}
	for _, id := range add {
		if id == nil || id.GetNamespace() == "" || id.GetValue() == "" {
			continue
		}
		probe := idClaim{Namespace: id.GetNamespace(), Value: id.GetValue()}
		if i, ok := index[probe.key()]; ok {
			if !contains(out[i].Suppliers, supplier) {
				out[i].Suppliers = append(out[i].Suppliers, supplier)
				sort.Strings(out[i].Suppliers)
			}
			continue
		}
		probe.Suppliers = []string{supplier}
		index[probe.key()] = len(out)
		out = append(out, probe)
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// unionStrings appends members of add that base does not already contain,
// preserving order.
func unionStrings(base, add []string) []string {
	if len(add) == 0 {
		return base
	}
	set := make(map[string]bool, len(base)+len(add))
	for _, s := range base {
		set[s] = true
	}
	out := base
	for _, s := range add {
		if !set[s] {
			set[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out[len(base):])
	return out
}

func appendUniqueIDs(order []string, seen map[string]bool, keys []string) []string {
	for _, k := range keys {
		if id := pathTail(k); !seen[id] {
			seen[id] = true
			order = append(order, id)
		}
	}
	return order
}

func conflictEvent(provider, providerID, entryID, ownerID string) *corev1.EventEnvelope {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return &corev1.EventEnvelope{
		Id:       hex.EncodeToString(b[:]),
		Type:     corev1.EventType_EVENT_TYPE_MERGE_CONFLICT,
		Audience: corev1.EventAudience_EVENT_AUDIENCE_OWNER,
		UserId:   ownerID,
		Payload: &corev1.EventEnvelope_MergeConflict{
			MergeConflict: &corev1.MergeConflictEvent{
				Provider:   provider,
				ProviderId: providerID,
				EntryId:    entryID,
				Reason:     "provider item changed identity under a known id; entries kept apart pending corroboration",
			},
		},
		EmittedAt: timestamppb.Now(),
	}
}
