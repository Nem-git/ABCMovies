package apiserver

import (
	"context"
	"fmt"
	"sync/atomic"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/delivery"
	"github.com/nem-git/abcmovies/core/internal/schema"
	"github.com/nem-git/abcmovies/core/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// DeliveryManager is the delivery-engine surface the API layer calls
// (PLAN.md §6, §9.1). Exposing an interface keeps the apiserver decoupled
// from the engine's internals and lets the handlers be tested with a stub.
type DeliveryManager interface {
	Start(ctx context.Context, req delivery.StartRequest) (*delivery.Session, error)
	Heartbeat(id string) error
	PlayMenu(sessionID string) (*PlayMenu, error)
}

// Server implements the CoreService (PLAN.md §8).
type Server struct {
	apiv1.UnimplementedCoreServiceServer
	bus      Bus
	stores   config.Stores
	auth     *auth.CompositeAuthenticator
	session  auth.Session
	delivery DeliveryManager
	seq      atomic.Int64

	// library gates every read of the merged catalog and the delivery
	// authorization (PLAN.md §5.1); nil until armed, when absent the library
	// RPCs return Unavailable. accounts persists linked-account records and
	// their vaulted sessions; it is always available over the vault. probers
	// validate candidate linked-account credentials per provider (PLAN.md
	// §3.5: nothing is vaulted that was not probed); armed by wiring.
	library  LibrarySeam
	accounts *accounts.Store
	probers  map[string]CredentialProber
}

// NewServer returns a CoreService backed by the given bus, stores, and auth.
// An optional DeliveryManager may be supplied; when absent the delivery RPCs
// return Unavailable.
func NewServer(bus Bus, stores config.Stores, authenticator *auth.CompositeAuthenticator, session auth.Session, dm ...DeliveryManager) *Server {
	var d DeliveryManager
	if len(dm) > 0 {
		d = dm[0]
	}
	return &Server{
		bus:      bus,
		stores:   stores,
		auth:     authenticator,
		session:  session,
		delivery: d,
		// The accounts store is owned here, over the same vault the wiring's
		// session-vault recovery reads, so a link made through the API is
		// picked up by slot provisioning exactly as an operator-configured
		// account would be (PLAN.md §3.5).
		accounts: accounts.NewStore(stores.Vault, nil),
		probers:  map[string]CredentialProber{},
	}
}

// SetDelivery arms the delivery engine after construction — used when the
// engine is composed lazily (after slots are wired) rather than at NewServer
// time. Until SetDelivery is called the delivery RPCs return Unavailable.
func (s *Server) SetDelivery(dm DeliveryManager) {
	s.delivery = dm
}

// Delivery returns the currently armed delivery engine, or nil.
func (s *Server) Delivery() DeliveryManager { return s.delivery }

// GetJob returns a job's current state from the jobs store (PLAN.md §9.1).
func (s *Server) GetJob(ctx context.Context, req *apiv1.GetJobRequest) (*apiv1.GetJobResponse, error) {
	if err := schema.ValidateGetJobRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	raw, err := s.stores.Jobs.Get(ctx, "job:"+req.GetJobId())
	if err == store.ErrKeyNotFound {
		return nil, status.Error(codes.NotFound, "job not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to read job")
	}
	var job corev1.Job
	if err := proto.Unmarshal(raw, &job); err != nil {
		return nil, status.Error(codes.Internal, "corrupted job data")
	}
	return &apiv1.GetJobResponse{Job: &job}, nil
}

// StartDelivery begins a play or download session and returns its job
// (PLAN.md §6, §9.1). Start is a plain "create" — each call makes a new
// session (TECHNICAL-DECISIONS.md §1.30).
func (s *Server) StartDelivery(ctx context.Context, req *apiv1.StartDeliveryRequest) (*apiv1.StartDeliveryResponse, error) {
	if err := schema.ValidateStartDeliveryRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if s.delivery == nil {
		return nil, status.Error(codes.Unavailable, "delivery engine not configured")
	}
	goal, err := deliveryGoal(req.GetGoal())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	sess, err := s.delivery.Start(ctx, delivery.StartRequest{
		Goal:           goal,
		MemberUserID:   req.GetMemberUserId(),
		Provider:       req.GetProvider(),
		AccountID:      req.GetAccountId(),
		NativeID:       req.GetNativeId(),
		Sink:           req.GetSink(),
		SelectedTarget: req.GetSelectedTarget(),
		Container:      req.GetContainer(),
	})
	if err != nil {
		return nil, status.Error(codes.Code(delivery.Code(err)), err.Error())
	}
	s.persistDeliveryJob(ctx, sess.Job())
	return &apiv1.StartDeliveryResponse{Job: sess.Job()}, nil
}

// Heartbeat keeps a play session alive (PLAN.md §9.1).
func (s *Server) Heartbeat(ctx context.Context, req *apiv1.HeartbeatRequest) (*apiv1.HeartbeatResponse, error) {
	if err := schema.ValidateHeartbeatRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if s.delivery == nil {
		return nil, status.Error(codes.Unavailable, "delivery engine not configured")
	}
	if err := s.delivery.Heartbeat(req.GetSessionId()); err != nil {
		return nil, status.Error(codes.Code(delivery.Code(err)), err.Error())
	}
	return &apiv1.HeartbeatResponse{SessionId: req.GetSessionId()}, nil
}

// deliveryGoal maps the API goal enum to the engine's goal.
func deliveryGoal(g apiv1.DeliveryGoal) (delivery.Goal, error) {
	switch g {
	case apiv1.DeliveryGoal_DELIVERY_GOAL_PLAY:
		return delivery.GoalPlay, nil
	case apiv1.DeliveryGoal_DELIVERY_GOAL_DOWNLOAD:
		return delivery.GoalDownload, nil
	default:
		return "", status.Error(codes.InvalidArgument, "unknown delivery goal")
	}
}

// persistDeliveryJob writes the job and announces its status event, mirroring
// CreateJob so GetJob and Subscribe stay current (PLAN.md §9.1, §9.2).
func (s *Server) persistDeliveryJob(ctx context.Context, job *corev1.Job) {
	if job == nil {
		return
	}
	raw, err := proto.Marshal(job)
	if err != nil {
		return
	}
	_ = s.stores.Jobs.Put(ctx, "job:"+job.GetId(), raw)
	s.bus.Publish(&corev1.EventEnvelope{
		Id:       fmt.Sprintf("evt-delivery-%s", job.GetId()),
		Type:     corev1.EventType_EVENT_TYPE_JOB_STATUS,
		Audience: corev1.EventAudience_EVENT_AUDIENCE_USER,
		UserId:   job.GetOwnerUserId(),
		Payload: &corev1.EventEnvelope_JobStatus{
			JobStatus: &corev1.JobStatusEvent{
				JobId:  job.GetId(),
				Status: job.GetStatus(),
			},
		},
		EmittedAt: timestamppb.Now(),
	})
}

// CreateJob persists a job in the jobs store. This is an internal method for
// M0 — it proves the jobs storage class works end-to-end. A public CreateJob
// RPC is not yet exposed (the proto does not define one).
func (s *Server) CreateJob(ctx context.Context, job *corev1.Job) error {
	if err := schema.ValidateJob(job); err != nil {
		return err
	}
	raw, err := proto.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	if err := s.stores.Jobs.Put(ctx, "job:"+job.GetId(), raw); err != nil {
		return err
	}
	// Publish job-status event (PLAN.md §9.2, M0 acceptance).
	s.bus.Publish(&corev1.EventEnvelope{
		Id:       fmt.Sprintf("evt-job-%s", job.GetId()),
		Type:     corev1.EventType_EVENT_TYPE_JOB_STATUS,
		Audience: corev1.EventAudience_EVENT_AUDIENCE_USER,
		UserId:   job.GetOwnerUserId(),
		Payload: &corev1.EventEnvelope_JobStatus{
			JobStatus: &corev1.JobStatusEvent{
				JobId:  job.GetId(),
				Status: job.GetStatus(),
			},
		},
		EmittedAt: timestamppb.Now(),
	})
	return nil
}

// Subscribe streams events to the client. The stream stays open until the
// client disconnects.
func (s *Server) Subscribe(req *apiv1.SubscribeRequest, stream apiv1.CoreService_SubscribeServer) error {
	if err := schema.ValidateSubscribeRequest(req); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	uid, _ := UserIDFromContext(stream.Context())
	id := fmt.Sprintf("sub-%d", s.seq.Add(1))
	ch := s.bus.Subscribe(id, uid)
	defer s.bus.Unsubscribe(id)
	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(&apiv1.SubscribeResponse{Event: event}); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

// SignUp creates a new user account. The auth method is determined by the
// oneof field in the request.
func (s *Server) SignUp(ctx context.Context, req *apiv1.SignUpRequest) (*apiv1.SignUpResponse, error) {
	if err := schema.ValidateSignUpRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	method := authMethod(req.GetPassword(), nil)
	a, ok := s.auth.Get(method)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported auth method: %s", method)
	}
	result, err := a.SignUp(req.GetUsername(), req.GetPassword().GetPassword())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &apiv1.SignUpResponse{
		UserId:      result.UserID,
		RecoveryKey: result.RecoveryKey,
	}, nil
}

// Login authenticates a user and returns a session token. The auth method
// is determined by the oneof field in the request.
func (s *Server) Login(ctx context.Context, req *apiv1.LoginRequest) (*apiv1.LoginResponse, error) {
	if err := schema.ValidateLoginRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	method := authMethod(nil, req.GetPassword())
	a, ok := s.auth.Get(method)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported auth method: %s", method)
	}
	result, err := a.Login(req.GetUsername(), req.GetPassword().GetPassword())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}
	token, err := s.session.Mint(result.UserID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create session")
	}
	// Cache the DEK for this session: per-user blob stores decrypt with it
	// until the session ends. The entry is keyed by the token and evicted
	// with it (IMPLEMENTATION.md §1.3).
	if err := s.session.StoreDEK(token, result.DEK); err != nil {
		return nil, status.Error(codes.Internal, "failed to cache session key material")
	}
	return &apiv1.LoginResponse{
		Token: token,
	}, nil
}

// authMethod returns the method name from the non-nil oneof field.
func authMethod(passwordSignUp *apiv1.PasswordSignUp, passwordLogin *apiv1.PasswordLogin) string {
	if passwordSignUp != nil {
		return "password"
	}
	if passwordLogin != nil {
		return "password"
	}
	return ""
}
