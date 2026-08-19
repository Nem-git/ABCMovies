package apiserver

import (
	"context"
	"fmt"
	"sync/atomic"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/schema"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements the CoreService (PLAN.md §8).
type Server struct {
	apiv1.UnimplementedCoreServiceServer
	bus *Bus
	seq atomic.Int64
}

// NewServer returns a CoreService backed by the given bus.
func NewServer(bus *Bus) *Server {
	return &Server{bus: bus}
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
