package slotwiring

import (
	"strings"
	"testing"

	"github.com/nem-git/abcmovies/core/internal/config"
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
			_, err := SetupSinks([]config.SlotEntry{{
				ID: "disk", Adapter: "disk", Enabled: true, Options: tc.opts,
			}}, delivery.NewRelay())
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
	_, err := SetupSinks([]config.SlotEntry{{
		ID: "x", Adapter: "vhs-player", Enabled: true,
	}}, delivery.NewRelay())
	if err == nil || !strings.Contains(err.Error(), "unknown adapter") {
		t.Fatalf("want unknown-adapter error, got %v", err)
	}
}
