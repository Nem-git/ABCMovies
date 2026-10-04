package app

import (
	"log/slog"
	"testing"

	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// The resolver keys off two stores: linked-account records for operator-user
// accounts and slot config for host-provided public ones. Neither path may
// invent state: unknown ids admit with the instance policy alone.
func TestResolveAccountConstraints(t *testing.T) {
	ctx := t.Context()
	vault := store.NewInMemory()
	accts := accounts.NewStore(vault, slog.Default())
	if err := accts.Add(ctx, accounts.Record{ID: "lnk_alice", Provider: "jellyfin", BaseURL: "http://jf", Username: "alice", MaxConcurrentStreams: 2}); err != nil {
		t.Fatalf("add: %v", err)
	}
	cfg := &config.Config{}
	cfg.Slots.Providers = []config.SlotEntry{{
		ID: "primary", Adapter: "jellyfin", Enabled: true,
		Accounts: []config.AccountConfig{{
			ID: "home", URL: "http://jf", Username: "bob", PasswordEnv: "JF_PASSWORD",
			MaxConcurrentStreams: 4, Policy: map[string]string{"concurrentStreams": "1"},
		}},
	}}

	// Linked account: cap from the record, override nil (nothing declared).
	override, cap, err := resolveAccountConstraints(ctx, cfg, accts, "primary", "lnk_alice")
	if err != nil {
		t.Fatalf("linked: %v", err)
	}
	if override != nil {
		t.Fatalf("linked override = %v, want nil", override)
	}
	if s, _ := cap.Streams(); s != 2 {
		t.Fatalf("linked cap = %d, want 2", s)
	}

	// Operator-declared account: cap and override both from config.
	override, cap, err = resolveAccountConstraints(ctx, cfg, accts, "primary", "home")
	if err != nil {
		t.Fatalf("operator: %v", err)
	}
	if s, _ := override.Streams(); s != 1 {
		t.Fatalf("operator override = %d, want 1", s)
	}
	if s, _ := cap.Streams(); s != 4 {
		t.Fatalf("operator cap = %d, want 4", s)
	}

	// Unknown: defaults admit; engine falls back to instance policy only.
	override, cap, err = resolveAccountConstraints(ctx, cfg, accts, "primary", "nope")
	if err != nil {
		t.Fatalf("unknown: %v", err)
	}
	if override != nil || cap != nil {
		t.Fatalf("unknown should be nil/nil, got %v %v", override, cap)
	}

	// Zero cap is unbound, not "admitting zero": the instance policy on its
	// own must govern.
	cfg.Slots.Providers[0].Accounts[0].MaxConcurrentStreams = 0
	_, cap, err = resolveAccountConstraints(ctx, cfg, accts, "primary", "home")
	if err != nil {
		t.Fatalf("zero cap: %v", err)
	}
	if cap != nil {
		t.Fatalf("zero cap should mean unbound (nil), got %v", cap)
	}
}
