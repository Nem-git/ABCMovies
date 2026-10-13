package slotwiring

import (
	"context"
	"fmt"
	"time"

	"github.com/nem-git/abcmovies/adapters/jellyfin"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/itemregistry"
	"github.com/nem-git/abcmovies/core/internal/library"
	"github.com/nem-git/abcmovies/core/internal/scheduler"
	"github.com/nem-git/abcmovies/core/internal/sourcecache"
)

// reachMeta is the per-account sharing metadata the wiring attaches to each
// derived reach: host-provided operator accounts are public, linked accounts
// carry the owner's visibility choice from the stored record (§5.1).
type reachMeta struct {
	owner      string
	visibility accounts.Visibility
	members    []string
}

// The cadence-policy key this adapter declares in its handshake.
const jellyfinCadenceKey = "browse.sync-cadence"

func init() {
	RegisterProvider("jellyfin", wireJellyfin)
	// Jellyfin validates a linked account's password itself, so it carries
	// the prober the accounts RPCs probe through (PLAN.md §3.5).
	RegisterProber("jellyfin", jellyfinProber{})
}

// slotRoster is the core-side source of truth the adapter consults for
// account details: operator-declared accounts come from config, linked
// accounts from the record store (which also holds their vaulted sessions).
// The adapter keeps no copy of this — only a per-account token cache — and
// linked accounts are always resolved as vault-first: PasswordEnv is empty,
// so a dead session is a re-link, never a silent re-login.
type slotRoster struct {
	server string
	ops    map[string]jellyfin.Account
	store  *accounts.Store
}

func (r slotRoster) Lookup(ctx context.Context, id string) (jellyfin.Account, error) {
	if a, ok := r.ops[id]; ok {
		return a, nil
	}
	rec, err := r.store.Get(ctx, id)
	if err != nil {
		return jellyfin.Account{}, fmt.Errorf("account %q: %w", id, err)
	}
	if canonicalServer(rec.BaseURL) != r.server {
		return jellyfin.Account{}, fmt.Errorf("account %q is not on this slot's server", id)
	}
	return jellyfin.Account{ID: rec.ID, URL: rec.BaseURL, Username: rec.Username}, nil
}

// registryResolver adapts the item registry to the synchronizer's
// ItemResolver: every synced item resolves behind the run's success boundary.
// Any status other than unchanged means identity work happened — a mapping
// was created, attached or its proof evolved — which is exactly the T2
// trigger: the affected entry becomes an enrichment candidate
// (TECHNICAL-DECISIONS.md §1.28). Unchanged mappings enqueue nothing.
type registryResolver struct {
	r      *itemregistry.Registry
	notify func(entryID string)
}

func (a registryResolver) Resolve(ctx context.Context, provider string, item *slotsv1.CatalogueItem) error {
	out, err := a.r.Resolve(ctx, provider, item)
	if err != nil {
		return err
	}
	if a.notify != nil && out.Status != itemregistry.StatusUnchanged {
		a.notify(out.EntryID)
	}
	return nil
}

// providerNamespace is the string that scopes everything this slot instance
// owns in shared state: source-cache keys, registry mappings, event payloads
// (TECHNICAL-DECISIONS.md §1.25). The adapter name cannot disambiguate two
// deployed instances of one adapter; the slot id can.
func providerNamespace(slot *corev1.Slot) string {
	return slot.GetId()
}

