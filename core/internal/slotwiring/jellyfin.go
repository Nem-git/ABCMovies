package slotwiring

import (
	"context"
	"fmt"

	"github.com/nem-git/abcmovies/adapters/jellyfin"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/delivery"
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
func providerNamespace(entry config.SlotEntry) string {
	return entry.ID
}

// wireJellyfin admits one Jellyfin slot instance under its configured id,
// builds its accounts from operator config *and* the linked accounts routed
// to this slot (same server namespace, §1.25), wires the vault-backed session
// store, schedules each account's catalogue sync at the resolved cadence
// (config override > declared > default), and hands back the slot's
// produce-sources resolver for the delivery engine. A provisioned user-owned
// server arrives here as a synthetic entry carrying exactly one linked
// account with no operator accounts.
func wireJellyfin(entry config.SlotEntry, deps Deps) ([]scheduler.Job, []library.Reach, delivery.Resolver, error) {
	accts := make([]jellyfin.Account, 0, len(entry.Accounts)+len(deps.LinkedBySlot[entry.ID]))
	reachesMeta := make([]reachMeta, 0, cap(accts))
	for _, a := range entry.Accounts {
		if a.ID == "" {
			return nil, nil, nil, fmt.Errorf("account entry missing id")
		}
		accts = append(accts, jellyfin.Account{
			ID:          a.ID,
			URL:         a.URL,
			Username:    a.Username,
			PasswordEnv: a.PasswordEnv,
		})
		// Operator-declared accounts are host-provided and public (PLAN.md
		// §2.2): every user may derive them into a library.
		reachesMeta = append(reachesMeta, reachMeta{visibility: accounts.VisibilityPublic})
	}
	// Linked accounts join the same slot as the operator accounts of the same
	// server: they carry no password-env — their session was validated and
	// vaulted at link time (§3.5) and is restored by the adapter. Sharing
	// follows the record the owner chose at link time (§5.1).
	for _, rec := range deps.LinkedBySlot[entry.ID] {
		accts = append(accts, jellyfin.Account{
			ID:       rec.ID,
			URL:      rec.BaseURL,
			Username: rec.Username,
		})
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
	slot, err := jellyfin.New(accts, opts...)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build: %w", err)
	}
	caps, err := deps.Registry.Admit(entry.ID, slot)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("handshake: %w", err)
	}
	logAdmitted(deps.Logger, entry.ID, caps)

	policy, _ := deps.Registry.Policy(entry.ID)
	cadence, err := DeclaredCadence(entry.SyncCadence, policy, jellyfinCadenceKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("cadence: %w", err)
	}

	jobs := make([]scheduler.Job, 0, len(accts))
	reaches := make([]library.Reach, 0, len(accts))
	namespace := providerNamespace(entry)
	for i, acc := range accts {
		if deps.ItemRegistry == nil {
			return nil, nil, nil, fmt.Errorf("slot %q: identity work requires an item registry; none was wired", entry.ID)
		}
		opts := []sourcecache.Option{
			sourcecache.WithEntryLookup(deps.ItemRegistry),
			sourcecache.WithItemResolver(registryResolver{r: deps.ItemRegistry, notify: deps.Enqueue}),
		}
		if deps.EventSink != nil {
			opts = append(opts, sourcecache.WithEventsSink(deps.EventSink))
		}
		syncer, err := sourcecache.New(namespace, slot, deps.SourceCache, deps.Logger, opts...)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("source cache: %w", err)
		}
		accountID := acc.ID
		reaches = append(reaches, library.Reach{
			Sync:       syncer,
			AccountID:  accountID,
			Owner:      reachesMeta[i].owner,
			Visibility: reachesMeta[i].visibility,
			Members:    reachesMeta[i].members,
		})
		if _, err := syncer.SyncAccount(deps.Ctx, accountID); err != nil {
			deps.Logger.Warn("initial source-cache sync failed; will retry on cadence",
				"slot", entry.ID, "account", accountID, "error", err)
		}
		jobs = append(jobs, scheduler.Job{
			Name:    "source-cache-sync/" + entry.ID + "/" + accountID,
			Cadence: cadence,
			Run: func(jobCtx context.Context) error {
				_, err := syncer.SyncAccount(jobCtx, accountID)
				return err
			},
		})
	}
	return jobs, reaches, jellyfinResolver{slot: slot}, nil
}

// jellyfinResolver adapts the Jellyfin slot's ProduceSources to the delivery
// engine's Resolver surface. The provider identity is assigned by the caller
// (the slot id, §1.25); here we only bridge account + native id to the adapter.
type jellyfinResolver struct {
	slot *jellyfin.Slot
}

func (r jellyfinResolver) ProduceSources(ctx context.Context, provider, accountID, nativeID string) (*corev1.MediaSource, error) {
	resp, err := r.slot.ProduceSources(ctx, &slotsv1.ProduceSourcesRequest{
		AccountId: accountID,
		NativeId:  nativeID,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetSource(), nil
}

// jellyfinProber validates a linked-account credential against any Jellyfin
// server through the adapter's exported probe (PLAN.md §3.5). The probe is
// server-agnostic: it authenticates the driver directly, so one prober serves
// every Jellyfin slot regardless of how the operator sliced their servers.
type jellyfinProber struct{}

func (jellyfinProber) Probe(ctx context.Context, baseURL, username string, password []byte) ([]byte, error) {
	return jellyfin.ProbeCredentials(ctx, baseURL, username, password)
}

// ProberForAdapter returns the credential prober registered for an adapter,
// or nil when the adapter does not validate a linked account's credentials by
// itself (PLAN.md §3.5: the core never vaults material it has not confirmed
// works; adapters without a prober cannot be linked). The key is the adapter
// name — the same value a LinkAccountRequest carries as its provider.
func ProberForAdapter(adapter string) apiserver.CredentialProber {
	switch adapter {
	case "jellyfin":
		return jellyfinProber{}
	default:
		return nil
	}
}
