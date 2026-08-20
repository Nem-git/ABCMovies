package apiserver

import (
	"context"
	"fmt"
	"sync/atomic"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/schema"
	"github.com/nem-git/abcmovies/core/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// Server implements the CoreService (PLAN.md §8).
type Server struct {
	apiv1.UnimplementedCoreServiceServer
	bus     Bus
	stores  config.Stores
	auth    *auth.CompositeAuthenticator
	session auth.Session
	seq     atomic.Int64
}

// NewServer returns a CoreService backed by the given bus, stores, and auth.
func NewServer(bus Bus, stores config.Stores, authenticator *auth.CompositeAuthenticator, session auth.Session) *Server {
	return &Server{bus: bus, stores: stores, auth: authenticator, session: session}
}

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
	return s.stores.Jobs.Put(ctx, "job:"+job.GetId(), raw)
}

// Subscribe streams events to the client. The stream stays open until the
// client disconnects.
func (s *Server) Subscribe(req *apiv1.SubscribeRequest, stream apiv1.CoreService_SubscribeServer) error {
	if err := schema.ValidateSubscribeRequest(req); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	id := fmt.Sprintf("sub-%d", s.seq.Add(1))
	ch := s.bus.Subscribe(id)
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
	// Cache the DEK for per-user blob encryption.
	s.session.StoreDEK(result.UserID, result.DEK)
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
