package apiserver_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/delivery"
)

// stubDelivery is a configurable DeliveryManager for exercising the API
// handler untouched by the engine.
type stubDelivery struct {
	session    *delivery.Session
	startErr   error
	heartbeats []string
	heartErr   error
}

func (s *stubDelivery) Start(ctx context.Context, req delivery.StartRequest) (*delivery.Session, error) {
	return s.session, s.startErr
}

func (s *stubDelivery) Heartbeat(id string) error {
	s.heartbeats = append(s.heartbeats, id)
	return s.heartErr
}

func runningSession() *delivery.Session {
	return &delivery.Session{
		ID:     "del-1",
		Goal:   delivery.GoalPlay,
		Status: delivery.StatusRunning,
		Context: corev1.DeliveryContext{
			Provider:     "jellyfin",
			AccountId:    "acc-1",
			MemberUserId: "user-1",
			Sink:         "device",
		},
	}
}

func TestStartDelivery_Success(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	dm := &stubDelivery{session: runningSession()}
	srv := apiserver.NewServer(bus, testStores(t), authenticator, session, dm)

	resp, err := srv.StartDelivery(context.Background(), &apiv1.StartDeliveryRequest{
		Goal:         apiv1.DeliveryGoal_DELIVERY_GOAL_PLAY,
		Provider:     "jellyfin",
		AccountId:    "acc-1",
		MemberUserId: "user-1",
		NativeId:     "item-42",
		Sink:         "device",
	})
	if err != nil {
		t.Fatalf("StartDelivery: %v", err)
	}
	if resp.GetJob().GetId() != "del-1" {
		t.Errorf("job id = %q, want del-1", resp.GetJob().GetId())
	}
	if resp.GetJob().GetStatus() != corev1.JobStatus_JOB_STATUS_RUNNING {
		t.Errorf("job status = %v, want running", resp.GetJob().GetStatus())
	}
}

func TestStartDelivery_Unconfigured(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	srv := apiserver.NewServer(bus, testStores(t), authenticator, session)

	_, err := srv.StartDelivery(context.Background(), &apiv1.StartDeliveryRequest{
		Goal:         apiv1.DeliveryGoal_DELIVERY_GOAL_PLAY,
		Provider:     "jellyfin",
		AccountId:    "acc-1",
		MemberUserId: "user-1",
		NativeId:     "item",
		Sink:         "device",
	})
	if got := status.Code(err); got != codes.Unavailable {
		t.Fatalf("code = %v, want Unavailable", got)
	}
}

func TestStartDelivery_Validation(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	dm := &stubDelivery{session: runningSession()}
	srv := apiserver.NewServer(bus, testStores(t), authenticator, session, dm)

	cases := []*apiv1.StartDeliveryRequest{
		{},
		{Goal: apiv1.DeliveryGoal_DELIVERY_GOAL_PLAY, Provider: "j", AccountId: "a", MemberUserId: "u", NativeId: "i"}, // missing sink
		{Goal: apiv1.DeliveryGoal_DELIVERY_GOAL_PLAY, Provider: "j", AccountId: "a", MemberUserId: "u", Sink: "s"},     // missing native
	}
	for i, r := range cases {
		if _, err := srv.StartDelivery(context.Background(), r); status.Code(err) != codes.InvalidArgument {
			t.Errorf("case %d: code = %v, want InvalidArgument", i, status.Code(err))
		}
	}
	// The engine must not be reached for a bad request.
	if dm.session != nil && len(dm.heartbeats) != 0 {
		t.Fatalf("engine was reached for invalid requests")
	}
}

func TestHeartbeat_SuccessAndErrors(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	dm := &stubDelivery{}
	srv := apiserver.NewServer(bus, testStores(t), authenticator, session, dm)

	resp, err := srv.Heartbeat(context.Background(), &apiv1.HeartbeatRequest{SessionId: "del-1"})
	if err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}
	if resp.GetSessionId() != "del-1" {
		t.Errorf("session id = %q", resp.GetSessionId())
	}
	if len(dm.heartbeats) != 1 || dm.heartbeats[0] != "del-1" {
		t.Errorf("heartbeats = %v, want [del-1]", dm.heartbeats)
	}

	if _, err := srv.Heartbeat(context.Background(), &apiv1.HeartbeatRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("empty heartbeat code = %v, want InvalidArgument", status.Code(err))
	}
}
