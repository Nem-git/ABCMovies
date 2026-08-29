package serving

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/nem-git/abcmovies/core/app"
)

// debugMetadataHandler serves GET /debug/metadata: the records and alias
// index held in the metadata cache (whatever enrichment has accumulated).
type debugMetadataHandler struct{ stack *app.Stack }

func (h debugMetadataHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if _, ok := bearerUser(r, h.stack.Auth()); !ok {
		http.Error(w, "missing or invalid bearer token", http.StatusUnauthorized)
		return
	}
	c := h.stack.Slots().Meta
	records, err := c.ListRecords(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	aliases, err := c.ListAliases(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sort.Strings(records)
	writeJSON(w, map[string]any{"records": records, "aliases": aliases})
}

// debugRegistryHandler serves GET /debug/registry: the registry's entries,
// provider mappings, and any persisted merge conflicts.
type debugRegistryHandler struct{ stack *app.Stack }

func (h debugRegistryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if _, ok := bearerUser(r, h.stack.Auth()); !ok {
		http.Error(w, "missing or invalid bearer token", http.StatusUnauthorized)
		return
	}
	reg := h.stack.Slots().ItemRegistry
	entries, err := reg.ListEntries(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	mappings, err := reg.ListMappings(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	conflicts, err := reg.Conflicts(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"entries":   entries,
		"mappings":  mappings,
		"conflicts": conflicts,
	})
}

// debugSourceCacheHandler serves GET /debug/sourcecache: every reachable
// provider account and how many items it has cached.
type debugSourceCacheHandler struct{ stack *app.Stack }

func (h debugSourceCacheHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if _, ok := bearerUser(r, h.stack.Auth()); !ok {
		http.Error(w, "missing or invalid bearer token", http.StatusUnauthorized)
		return
	}
	reaches := h.stack.Slots().Library.Reaches()
	type accountView struct {
		Provider string `json:"provider"`
		Account  string `json:"accountId"`
		Items    int    `json:"items"`
	}
	var accounts []accountView
	for _, reach := range reaches {
		items, err := reach.Sync.ListItems(r.Context(), reach.AccountID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		accounts = append(accounts, accountView{
			Provider: reach.Sync.Provider(),
			Account:  reach.AccountID,
			Items:    len(items),
		})
	}
	sort.Slice(accounts, func(i, j int) bool {
		if accounts[i].Provider != accounts[j].Provider {
			return accounts[i].Provider < accounts[j].Provider
		}
		return accounts[i].Account < accounts[j].Account
	})
	writeJSON(w, map[string]any{"accounts": accounts})
}

// debugEnrichmentHandler serves GET /debug/enrichment: the pending backlog,
// the enabled catalogue slots, and the accumulated metadata-cache record
// count.
type debugEnrichmentHandler struct{ stack *app.Stack }

func (h debugEnrichmentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if _, ok := bearerUser(r, h.stack.Auth()); !ok {
		http.Error(w, "missing or invalid bearer token", http.StatusUnauthorized)
		return
	}
	slots := h.stack.Slots()
	records, err := slots.Meta.ListRecords(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	slotsList := make([]string, 0, len(slots.Catalogues))
	for _, c := range slots.Catalogues {
		slotsList = append(slotsList, c.Slot)
	}
	sort.Strings(slotsList)
	writeJSON(w, map[string]any{
		"pending":     slots.Queue.Pending(),
		"catalogues":  slotsList,
		"recordCount": len(records),
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
