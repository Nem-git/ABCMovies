package app

import (
	"context"
	"log/slog"
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"

	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/library"
	"github.com/nem-git/abcmovies/core/internal/policy"
	"github.com/nem-git/abcmovies/core/internal/sourcecache"
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
	slots := []*corev1.Slot{{
		Id: "primary", Kind: corev1.SlotKind_SLOT_KIND_PROVIDER,
		Config: &corev1.Slot_Provider{Provider: &corev1.ProviderConfig{
			Adapter: "jellyfin", Server: "http://jf",
			Accounts: []*corev1.AccountConfig{{
				Id: "home", Username: "bob", PasswordEnv: "JF_PASSWORD",
				MaxConcurrentStreams: 4, Policy: map[string]string{policy.KeyConcurrentStreams: "1"},
			}},
		}},
	}}

	// Linked account: cap from the record, override nil (nothing declared).
	override, cap, err := resolveAccountConstraints(ctx, slots, accts, "primary", "lnk_alice")
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
	override, cap, err = resolveAccountConstraints(ctx, slots, accts, "primary", "home")
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
	override, cap, err = resolveAccountConstraints(ctx, slots, accts, "primary", "nope")
	if err != nil {
		t.Fatalf("unknown: %v", err)
	}
	if override != nil {
		t.Fatalf("unknown override = %v, want nil", override)
	}
	if _, declared := cap.Streams(); declared {
		t.Fatalf("unknown cap = %v, want undeclared", cap)
	}

	// Zero cap is unbound, not "admitting zero": the instance policy on its
	// own must govern.
	slots[0].GetProvider().GetAccounts()[0].MaxConcurrentStreams = 0
	_, cap, err = resolveAccountConstraints(ctx, slots, accts, "primary", "home")
	if err != nil {
		t.Fatalf("zero cap: %v", err)
	}
	if _, declared := cap.Streams(); declared {
		t.Fatalf("zero cap should mean unbound, got %v", cap)
	}
}

// scopedLibrary is a LibrarySeam that serves a fixed entry set per user, so a
// test can prove which user's library the caller's entry was resolved through.
type scopedLibrary struct {
	entries map[string][]*corev1.LibraryEntry
}

func (l scopedLibrary) Library(_ context.Context, userID string) ([]*corev1.LibraryEntry, error) {
	return l.entries[userID], nil
}

func (l scopedLibrary) Metadata(context.Context, string) (*corev1.TitleMetadata, bool, error) {
	return nil, false, nil
}

func (l scopedLibrary) ReachAuthorized(string, string) (library.Reach, bool) {
	return library.Reach{}, false
}

func (l scopedLibrary) ReachesForUser(string) []library.Reach { return nil }

func (l scopedLibrary) SetReachSharing(string, accounts.Visibility, []string) error { return nil }

func (l scopedLibrary) RemoveReach(string) {}

func (l scopedLibrary) RefreshAvailability(context.Context, string, []string) (sourcecache.Stats, error) {
	return sourcecache.Stats{}, nil
}

// GetDeliveryOptions resolves the entry through the *caller's* derived library
// (PLAN.md §5.1, §6.3): a regular user's entry from their own linked account
// must resolve for that user, and must not resolve for anyone else. The entry
// carries no coverage, so the engine's manifest resolver is never reached and
// the test stays hermetic.
func TestGetDeliveryOptionsScopesEntryToCaller(t *testing.T) {
	lib := scopedLibrary{entries: map[string][]*corev1.LibraryEntry{
		"user-1": {{Id: "le-1"}},
	}}
	m := managedDelivery{library: lib}

	resp, err := m.GetDeliveryOptions("le-1", "user-1")
	if err != nil {
		t.Fatalf("GetDeliveryOptions(user-1): %v", err)
	}
	if len(resp.GetSinks()) == 0 || len(resp.GetContainers()) == 0 {
		t.Fatalf("options = %+v, want a sink and containers", resp)
	}

	if _, err := m.GetDeliveryOptions("le-1", "user-2"); err == nil {
		t.Fatal("GetDeliveryOptions resolved an entry outside the caller's library")
	}
}
