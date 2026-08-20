package apiserver

import (
	"context"
	"fmt"
	"sync/atomic"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/schema"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements the CoreService (PLAN.md §8).
type Server struct {
	apiv1.UnimplementedCoreServiceServer
	bus     *Bus
	stores  config.Stores
	auth    auth.Authenticator
	session *auth.Session
	seq     atomic.Int64
}

// NewServer returns a CoreService backed by the given bus, stores, and auth.
func NewServer(bus *Bus, stores config.Stores, authenticator auth.Authenticator, session *auth.Session) *Server {
	return &Server{bus: bus, stores: stores, auth: authenticator, session: session}
}

// GetJob returns a job's current state. For M0, no jobs exist yet — the call
// always returns NotFound, proving the full vertical slice works.
func (s *Server) GetJob(ctx context.Context, req *apiv1.GetJobRequest) (*apiv1.GetJobResponse, error) {
	if err := schema.ValidateGetJobRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	_ = ctx // M0: no job store yet
	return nil, status.Error(codes.NotFound, "job not found")
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

// SignUp creates a new user account.
func (s *Server) SignUp(ctx context.Context, req *apiv1.SignUpRequest) (*apiv1.SignUpResponse, error) {
	if err := schema.ValidateSignUpRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	result, err := s.auth.SignUp(req.GetUsername(), req.GetPassword().GetPassword())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &apiv1.SignUpResponse{
		UserId:      result.UserID,
		RecoveryKey: result.RecoveryKey,
	}, nil
}

// Login authenticates a user and returns a session token.
func (s *Server) Login(ctx context.Context, req *apiv1.LoginRequest) (*apiv1.LoginResponse, error) {
	if err := schema.ValidateLoginRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	result, err := s.auth.Login(req.GetUsername(), req.GetPassword().GetPassword())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}
	token, err := s.session.Mint(result.UserID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create session")
	}
	return &apiv1.LoginResponse{
		Token: token,
	}, nil
}
