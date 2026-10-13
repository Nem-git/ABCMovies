package app

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

// minimalConfigPath writes the smallest instance config: an ephemeral API
// bind and an in-memory slot store, so a stack test never reads or writes a
// default on-disk store shared with another test.
func minimalConfigPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(`
core:
  api:
    bind: "127.0.0.1:0"
stores:
  slots:
    backend: in-memory
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// seedJellyfinSlots writes the standard operator provider slot plus the
// built-in device sink into the stack's slot store and applies them live.
// It replaces the retired config `slots:` block (D8): slots are managed in
// the store, so a test seeds them the same way an operator's CreateSlot
// does, through the same load-and-apply path a boot uses.
func seedJellyfinSlots(t *testing.T, s *Stack, providerID, serverURL string) {
	t.Helper()
	putSlot(t, s, operatorJellyfinSlot(providerID, serverURL))
	putSlot(t, s, deviceSinkSlot("device"))
	applyStoredSlots(t, s)
}

// operatorJellyfinSlot is the standard operator provider slot the retired
// config block declared: one operator account reading its password from
// JF_TEST_PASSWORD.
func operatorJellyfinSlot(providerID, serverURL string) *corev1.Slot {
	return &corev1.Slot{
		Id:   providerID,
		Kind: corev1.SlotKind_SLOT_KIND_PROVIDER,

		Config: &corev1.Slot_Provider{Provider: &corev1.ProviderConfig{
			Adapter: "jellyfin",
			Server:  serverURL,
			Accounts: []*corev1.AccountConfig{{
				Id:          "op",
				Username:    "alice.homeserver.user",
				PasswordEnv: "JF_TEST_PASSWORD",
			}},
		}},
	}
}

// deviceSinkSlot is the standard built-in device sink slot.
func deviceSinkSlot(sinkID string) *corev1.Slot {
	return &corev1.Slot{
		Id:   sinkID,
		Kind: corev1.SlotKind_SLOT_KIND_SINK,

		Config: &corev1.Slot_Sink{Sink: &corev1.SinkConfig{Adapter: "device"}},
	}
}

// putSlot stores one slot record. Absence is normal; a put is not.
func putSlot(t *testing.T, s *Stack, slot *corev1.Slot) {
	t.Helper()
	if err := s.slotStore.Put(context.Background(), slot); err != nil {
		t.Fatalf("put slot %q: %v", slot.GetId(), err)
	}
}

// applyStoredSlots re-reads the slot store and applies it to the running
// stack — the live-reload path an admin CRUD drives.
func applyStoredSlots(t *testing.T, s *Stack) {
	t.Helper()
	slots, err := s.loadSlots(context.Background())
	if err != nil {
		t.Fatalf("load slots: %v", err)
	}
	if err := s.reloadSlots(context.Background(), slots, slog.Default()); err != nil {
		t.Fatalf("apply slots live: %v", err)
	}
}
