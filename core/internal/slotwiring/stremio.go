package slotwiring

import (
	"context"
	"fmt"

	"github.com/nem-git/abcmovies/adapters/stremio"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/library"
)

func init() {
	RegisterProvider("stremio", wireStremio)
}

// stremioRoster is the core-side source of truth: operator-declared addon
// accounts from config (one anonymous identity per addon instance — there is
// no per-account login), plus linked accounts routed to this slot.
type stremioRoster struct {
	server string
	ops    map[string]stremio.Account
	store  *accounts.Store
}

func (r stremioRoster) Lookup(ctx context.Context, id string) (stremio.Account, error) {
	if a, ok := r.ops[id]; ok {
		return a, nil
	}
	if r.store != nil {
		rec, err := r.store.Get(ctx, id)
		if err != nil {
			return stremio.Account{}, fmt.Errorf("account %q: %w", id, err)
		}
		if canonicalServer(rec.BaseURL) != canonicalServer(r.server) {
			return stremio.Account{}, fmt.Errorf("account %q is not on this slot's addon", id)
		}
		return stremio.Account{ID: rec.ID, ManifestURL: rec.BaseURL}, nil
	}
	return stremio.Account{}, fmt.Errorf("account %q not declared", id)
}

// wireStremio builds a generic Stremio-addon slot. Unlike wireJellyfin it
// builds no catalogue sync and no per-account refresh job: a lazy provider
// only refreshes on usage (PLAN.md §5.4), so there is nothing to schedule
// and no initial source-cache sync. Availability arrives through the
// explicit RefreshAvailability seam, and produce-sources is resolved live.
// Runtime-linked addons are not accepted here yet: the slot does not
// implement the sourcecache-backed AttachableSlot contract, so such a link
// fails with a typed error rather than half-wiring a sync job.
func wireStremio(entry config.SlotEntry, deps Deps) (*BuiltSlot, error) {
	if entry.Server == "" {
		return nil, fmt.Errorf("slot %q: stremio adapter requires server (the addon manifest URL)", entry.ID)
	}
	ops := make(map[string]stremio.Account, len(entry.Accounts))
	ids := make([]string, 0, len(entry.Accounts)+len(deps.LinkedBySlot[entry.ID]))
	for _, a := range entry.Accounts {
		if a.ID == "" {
			return nil, fmt.Errorf("slot %q: account entry missing id", entry.ID)
		}
		ids = append(ids, a.ID)
		ops[a.ID] = stremio.Account{ID: a.ID, ManifestURL: entry.Server}
	}
	for _, rec := range deps.LinkedBySlot[entry.ID] {
		if canonicalServer(rec.BaseURL) != canonicalServer(entry.Server) {
			return nil, fmt.Errorf("slot %q serves addon %q but linked account %q is on %q",
				entry.ID, entry.Server, rec.ID, rec.BaseURL)
		}
		ids = append(ids, rec.ID)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("slot %q: stremio adapter requires at least one account entry", entry.ID)
	}
	slot, err := stremio.New(ids, stremioRoster{server: canonicalServer(entry.Server), ops: ops, store: deps.Accounts})
	if err != nil {
		return nil, fmt.Errorf("build: %w", err)
	}
	if _, err := deps.Registry.Describe(entry.ID, slot); err != nil {
		return nil, fmt.Errorf("handshake: %w", err)
	}
	reachesMeta := make([]reachMeta, 0, len(ids))
	for range entry.Accounts {
		reachesMeta = append(reachesMeta, reachMeta{visibility: accounts.VisibilityPublic})
	}
	for _, rec := range deps.LinkedBySlot[entry.ID] {
		reachesMeta = append(reachesMeta, reachMeta{owner: rec.OwnerUserID, visibility: rec.Visibility, members: rec.SharedWith})
	}
	pc := newPacedClient(entry, slot, slot, slot)
	reaches := make([]library.Reach, 0, len(ids))
	for i, accountID := range ids {
		syncer, reach, job, err := accountSyncMachine(providerNamespace(entry), accountID, pc, 0, reachesMeta[i], deps)
		if err != nil {
			return nil, err
		}
		_ = job // lazy providers never run a catalogue-sync job
		reaches = append(reaches, *reach)
		_ = syncer // kept alive via the reach's Sync field; RefreshItems runs on it
	}
	return &BuiltSlot{
		Entry:    entry,
		Impl:     slot,
		Reaches:  reaches,
		Resolver: producesResolver{pc: pc},
	}, nil
}
