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
