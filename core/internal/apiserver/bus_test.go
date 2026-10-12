package apiserver_test

import (
	"testing"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
)

// An ACCOUNT-audience event is delivered only to subscribers whose uid is
// entitled to that account; a non-member subscriber must not see it.
func TestInMemoryBusRoutesAccountAudienceByEntitlement(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	bus.SetAccountEntiter(func(accountID, uid string) bool {
		return accountID == "acct-alice" && uid == "alice"
	})
	aliceCh := bus.Subscribe("sub-alice", "alice")
	bobCh := bus.Subscribe("sub-bob", "bob")
	defer bus.Unsubscribe("sub-alice")
	defer bus.Unsubscribe("sub-bob")

	env := &corev1.EventEnvelope{
		Type:      corev1.EventType_EVENT_TYPE_AVAILABILITY_CHANGED,
		Audience:  corev1.EventAudience_EVENT_AUDIENCE_ACCOUNT,
		AccountId: "acct-alice",
	}
	bus.Publish(env)

	select {
	case e := <-aliceCh:
		if e.GetType() != corev1.EventType_EVENT_TYPE_AVAILABILITY_CHANGED {
			t.Fatalf("alice got %v, want availability event", e.GetType())
		}
	case <-time.After(time.Second):
		t.Fatal("alice should receive the account-scoped event")
	}

	select {
	case e := <-bobCh:
		t.Fatalf("bob (not entitled) received %v", e.GetType())
	case <-time.After(100 * time.Millisecond):
	}
}

// Without an entiter wired, account-audience events are dropped rather than
// sprayed to every subscriber.
func TestInMemoryBusDropsAccountEventsWithoutEntiter(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	aliceCh := bus.Subscribe("sub-alice", "alice")
	defer bus.Unsubscribe("sub-alice")

	env := &corev1.EventEnvelope{
		Type:      corev1.EventType_EVENT_TYPE_AVAILABILITY_CHANGED,
		Audience:  corev1.EventAudience_EVENT_AUDIENCE_ACCOUNT,
		AccountId: "acct-alice",
	}
	bus.Publish(env)

	select {
	case e := <-aliceCh:
		t.Fatalf("no entitler wired: alice got %v, want drop", e.GetType())
	case <-time.After(100 * time.Millisecond):
	}
}

// An INSTANCE-audience event (operational) is delivered only to subscribers
// the operator predicate admits; everyone else must not see it.
func TestInMemoryBusRoutesInstanceAudienceToOperators(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	bus.SetOperatorEntiter(func(uid string) bool { return uid == "op" })
	opCh := bus.Subscribe("sub-op", "op")
	userCh := bus.Subscribe("sub-user", "alice")
	defer bus.Unsubscribe("sub-op")
	defer bus.Unsubscribe("sub-user")

	bus.Publish(&corev1.EventEnvelope{
		Type:     corev1.EventType_EVENT_TYPE_SLOT_HEALTH_TRANSITION,
		Audience: corev1.EventAudience_EVENT_AUDIENCE_INSTANCE,
		Payload: &corev1.EventEnvelope_SlotHealthTransition{
			SlotHealthTransition: &corev1.SlotHealthTransitionEvent{SlotId: "jellyfin-1"},
		},
	})

	select {
	case e := <-opCh:
		if e.GetType() != corev1.EventType_EVENT_TYPE_SLOT_HEALTH_TRANSITION {
			t.Fatalf("operator got %v, want slot-health transition", e.GetType())
		}
	case <-time.After(time.Second):
		t.Fatal("operator should receive the instance-audience event")
	}

	select {
	case e := <-userCh:
		t.Fatalf("regular subscriber received an instance-audience event: %v", e.GetType())
	default:
	}
}

// Without the operator predicate the bus drops INSTANCE-audience events,
// fail-closed — the same rule the account audience follows.
func TestInMemoryBusInstanceAudienceFailClosed(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	subCh := bus.Subscribe("sub-op", "op")
	defer bus.Unsubscribe("sub-op")

	bus.Publish(&corev1.EventEnvelope{
		Type:     corev1.EventType_EVENT_TYPE_SLOT_HEALTH_TRANSITION,
		Audience: corev1.EventAudience_EVENT_AUDIENCE_INSTANCE,
		Payload: &corev1.EventEnvelope_SlotHealthTransition{
			SlotHealthTransition: &corev1.SlotHealthTransitionEvent{SlotId: "jellyfin-1"},
		},
	})

	select {
	case e := <-subCh:
		t.Fatalf("received instance-audience event without predicate: %v", e.GetType())
	default:
	}
}
