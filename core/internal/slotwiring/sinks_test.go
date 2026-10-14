package slotwiring

import (
	"strings"
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/delivery"
)

// TestSetupSinksDiskRequiresOptionsPath pins that a disk sink must name its
// root under options.path — the shared SlotEntry carries no adapter-specific
// knobs (PLAN.md §6.4), so a missing path is a loud error, never a silent
// default.
func TestSetupSinksDiskRequiresOptionsPath(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		opts map[string]string
	}{
		{name: "missing-options", opts: nil},
		{name: "empty-path", opts: map[string]string{"path": ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := SetupSinks([]*corev1.Slot{{
				Id: "disk", Kind: corev1.SlotKind_SLOT_KIND_SINK,
				Config: &corev1.Slot_Sink{Sink: &corev1.SinkConfig{Adapter: "disk", Options: tc.opts}},
			}}, delivery.NewRelay(), nil)
			if err == nil || !strings.Contains(err.Error(), "options.path is required") {
				t.Fatalf("want options.path error, got %v", err)
			}
		})
	}
}

// TestSetupSinksUnknownAdapterRejected pins loud failure for a typo'd sink
// adapter name.
func TestSetupSinksUnknownAdapterRejected(t *testing.T) {
	t.Parallel()
	_, err := SetupSinks([]*corev1.Slot{{
		Id: "x", Kind: corev1.SlotKind_SLOT_KIND_SINK,
		Config: &corev1.Slot_Sink{Sink: &corev1.SinkConfig{Adapter: "vhs-player"}},
	}}, delivery.NewRelay(), nil)
	if err == nil || !strings.Contains(err.Error(), "unknown adapter") {
		t.Fatalf("want unknown-adapter error, got %v", err)
	}
}

// TestSetupSinksMediaServerRegistered is the M8 pluggability proof: the
// media-server sink (a second sink type) is registered by its own wiring
// shim's init(), and SetupSinks builds it through the generic registry — no
// core dispatch change names it (PLAN.md §6.4: sinks are added without
// touching the core).
func TestSetupSinksMediaServerRegistered(t *testing.T) {
	t.Parallel()
	sinks, err := SetupSinks([]*corev1.Slot{{
		Id:   "mediaserver",
		Kind: corev1.SlotKind_SLOT_KIND_SINK,
		Config: &corev1.Slot_Sink{
			Sink: &corev1.SinkConfig{Adapter: "relay"},
		},
	}}, delivery.NewRelay(), delivery.NewOpener(nil, nil, nil))
	if err != nil {
		t.Fatalf("SetupSinks: %v", err)
	}
	if sinks == nil {
		t.Fatal("sinks is nil")
	}
}
