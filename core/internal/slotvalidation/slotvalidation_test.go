package slotvalidation

import (
	"strings"
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/policy"
)

func provider(id string) *corev1.Slot {
	s := &corev1.Slot{Id: id, Kind: corev1.SlotKind_SLOT_KIND_PROVIDER}
	s.Config = &corev1.Slot_Provider{Provider: &corev1.ProviderConfig{
		Adapter: "jellyfin",
		Server:  "https://jf.example",
	}}
	return s
}

func sink(id string) *corev1.Slot {
	s := &corev1.Slot{Id: id, Kind: corev1.SlotKind_SLOT_KIND_SINK}
	s.Config = &corev1.Slot_Sink{Sink: &corev1.SinkConfig{Adapter: "device"}}
	return s
}

func catalogue(id string) *corev1.Slot {
	s := &corev1.Slot{Id: id, Kind: corev1.SlotKind_SLOT_KIND_CATALOGUE}
	s.Config = &corev1.Slot_Catalogue{Catalogue: &corev1.CatalogueConfig{
		Adapter:  "tmdb",
		TokenEnv: "TMDB_TOKEN",
	}}
	return s
}

func subtitleSource(id string) *corev1.Slot {
	s := &corev1.Slot{Id: id, Kind: corev1.SlotKind_SLOT_KIND_SUBTITLE_SOURCE}
	s.Config = &corev1.Slot_SubtitleSource{SubtitleSource: &corev1.SubtitleSourceConfig{Adapter: "opensubtitles"}}
	return s
}

func drm(id string) *corev1.Slot {
	s := &corev1.Slot{Id: id, Kind: corev1.SlotKind_SLOT_KIND_DRM}
	s.Config = &corev1.Slot_Drm{Drm: &corev1.DrmConfig{Adapter: "gowidevine"}}
	return s
}

// TestValidateSlot_Accepts pins that a well-formed slot of every kind is
// accepted. A positive case per kind keeps a new kind from shipping with no
// accepted shape.
func TestValidateSlot_Accepts(t *testing.T) {
	for _, slot := range []*corev1.Slot{
		provider("jf"), sink("device"), catalogue("tmdb"), subtitleSource("subs"), drm("drm"),
	} {
		if err := ValidateSlot(slot); err != nil {
			t.Errorf("ValidateSlot(%s/%v): %v", slot.GetId(), slot.GetKind(), err)
		}
	}
}

func TestValidateSlotRejections(t *testing.T) {
	tests := []struct {
		name string
		slot *corev1.Slot
		want string
	}{
		{"nil", nil, "nil"},
		{"missing-id", &corev1.Slot{Kind: corev1.SlotKind_SLOT_KIND_SINK, Config: &corev1.Slot_Sink{Sink: &corev1.SinkConfig{Adapter: "device"}}}, "id is required"},
		{"unspecified-kind", &corev1.Slot{Id: "x"}, "kind is required"},
		{"unknown-kind", &corev1.Slot{Id: "x", Kind: corev1.SlotKind(99)}, "unknown kind"},

		{"provider-nil-config", &corev1.Slot{Id: "x", Kind: corev1.SlotKind_SLOT_KIND_PROVIDER}, "config is required"},
		{"provider-missing-adapter", configProvider("x", "", "https://jf.example"), "adapter is required"},
		{"provider-missing-server", configProvider("x", "jellyfin", ""), "server is required"},
		{"provider-account-missing-id", providerWithAccount(&corev1.AccountConfig{Username: "a", PasswordEnv: "P"}), "account entry missing id"},
		{"provider-account-missing-username", providerWithAccount(&corev1.AccountConfig{Id: "a", PasswordEnv: "P"}), "username is required"},
		{"provider-account-missing-password-env", providerWithAccount(&corev1.AccountConfig{Id: "a", Username: "u"}), "password_env is required"},
		{"provider-account-bad-policy", providerWithAccount(&corev1.AccountConfig{Id: "a", Username: "u", PasswordEnv: "P", Policy: map[string]string{"notAKey": "1"}}), "policy"},

		{"sink-nil-config", &corev1.Slot{Id: "x", Kind: corev1.SlotKind_SLOT_KIND_SINK}, "config is required"},
		{"sink-missing-adapter", configSink("x", ""), "adapter is required"},

		{"catalogue-nil-config", &corev1.Slot{Id: "x", Kind: corev1.SlotKind_SLOT_KIND_CATALOGUE}, "config is required"},
		{"catalogue-missing-adapter", configCatalogue("x", "", "T"), "adapter is required"},
		{"catalogue-missing-token-env", configCatalogue("x", "tmdb", ""), "token_env is required"},

		{"subtitle-nil-config", &corev1.Slot{Id: "x", Kind: corev1.SlotKind_SLOT_KIND_SUBTITLE_SOURCE}, "config is required"},
		{"subtitle-missing-adapter", configSubtitle("x", ""), "adapter is required"},

		{"drm-nil-config", &corev1.Slot{Id: "x", Kind: corev1.SlotKind_SLOT_KIND_DRM}, "config is required"},
		{"drm-missing-adapter", configDrm("x"), "adapter is required"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSlot(tc.slot)
			if err == nil {
				t.Fatalf("ValidateSlot = nil, want error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateSlot error = %q, want it to contain %q", err, tc.want)
			}
		})
	}
}