// wireJellyfin builds one Jellyfin slot instance under its configured id:
// accounts come from operator config *and* the linked accounts routed to
// this slot (same server, §1.25), the vault-backed session store is wired in,
// each account's source cache is built and synced once, and the
// handshake-declared cadence is resolved into refresh jobs. It returns the
// unpublished slot — the composition root admits it as its final step, so
// this function never publishes on its caller's behalf (publish last).
func wireJellyfin(slot *corev1.Slot, deps Deps) (*BuiltSlot, error) {
	prov := slot.GetProvider()
	slotAccounts := prov.GetAccounts()
	server := prov.GetServer()
	id := slot.GetId()
	// Operator-declared accounts are host-provided and public (PLAN.md
	// §2.2): every user may derive them into a library.
	ops := make(map[string]jellyfin.Account, len(slotAccounts))
	ids := make([]string, 0, len(slotAccounts)+len(deps.LinkedBySlot[id]))
	reachesMeta := make([]reachMeta, 0, len(slotAccounts)+len(deps.LinkedBySlot[id]))
	for _, a := range slotAccounts {
		if a.GetId() == "" {
			return nil, fmt.Errorf("account entry missing id")
		}
		ids = append(ids, a.GetId())
		ops[a.GetId()] = jellyfin.Account{
			ID:          a.GetId(),
			URL:         server,
			Username:    a.GetUsername(),
			PasswordEnv: a.GetPasswordEnv(),
		}
		reachesMeta = append(reachesMeta, reachMeta{visibility: accounts.VisibilityPublic})
	}
	// Linked accounts join the same slot as the operator accounts of the same
	// server: they carry no password-env — their session was validated and
	// vaulted at link time (§3.5) and is restored by the adapter. Sharing
	// follows the record the owner chose at link time (§5.1).
	for _, rec := range deps.LinkedBySlot[id] {
		if canonicalServer(rec.BaseURL) != canonicalServer(server) {
			return nil, fmt.Errorf("slot %q serves %q but linked account %q is on %q",
				id, server, rec.ID, rec.BaseURL)
		}
		ids = append(ids, rec.ID)
		reachesMeta = append(reachesMeta, reachMeta{
			owner:      rec.OwnerUserID,
			visibility: rec.Visibility,
			members:    rec.SharedWith,
		})
	}

	opts := []jellyfin.Option{}
	if deps.Accounts != nil {
		opts = append(opts, jellyfin.WithSessionVault(deps.Accounts))
	}
	src := slotRoster{canonicalServer(server), ops, deps.Accounts}
	impl, err := jellyfin.New(ids, src, opts...)
	if err != nil {
		return nil, fmt.Errorf("build: %w", err)
	}
	info, err := deps.Registry.Describe(id, impl)
	if err != nil {
		return nil, fmt.Errorf("handshake: %w", err)
	}

	cadence, err := DeclaredCadence(prov.GetSyncCadence(), info.Policy, jellyfinCadenceKey)
	if err != nil {
		return nil, fmt.Errorf("cadence: %w", err)
	}

	jobs := make([]scheduler.Job, 0, len(ids))
	reaches := make([]library.Reach, 0, len(ids))
	// A provider slot's calls to the upstream server are paced through one
	// Gate composition: the slot-wide governor (§7.2) plus the per-account
	// budget derived from each account's policy map (§1.35).
	pc := newPacedClient(slot, deps.InstancePolicy, impl, nil, impl, nil)
	namespace := providerNamespace(slot)
	for i, accountID := range ids {
		syncer, reach, job, err := accountSyncMachine(namespace, accountID, pc, cadence, reachesMeta[i], deps)
		if err != nil {
			return nil, err
		}
		reaches = append(reaches, *reach)
		jobs = append(jobs, *job)
		if _, err := syncer.SyncAccount(deps.Ctx, accountID); err != nil {
			deps.Logger.Warn("initial source-cache sync failed; will retry on cadence",
				"slot", id, "account", accountID, "error", err)
		}
	}
	return &BuiltSlot{
		Entry:    slot,
		Impl:     impl,
		Jobs:     jobs,
		Reaches:  reaches,
		Resolver: producesResolver{pc: pc},
		Cadence:  cadence,
	}, nil
}

// accountSyncMachine builds the per-account sync machinery for one account of
// a provider slot: the source-cache synchronizer, the derived-library reach
// that exposes its cached items, and the recurring refresh job. wireJellyfin
// builds these at boot and a runtime link builds them again, one account at a
// time — one code path, no slot rebuild. The first sync is the caller's job:
// boot runs it inline so items exist before the API serves; a runtime link
// runs it in the background because LinkAccount must not block on a paged sync.
func accountSyncMachine(namespace, accountID string, client sourcecache.Client, cadence time.Duration, meta reachMeta, deps Deps) (*sourcecache.Synchronizer, *library.Reach, *scheduler.Job, error) {
	if deps.ItemRegistry == nil {
		return nil, nil, nil, fmt.Errorf("slot %q: identity work requires an item registry; none was wired", namespace)
	}
	opts := []sourcecache.Option{
		sourcecache.WithEntryLookup(deps.ItemRegistry),
		sourcecache.WithItemResolver(registryResolver{r: deps.ItemRegistry, notify: deps.Enqueue}),
	}
	if deps.EventSink != nil {
		opts = append(opts, sourcecache.WithEventsSink(deps.EventSink))
	}
	// Lazy provider slots implement the refresh surface too; wiring it in lets
	// every provider, catalogue-sync or lazy, serve RefreshItems through the
	// same synchronizer. A slot whose paced wrapper has no real refresh
	// backend is left unwired rather than failing on a nil surface.
	if rc, ok := client.(interface{ HasRefresh() bool }); ok && rc.HasRefresh() {
		rcClient, _ := client.(sourcecache.RefreshClient)
		opts = append(opts, sourcecache.WithRefreshClient(rcClient))
	}
	syncer, err := sourcecache.New(namespace, client, deps.SourceCache, deps.Logger, opts...)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("source cache: %w", err)
	}
	reach := &library.Reach{
		Sync:       syncer,
		AccountID:  accountID,
		Owner:      meta.owner,
		Visibility: meta.visibility,
		Members:    meta.members,
	}
	job := &scheduler.Job{
		Name:    "source-cache-sync/" + namespace + "/" + accountID,
		Cadence: cadence,
		Run: func(jobCtx context.Context) error {
			_, err := syncer.SyncAccount(jobCtx, accountID)
			return err
		},
	}
	return syncer, reach, job, nil
}

// jellyfinProber validates a linked-account credential against any Jellyfin
// server through the adapter's exported probe (PLAN.md §3.5). The probe is
// server-agnostic: it authenticates the driver directly, so one prober serves
// every Jellyfin slot regardless of how the operator sliced their servers.
type jellyfinProber struct{}

func (jellyfinProber) Probe(ctx context.Context, baseURL, username string, password []byte) ([]byte, error) {
	return jellyfin.ProbeCredentials(ctx, baseURL, username, password)
}
