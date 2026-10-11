package slotwiring

import (
	"context"
	"testing"
)

// stubProber is a CredentialProber that records it was called; the registry
// tests only care that the right prober comes back, never what it does.
type stubProber struct{}

func (stubProber) Probe(context.Context, string, string, []byte) ([]byte, error) {
	return nil, nil
}

// The registry is the seam an adapter uses to declare how it validates a
// linked account's credentials. Adding an adapter must never mean editing a
// shared switch, so registration is per-adapter and lookup is a map read.
func TestProberRegistry(t *testing.T) {
	const adapter = "prober-registry-test-adapter"
	RegisterProber(adapter, stubProber{})

	if got := ProberForAdapter(adapter); got == nil {
		t.Fatal("registered prober came back nil")
	}

	// An adapter that registers nothing has no prober: linking it is
	// Unavailable by contract (PLAN.md §3.5), and the lookup must say so
	// rather than fall through to another adapter's prober.
	if got := ProberForAdapter("prober-registry-test-absent"); got != nil {
		t.Fatalf("unregistered adapter prober = %v, want nil", got)
	}
}

// A nil prober is a wiring bug, not a state to carry: an adapter that
// registers nil would otherwise silently make its accounts unlinkable.
func TestRegisterProberRejectsNil(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("RegisterProber(nil) did not panic")
		}
	}()
	RegisterProber("prober-registry-test-nil", nil)
}

// Double registration is a wiring bug too — it means two files claim the
// same adapter, and the winner would be decided by init order.
func TestRegisterProberRejectsDuplicate(t *testing.T) {
	const adapter = "prober-registry-test-dup"
	RegisterProber(adapter, stubProber{})
	defer func() {
		if recover() == nil {
			t.Fatal("RegisterProber(duplicate) did not panic")
		}
	}()
	RegisterProber(adapter, stubProber{})
}

// Every shipped adapter that validates credentials itself must be registered:
// this catches a prober implemented in an adapter but never wired into the
// registry, which would leave its accounts unlinkable with no compile error.
func TestShippedAdaptersAreRegistered(t *testing.T) {
	for _, adapter := range []string{"jellyfin", "stremio"} {
		if ProberForAdapter(adapter) == nil {
			t.Errorf("adapter %q has no registered prober", adapter)
		}
	}
}
