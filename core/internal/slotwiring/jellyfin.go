package slotwiring

import (
	"context"
	"fmt"

	"github.com/nem-git/abcmovies/adapters/jellyfin"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/itemregistry"
	"github.com/nem-git/abcmovies/core/internal/library"
	"github.com/nem-git/abcmovies/core/internal/scheduler"
	"github.com/nem-git/abcmovies/core/internal/sourcecache"
)

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
// wires vault-backed session storage, and schedules each account's catalogue
// sync at the resolved cadence (config override > declared > default).
func wireJellyfin(entry config.SlotEntry, deps Deps) ([]scheduler.Job, []library.Reach, error) {
	accounts := make([]jellyfin.Account, 0, len(entry.Accounts))
	for _, a := range entry.Accounts {
		if a.ID == "" {
			return nil, nil, fmt.Errorf("account entry missing id")
		}
		accounts = append(accounts, jellyfin.Account{
			ID:          a.ID,
			URL:         a.URL,
			Username:    a.Username,
			PasswordEnv: a.PasswordEnv,
		})
	}

	slot, err := jellyfin.New(accounts, jellyfin.WithSessionVault(deps.SealedBlobs))
	if err != nil {
		return nil, nil, fmt.Errorf("build: %w", err)
	}
	caps, err := deps.Registry.Admit(entry.ID, slot)
	if err != nil {
		return nil, nil, fmt.Errorf("handshake: %w", err)
	}
	logAdmitted(deps.Logger, entry.ID, caps)

	policy, _ := deps.Registry.Policy(entry.ID)
	cadence, err := DeclaredCadence(entry.SyncCadence, policy, jellyfinCadenceKey)
	if err != nil {
		return nil, nil, fmt.Errorf("cadence: %w", err)
	}

	jobs := make([]scheduler.Job, 0, len(accounts))
	reaches := make([]library.Reach, 0, len(accounts))
	namespace := providerNamespace(entry)
	for _, acc := range accounts {
		if deps.ItemRegistry == nil {
			return nil, nil, fmt.Errorf("slot %q: identity work requires an item registry; none was wired", entry.ID)
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
			return nil, nil, fmt.Errorf("source cache: %w", err)
		}
		accountID := acc.ID
		reaches = append(reaches, library.Reach{Sync: syncer, AccountID: accountID})
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
	return jobs, reaches, nil
}
