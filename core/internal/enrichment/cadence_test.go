package enrichment

import (
	"testing"
	"time"
)

func TestDrainCadenceDefaultsWhenUnconfigured(t *testing.T) {
	d, err := DrainCadence("")
	if err != nil {
		t.Fatalf("empty config: %v", err)
	}
	if d != 15*time.Minute {
		t.Fatalf("default cadence = %v, want 15m", d)
	}
}

func TestDrainCadenceConfigOverridesDefault(t *testing.T) {
	d, err := DrainCadence("90s")
	if err != nil {
		t.Fatalf("valid override: %v", err)
	}
	if d != 90*time.Second {
		t.Fatalf("override cadence = %v, want 90s", d)
	}
}

func TestDrainCadenceRejectsBrokenValues(t *testing.T) {
	for _, bad := range []string{"nope", "0s", "-5m", "10"} {
		if _, err := DrainCadence(bad); err == nil {
			t.Fatalf("value %q accepted, want error", bad)
		}
	}
}
