// Package web is the reference web frontend's serving layer: it composes
// the core in-process and terminates gRPC-Web (plus plain gRPC and the
// Connect protocol) for browsers on one port, per TECHNICAL-DECISIONS.md
// §1.2.
package serving

import (
	"context"
	"errors"
	"io"
	"net/http"

	"connectrpc.com/connect"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	apiv1connect "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1/apiv1connect"
)

// mergeMetadata copies gRPC metadata entries into an HTTP header map.
func mergeMetadata(dst http.Header, md metadata.MD) {
	for k, vs := range md {
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}

// translate converts the core's grpc status errors into connect errors so
// they cross the gRPC-Web/Connect wire with their code intact (connect
// handlers do not translate automatically). The two code spaces share one
// numbering by design.
func translate(err error) error {
	if err == nil {
		return nil
	}
	if cerr, ok := errors.AsType[*connect.Error](err); ok {
		return cerr
	}
	s, ok := status.FromError(err)
	if !ok {
		s = status.New(codes.Unknown, err.Error())
	}
	return connect.NewError(connect.Code(int32(s.Code())), err)
}

// coreServiceAdapter adapts the core's service implementation to the
// connect handler interface. It is pure delegation: no logic lives here.
type coreServiceAdapter struct {
	srv apiv1.CoreServiceServer
}

var _ apiv1connect.CoreServiceHandler = (*coreServiceAdapter)(nil)

func (a *coreServiceAdapter) GetJob(
	ctx context.Context,
	req *connect.Request[apiv1.GetJobRequest],
) (*connect.Response[apiv1.GetJobResponse], error) {
	resp, err := a.srv.GetJob(ctx, req.Msg)
	if err != nil {
		return nil, translate(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *coreServiceAdapter) SignUp(
	ctx context.Context,
	req *connect.Request[apiv1.SignUpRequest],
) (*connect.Response[apiv1.SignUpResponse], error) {
	resp, err := a.srv.SignUp(ctx, req.Msg)
	if err != nil {
		return nil, translate(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *coreServiceAdapter) Login(
	ctx context.Context,
	req *connect.Request[apiv1.LoginRequest],
) (*connect.Response[apiv1.LoginResponse], error) {
	resp, err := a.srv.Login(ctx, req.Msg)
	if err != nil {
		return nil, translate(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *coreServiceAdapter) StartDelivery(
	ctx context.Context,
	req *connect.Request[apiv1.StartDeliveryRequest],
) (*connect.Response[apiv1.StartDeliveryResponse], error) {
	resp, err := a.srv.StartDelivery(ctx, req.Msg)
	if err != nil {
		return nil, translate(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *coreServiceAdapter) Heartbeat(
	ctx context.Context,
	req *connect.Request[apiv1.HeartbeatRequest],
) (*connect.Response[apiv1.HeartbeatResponse], error) {
	resp, err := a.srv.Heartbeat(ctx, req.Msg)
	if err != nil {
		return nil, translate(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *coreServiceAdapter) Subscribe(
	ctx context.Context,
	req *connect.Request[apiv1.SubscribeRequest],
	stream *connect.ServerStream[apiv1.SubscribeResponse],
) error {
	if err := a.srv.Subscribe(req.Msg, &subscribeStream{ctx: ctx, stream: stream}); err != nil {
		return translate(err)
	}
	return nil
}

// subscribeStream bridges connect's server stream to the grpc-go
// server-streaming interface the core's Subscribe implementation expects.
type subscribeStream struct {
	ctx    context.Context
	stream *connect.ServerStream[apiv1.SubscribeResponse]
}

func (s *subscribeStream) Context() context.Context { return s.ctx }

func (s *subscribeStream) Send(msg *apiv1.SubscribeResponse) error {
	return s.stream.Send(msg)
}

func (s *subscribeStream) SendMsg(msg any) error {
	return s.Send(msg.(*apiv1.SubscribeResponse))
}

func (s *subscribeStream) RecvMsg(any) error { return io.EOF }

func (s *subscribeStream) SetHeader(md metadata.MD) error {
	mergeMetadata(s.stream.ResponseHeader(), md)
	return nil
}

func (s *subscribeStream) SendHeader(md metadata.MD) error {
	mergeMetadata(s.stream.ResponseHeader(), md)
	return nil
}

func (s *subscribeStream) SetTrailer(md metadata.MD) {
	mergeMetadata(s.stream.ResponseTrailer(), md)
}
