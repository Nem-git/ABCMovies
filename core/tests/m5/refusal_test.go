package m5_test

import (
	"testing"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestM5StartDeliveryTypedAtCapacityRefusal proves a refused start over the
// real wire carries the typed DeliveryRefusal detail: the harness policy caps
// a member at three concurrent streams, so a fourth start is refused with
// ResourceExhausted and an AT_CAPACITY reason a client reads without
// string-matching (PLAN.md §6.5, §11). The refusal carries no queue position.
func TestM5StartDeliveryTypedAtCapacityRefusal(t *testing.T) {
	stack := newM5Stack(t, fakeJellyfinServer(t))
	client := apiv1.NewCoreServiceClient(startWireServer(t, stack))
	aliceCtx := authedCtx(t.Context(), stack.aliceToken)

	start := func() error {
		_, err := client.StartDelivery(aliceCtx, &apiv1.StartDeliveryRequest{
			Goal:      apiv1.DeliveryGoal_DELIVERY_GOAL_PLAY,
			Provider:  stack.ns,
			AccountId: "lnk_alice_home",
			NativeId:  "movie-gondwana",
			Sink:      "relay",
		})
		return err
	}
	// The harness policy allows three concurrent streams; fill all three.
	for i := 0; i < 3; i++ {
		if err := start(); err != nil {
			t.Fatalf("start %d: %v", i+1, err)
		}
	}

	err := start()
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("4th start code = %v, want ResourceExhausted", status.Code(err))
	}
	details := status.Convert(err).Details()
	if len(details) != 1 {
		t.Fatalf("details = %v, want exactly one DeliveryRefusal", details)
	}
	refusal, ok := details[0].(*apiv1.DeliveryRefusal)
	if !ok {
		t.Fatalf("detail type = %T, want *apiv1.DeliveryRefusal", details[0])
	}
	if refusal.GetReason() != apiv1.DeliveryRefusalReason_DELIVERY_REFUSAL_REASON_AT_CAPACITY {
		t.Errorf("reason = %v, want at-capacity", refusal.GetReason())
	}
	if refusal.GetMessage() == "" {
		t.Error("refusal carries no message; the caller has nothing to act on")
	}
}
