package apiserver

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const userIDKey contextKey = "user_id"

// UserIDFromContext extracts the authenticated user ID from the context.
func UserIDFromContext(ctx context.Context) (string, bool) {
	uid, ok := ctx.Value(userIDKey).(string)
	return uid, ok
}

// AuthUnaryInterceptor returns a unary server interceptor that extracts a
// bearer token from gRPC metadata and injects the user ID into the context.
// For M0 the token is not validated; any Bearer token sets user "user:1".
func AuthUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		uid, err := authenticate(ctx)
		if err != nil {
			return nil, err
		}
		return handler(context.WithValue(ctx, userIDKey, uid), req)
	}
}

// AuthStreamInterceptor returns a stream server interceptor that extracts a
// bearer token from gRPC metadata and injects the user ID into the stream
// context.
func AuthStreamInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		uid, err := authenticate(ss.Context())
		if err != nil {
			return err
		}
		wrapped := &authStream{ServerStream: ss, ctx: context.WithValue(ss.Context(), userIDKey, uid)}
		return handler(srv, wrapped)
	}
}

func authenticate(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}
	values := md.Get("authorization")
	if len(values) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization")
	}
	token := values[0]
	const prefix = "Bearer "
	if !strings.HasPrefix(token, prefix) {
		return "", status.Error(codes.Unauthenticated, "invalid token format")
	}
	_ = token[len(prefix):] // M0: token not validated
	return "user:1", nil
}

type authStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authStream) Context() context.Context { return s.ctx }
