package slotwiring

import (
	"context"
	"fmt"

	"github.com/nem-git/abcmovies/adapters/stremio"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/library"
)

func init() {
	RegisterProvider("stremio", wireStremio)
	// Stremio validates a linked account's addon URL itself, so it carries
	// the prober the accounts RPCs probe through (PLAN.md §3.5).
	RegisterProber("stremio", stremio.Prober{})
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
func wireStremio(slot *corev1.Slot, deps Deps) (*BuiltSlot, error) {
	prov := slot.GetProvider()
	server := prov.GetServer()
	id := slot.GetId()
	if server == "" {
		return nil, fmt.Errorf("slot %q: stremio adapter requires server (the addon manifest URL)", id)
	}
	slotAccounts := prov.GetAccounts()
	ops := make(map[string]stremio.Account, len(slotAccounts))
	ids := make([]string, 0, len(slotAccounts)+len(deps.LinkedBySlot[id]))
	for _, a := range slotAccounts {
		if a.GetId() == "" {
			return nil, fmt.Errorf("slot %q: account entry missing id", id)
		}
		ids = append(ids, a.GetId())
		ops[a.GetId()] = stremio.Account{ID: a.GetId(), ManifestURL: server}
	}
	for _, rec := range deps.LinkedBySlot[id] {
		if canonicalServer(rec.BaseURL) != canonicalServer(server) {
			return nil, fmt.Errorf("slot %q serves addon %q but linked account %q is on %q",
				id, server, rec.ID, rec.BaseURL)
		}
		ids = append(ids, rec.ID)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("slot %q: stremio adapter requires at least one account entry", id)
	}
	impl, err := stremio.New(ids, stremioRoster{server: canonicalServer(server), ops: ops, store: deps.Accounts})
	if err != nil {
		return nil, fmt.Errorf("build: %w", err)
	}
	if _, err := deps.Registry.Describe(id, impl); err != nil {
		return nil, fmt.Errorf("handshake: %w", err)
	}
	reachesMeta := make([]reachMeta, 0, len(ids))
	for range slotAccounts {
		reachesMeta = append(reachesMeta, reachMeta{visibility: accounts.VisibilityPublic})
	}
	for _, rec := range deps.LinkedBySlot[id] {
		reachesMeta = append(reachesMeta, reachMeta{owner: rec.OwnerUserID, visibility: rec.Visibility, members: rec.SharedWith})
	}
	pc := newPacedClient(slot, deps.InstancePolicy, impl, impl, impl, impl)
	reaches := make([]library.Reach, 0, len(ids))
	for i, accountID := range ids {
		syncer, reach, job, err := accountSyncMachine(providerNamespace(slot), accountID, pc, 0, reachesMeta[i], deps)
		if err != nil {
			return nil, err
		}
		_ = job // lazy providers never run a catalogue-sync job
		reaches = append(reaches, *reach)
		_ = syncer // kept alive via the reach's Sync field; RefreshItems runs on it
	}
	return &BuiltSlot{
		Entry:      slot,
		Impl:       impl,
		Reaches:    reaches,
		Resolver:   producesResolver{pc: pc},
		LazySearch: pc,
	}, nil
}
