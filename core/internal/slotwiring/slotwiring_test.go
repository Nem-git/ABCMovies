package slotwiring

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
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

	_, _, _, _, err := SetupProviders([]*corev1.Slot{{
		Id: "primary", Kind: corev1.SlotKind_SLOT_KIND_PROVIDER, // deliberate typo
		Config: &corev1.Slot_Provider{Provider: &corev1.ProviderConfig{Adapter: "jellifin"}},
	}}, Deps{Registry: reg, Logger: slog.Default()})
	if err == nil || !strings.Contains(err.Error(), "unknown provider adapter") {
		t.Fatalf("want unknown-adapter error, got %v", err)
	}
}

// TestUnimplementedKindsRejected pins that declaring a slot of a kind whose
// milestone has not landed fails startup loudly — DRM and subtitle-source are
// store-managed (D8) but their implementations are deferred, so a stored
// entry of either kind must refuse to compose rather than be silently
// ignored (which would make a typo look like a working deployment).
func TestUnimplementedKindsRejected(t *testing.T) {
	t.Parallel()
	cases := map[string]*corev1.Slot{
		"subtitle-source": {
			Id: "sub-a", Kind: corev1.SlotKind_SLOT_KIND_SUBTITLE_SOURCE,
			Config: &corev1.Slot_SubtitleSource{SubtitleSource: &corev1.SubtitleSourceConfig{Adapter: "trakt-sub"}},
		},
		"drm": {
			Id: "drm-a", Kind: corev1.SlotKind_SLOT_KIND_DRM,
			Config: &corev1.Slot_Drm{Drm: &corev1.DrmConfig{Adapter: "gowidevine"}},
		},
	}
	for name, slot := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, _, _, _, err := SetupAll(context.Background(), []*corev1.Slot{slot}, Deps{}); err == nil ||
				!strings.Contains(err.Error(), "not implemented yet") {
				t.Fatalf("want not-implemented-yet error, got %v", err)
			}
		})
	}
}

// TestUnknownCatalogueAdapterRejected pins loud failure for a typo'd
// catalogue adapter name.
func TestUnknownCatalogueAdapterRejected(t *testing.T) {
	t.Parallel()
	slots := []*corev1.Slot{{
		Id: "trakt", Kind: corev1.SlotKind_SLOT_KIND_CATALOGUE,
		Config: &corev1.Slot_Catalogue{Catalogue: &corev1.CatalogueConfig{Adapter: "trakt"}},
	}}

	if _, _, _, _, _, err := SetupAll(context.Background(), slots, Deps{}); err == nil ||
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

	_, _, _, _, err := SetupProviders([]*corev1.Slot{{
		Id: "primary", Kind: corev1.SlotKind_SLOT_KIND_PROVIDER,
		Config: &corev1.Slot_Provider{Provider: &corev1.ProviderConfig{
			Adapter: "jellyfin", Server: "http://jf.invalid",
			SyncCadence: "not-a-duration",
			Accounts:    []*corev1.AccountConfig{{Id: "home", Username: "bob", PasswordEnv: "JELLYFIN_TEST_PASSWORD"}},
		}},
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

	_, err := SetupCatalogues([]*corev1.Slot{
		{
			Id: "tmdb-a", Kind: corev1.SlotKind_SLOT_KIND_CATALOGUE,
			Config: &corev1.Slot_Catalogue{Catalogue: &corev1.CatalogueConfig{Adapter: "tmdb", TokenEnv: "TMDB_TOKEN"}},
		},
		{
			Id: "tmdb-b", Kind: corev1.SlotKind_SLOT_KIND_CATALOGUE,
			Config: &corev1.Slot_Catalogue{Catalogue: &corev1.CatalogueConfig{Adapter: "tmdb", TokenEnv: "TMDB_TOKEN"}},
		},
	}, Deps{Registry: reg, Logger: slog.Default()})
	if err == nil || !strings.Contains(err.Error(), "both claim identity namespace") {
		t.Fatalf("want overlap error, got %v", err)
	}
	if len(reg.Snapshot()) != 0 {
		t.Fatalf("registry = %v, want empty: overlap check fires before publication", reg.Snapshot())
	}
}

// TestSetupCataloguesAdmitsEachEntry pins the happy path: one catalogue is
// admitted and the engine client is served.
func TestSetupCataloguesAdmitsEachEntry(t *testing.T) {
	t.Setenv("TMDB_TOKEN", "test-token")
	reg := registry.NewInProcess()
	defer reg.Close()

	cats, err := SetupCatalogues([]*corev1.Slot{
		{
			Id: "tmdb-main", Kind: corev1.SlotKind_SLOT_KIND_CATALOGUE,
			Config: &corev1.Slot_Catalogue{Catalogue: &corev1.CatalogueConfig{Adapter: "tmdb", TokenEnv: "TMDB_TOKEN"}},
		},
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
	slot := &corev1.Slot{Id: "home-jellyfin", Kind: corev1.SlotKind_SLOT_KIND_PROVIDER}
	if got := providerNamespace(slot); got != "home-jellyfin" {
		t.Fatalf("provider namespace = %q, want the slot id", got)
	}
}
