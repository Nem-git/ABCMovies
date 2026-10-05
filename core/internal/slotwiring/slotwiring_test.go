package slotwiring

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/registry"
)

func TestDeclaredCadencePrecedence(t *testing.T) {
	t.Parallel()
	declared := map[string]string{"browse.sync-cadence": "6h"}

	tests := []struct {
		name        string
		configValue string
		declared    map[string]string
		want        time.Duration
		wantErr     bool
	}{
		{name: "explicit config wins", configValue: "15m", declared: declared, want: 15 * time.Minute},
		{name: "declared used when config empty", configValue: "", declared: declared, want: 6 * time.Hour},
		{
			name:        "bad explicit config is an error, not a fallback",
			configValue: "every-full-moon",
			declared:    declared,
			wantErr:     true,
		},
		{
			name:        "non-positive explicit config is an error",
			configValue: "0s",
			declared:    declared,
			wantErr:     true,
		},
		{
			name:        "broken adapter declaration is an error",
			configValue: "",
			declared:    map[string]string{"browse.sync-cadence": "soon"},
			wantErr:     true,
		},
		{name: "nothing declared anywhere falls back to zero", configValue: "", declared: nil, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := DeclaredCadence(tt.configValue, tt.declared, "browse.sync-cadence")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// TestUnknownAdapterRejected pins that a typo'd adapter name aborts startup
// instead of silently producing a core with fewer slots.
func TestUnknownAdapterRejected(t *testing.T) {
	t.Parallel()
	reg := registry.NewInProcess()
	defer reg.Close()

	_, _, _, err := SetupProviders([]config.SlotEntry{{
		ID: "primary", Adapter: "jellifin", Enabled: true, // deliberate typo
	}}, Deps{Registry: reg, Logger: slog.Default()})
	if err == nil || !strings.Contains(err.Error(), "unknown provider adapter") {
		t.Fatalf("want unknown-adapter error, got %v", err)
	}
}

// TestDisabledEntriesAreSkipped pins that enabled:false means the slot never
// reaches its factory — a disabled Jellyfin must not require credentials.
func TestDisabledEntriesAreSkipped(t *testing.T) {
	t.Parallel()
	reg := registry.NewInProcess()
	defer reg.Close()

	jobs, _, _, err := SetupProviders([]config.SlotEntry{{
		ID: "primary", Adapter: "jellyfin", Enabled: false,
		Accounts: []config.AccountConfig{{ID: "primary"}}, // no credentials on purpose
	}}, Deps{Registry: reg, Logger: slog.Default()})
	if err != nil {
		t.Fatalf("disabled entry must not be wired: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected no jobs, got %v", jobs)
	}
}

// TestUnimplementedKindsRejected pins that declaring a slot of a kind whose
// milestone has not landed fails startup loudly.
func TestUnimplementedKindsRejected(t *testing.T) {
	t.Parallel()
	slots := config.SlotsConfig{}
	slots.SubtitleSources = []config.SlotEntry{{ID: "sub-a", Adapter: "trakt-sub", Enabled: true}}

	if _, _, _, _, err := SetupAll(context.Background(), slots, Deps{}); err == nil ||
		!strings.Contains(err.Error(), "not implemented yet") {
		t.Fatalf("want not-implemented-yet error, got %v", err)
	}
}

// TestUnknownCatalogueAdapterRejected pins loud failure for a typo'd
// catalogue adapter name.
func TestUnknownCatalogueAdapterRejected(t *testing.T) {
	t.Parallel()
	slots := config.SlotsConfig{}
	slots.Catalogue = []config.SlotEntry{{ID: "trakt", Adapter: "trakt", Enabled: true}}

	if _, _, _, _, err := SetupAll(context.Background(), slots, Deps{}); err == nil ||
		!strings.Contains(err.Error(), "unknown catalogue adapter") {
		t.Fatalf("want unknown-catalogue-adapter error, got %v", err)
	}
}

// TestSetupProvidersPublishesLastOnLateFailure pins the publish-last order:
// a slot that fails at a late build step (here the cadence is unparseable,
// after the adapter was already built and synced) must leave nothing behind
// in the registry.
func TestSetupProvidersPublishesLastOnLateFailure(t *testing.T) {
	t.Setenv("JELLYFIN_TEST_PASSWORD", "x")
	reg := registry.NewInProcess()
	defer reg.Close()

	_, _, _, err := SetupProviders([]config.SlotEntry{{
		ID: "primary", Adapter: "jellyfin", Enabled: true, Server: "http://jf.invalid",
		SyncCadence: "not-a-duration",
		Accounts:    []config.AccountConfig{{ID: "home", Username: "bob", PasswordEnv: "JELLYFIN_TEST_PASSWORD"}},
	}}, Deps{Registry: reg, Logger: slog.Default()})
	if err == nil || !strings.Contains(err.Error(), "sync-cadence") {
		t.Fatalf("want cadence error, got %v", err)
	}
	if _, ok := reg.Policy("primary"); ok {
		t.Fatal("registry holds a slot that was never published")
	}
	if len(reg.Snapshot()) != 0 {
		t.Fatalf("registry = %v, want empty", reg.Snapshot())
	}
}

// TestSetupCataloguesRejectsOverlappingNamespaces pins the overlap check:
// two catalogue slots claiming the same identity namespace fail startup
// before either is admitted. (The check used to be dead code — it probed
// the narrowed engine client instead of the served slot, which does not
// forward the claims.)
func TestSetupCataloguesRejectsOverlappingNamespaces(t *testing.T) {
	t.Setenv("TMDB_TOKEN", "test-token")
	reg := registry.NewInProcess()
	defer reg.Close()

	_, err := SetupCatalogues([]config.SlotEntry{
		{ID: "tmdb-a", Adapter: "tmdb", Enabled: true, TokenEnv: "TMDB_TOKEN"},
		{ID: "tmdb-b", Adapter: "tmdb", Enabled: true, TokenEnv: "TMDB_TOKEN"},
	}, Deps{Registry: reg, Logger: slog.Default()})
	if err == nil || !strings.Contains(err.Error(), "both claim identity namespace") {
		t.Fatalf("want overlap error, got %v", err)
	}
	if len(reg.Snapshot()) != 0 {
		t.Fatalf("registry = %v, want empty: overlap check fires before publication", reg.Snapshot())
	}
}

// TestSetupCataloguesAdmitsEachEnabledEntry pins the happy path: one enabled
// catalogue is admitted and the engine client is served.
func TestSetupCataloguesAdmitsEachEnabledEntry(t *testing.T) {
	t.Setenv("TMDB_TOKEN", "test-token")
	reg := registry.NewInProcess()
	defer reg.Close()

	cats, err := SetupCatalogues([]config.SlotEntry{
		{ID: "tmdb-main", Adapter: "tmdb", Enabled: true, TokenEnv: "TMDB_TOKEN"},
	}, Deps{Registry: reg, Logger: slog.Default()})
	if err != nil {
		t.Fatalf("SetupCatalogues: %v", err)
	}
	if len(cats) != 1 || cats[0].Slot != "tmdb-main" {
		t.Fatalf("cats = %v, want one catalogue tmdb-main", cats)
	}
	if _, ok := reg.Policy("tmdb-main"); !ok {
		t.Fatal("registry lacks the admitted catalogue slot")
	}
}

// TestProviderNamespaceIsSlotID pins the identity-scoping convention
// (TECHNICAL-DECISIONS.md §1.25): two instances of one adapter must not share
// a provider namespace, or their item ids would collide.
func TestProviderNamespaceIsSlotID(t *testing.T) {
	t.Parallel()
	entry := config.SlotEntry{Adapter: "jellyfin", ID: "home-jellyfin"}
	if got := providerNamespace(entry); got != "home-jellyfin" {
		t.Fatalf("provider namespace = %q, want the slot id", got)
	}
}