// TestValidateSlot_AccountPolicyOverlay pins the migrated rule: an account's
// policy override is validated at submit time, so an unknown key or an
// unparseable value refuses the slot rather than silently falling back to the
// instance policy.
func TestValidateSlot_AccountPolicyOverlay(t *testing.T) {
	ok := providerWithAccount(&corev1.AccountConfig{
		Id: "a", Username: "u", PasswordEnv: "P",
		Policy: map[string]string{policy.KeyConcurrentStreams: "1"},
	})
	if err := ValidateSlot(ok); err != nil {
		t.Fatalf("valid overlay rejected: %v", err)
	}

	unknown := providerWithAccount(&corev1.AccountConfig{
		Id: "a", Username: "u", PasswordEnv: "P",
		Policy: map[string]string{"notARealKey": "1"},
	})
	if err := ValidateSlot(unknown); err == nil {
		t.Fatal("unknown policy key should be rejected")
	}

	malformed := providerWithAccount(&corev1.AccountConfig{
		Id: "a", Username: "u", PasswordEnv: "P",
		Policy: map[string]string{policy.KeyConcurrentStreams: "not-a-number"},
	})
	if err := ValidateSlot(malformed); err == nil {
		t.Fatal("malformed policy value should be rejected")
	}
}

// TestValidateSlot_OptionsPacing pins the strict pacing knobs in a slot's
// options bag: a mis-typed value refuses the slot instead of silently running
// with no bound.
func TestValidateSlot_OptionsPacing(t *testing.T) {
	valid := provider("jf")
	valid.GetProvider().Options = map[string]string{
		policy.KeyPacingRequestsPerSec: "2.5",
		policy.KeyPacingMaxPulls:       "4",
		policy.KeyPacingDelay:          "250ms",
	}
	if err := ValidateSlot(valid); err != nil {
		t.Fatalf("valid pacing options rejected: %v", err)
	}

	for _, tc := range []struct {
		name string
		opts map[string]string
	}{
		{"requests-per-sec-not-number", map[string]string{policy.KeyPacingRequestsPerSec: "fast"}},
		{"requests-per-sec-not-positive", map[string]string{policy.KeyPacingRequestsPerSec: "0"}},
		{"max-pulls-not-integer", map[string]string{policy.KeyPacingMaxPulls: "many"}},
		{"max-pulls-not-positive", map[string]string{policy.KeyPacingMaxPulls: "0"}},
		{"delay-not-duration", map[string]string{policy.KeyPacingDelay: "soon"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := provider("jf")
			s.GetProvider().Options = tc.opts
			if err := ValidateSlot(s); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func configProvider(id, adapter, server string) *corev1.Slot {
	s := &corev1.Slot{Id: id, Kind: corev1.SlotKind_SLOT_KIND_PROVIDER}
	s.Config = &corev1.Slot_Provider{Provider: &corev1.ProviderConfig{Adapter: adapter, Server: server}}
	return s
}

func providerWithAccount(a *corev1.AccountConfig) *corev1.Slot {
	s := provider("jf")
	s.GetProvider().Accounts = []*corev1.AccountConfig{a}
	return s
}

func configSink(id, adapter string) *corev1.Slot {
	s := &corev1.Slot{Id: id, Kind: corev1.SlotKind_SLOT_KIND_SINK}
	s.Config = &corev1.Slot_Sink{Sink: &corev1.SinkConfig{Adapter: adapter}}
	return s
}

func configCatalogue(id, adapter, tokenEnv string) *corev1.Slot {
	s := &corev1.Slot{Id: id, Kind: corev1.SlotKind_SLOT_KIND_CATALOGUE}
	s.Config = &corev1.Slot_Catalogue{Catalogue: &corev1.CatalogueConfig{Adapter: adapter, TokenEnv: tokenEnv}}
	return s
}

func configSubtitle(id, adapter string) *corev1.Slot {
	s := &corev1.Slot{Id: id, Kind: corev1.SlotKind_SLOT_KIND_SUBTITLE_SOURCE}
	s.Config = &corev1.Slot_SubtitleSource{SubtitleSource: &corev1.SubtitleSourceConfig{Adapter: adapter}}
	return s
}

func configDrm(id string) *corev1.Slot {
	s := &corev1.Slot{Id: id, Kind: corev1.SlotKind_SLOT_KIND_DRM}
	s.Config = &corev1.Slot_Drm{Drm: &corev1.DrmConfig{}}
	return s
}
