package app

import (
	"context"
	"testing"
	"time"

	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/slothealth"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// The composed transition wiring delivers exactly one event per health
// crossing to operator subscribers only: a regular subscriber sees nothing,
// and repeated probes in the same state add nothing.
func TestSlotHealthTransitionsReachOperatorsOnly(t *testing.T) {
	users := auth.NewMemoryUserStore()
	if err := users.PutUser("op", &auth.UserData{Role: auth.RoleOperator}); err != nil {
		t.Fatal(err)
	}
	if err := users.PutUser("alice", &auth.UserData{Role: auth.RoleUser}); err != nil {
		t.Fatal(err)
	}

	slotHP, err := slothealth.New(store.NewInMemory())
	if err != nil {
		t.Fatal(err)
	}
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	bus.SetOperatorEntiter(operatorPredicate(users))
	wireSlotHealthTransitions(bus, slotHP)

	opCh := bus.Subscribe("sub-op", "user:op")
	userCh := bus.Subscribe("sub-alice", "user:alice")
	defer bus.Unsubscribe("sub-op")
	defer bus.Unsubscribe("sub-alice")

	ctx := context.Background()
	if err := slotHP.RecordProbeSuccess(ctx, "jellyfin-1"); err != nil {
		t.Fatal(err)
	}
	if err := slotHP.RecordProbeFailure(ctx, "jellyfin-1", "NETWORK", "down"); err != nil {
		t.Fatal(err)
	}
	// Same state again: no second event.
	if err := slotHP.RecordProbeFailure(ctx, "jellyfin-1", "TIMEOUT", "still down"); err != nil {
		t.Fatal(err)
	}

	select {
	case e := <-opCh:
		if e.GetSlotHealthTransition().GetSlotId() != "jellyfin-1" {
			t.Fatalf("slot id = %q, want jellyfin-1", e.GetSlotHealthTransition().GetSlotId())
		}
		if e.GetSlotHealthTransition().GetHealthy() || !e.GetSlotHealthTransition().GetWasHealthy() {
			t.Fatalf("transition = %+v, want unhealthy flip", e.GetSlotHealthTransition())
		}
		if e.GetSlotHealthTransition().GetLastErrorKind() != "NETWORK" {
			t.Fatalf("error kind = %q, want NETWORK", e.GetSlotHealthTransition().GetLastErrorKind())
		}
	case <-time.After(time.Second):
		t.Fatal("operator should receive the transition")
	}

	select {
	case e := <-opCh:
		t.Fatalf("operator received a second event for the same state: %v", e.GetType())
	default:
	}

	select {
	case e := <-userCh:
		t.Fatalf("regular subscriber received a transition: %v", e.GetType())
	default:
	}
}
