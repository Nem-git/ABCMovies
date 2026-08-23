package slotwiring

import (
	"context"
	"fmt"

	"github.com/nem-git/abcmovies/adapters/jellyfin"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/scheduler"
	"github.com/nem-git/abcmovies/core/internal/sourcecache"
)

// The cadence-policy key this adapter declares in its handshake.
const jellyfinCadenceKey = "browse.sync-cadence"

func init() {
	RegisterProvider("jellyfin", wireJellyfin)
}

// wireJellyfin admits one Jellyfin slot instance under its configured id,
// wires vault-backed session storage, and schedules each account's catalogue
// sync at the resolved cadence (config override > declared > default).
func wireJellyfin(entry config.SlotEntry, deps Deps) ([]scheduler.Job, error) {
	accounts := make([]jellyfin.Account, 0, len(entry.Accounts))
	for _, a := range entry.Accounts {
		if a.ID == "" {
			return nil, fmt.Errorf("account entry missing id")
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
		return nil, fmt.Errorf("build: %w", err)
	}
	caps, err := deps.Registry.Admit(entry.ID, slot)
	if err != nil {
		return nil, fmt.Errorf("handshake: %w", err)
	}
	logAdmitted(deps.Logger, entry.ID, caps)

	policy, _ := deps.Registry.Policy(entry.ID)
	cadence, err := DeclaredCadence(entry.SyncCadence, policy, jellyfinCadenceKey)
	if err != nil {
		return nil, fmt.Errorf("cadence: %w", err)
	}

	jobs := make([]scheduler.Job, 0, len(accounts))
	for _, acc := range accounts {
		syncer, err := sourcecache.New(entry.Adapter, slot, deps.SourceCache, deps.Logger)
		if err != nil {
			return nil, fmt.Errorf("source cache: %w", err)
		}
		accountID := acc.ID
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
	return jobs, nil
}
