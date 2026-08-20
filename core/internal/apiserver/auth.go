package apiserver

import (
	"context"
	"strings"

	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const (
	userIDKey contextKey = "user_id"
	dekKey    contextKey = "user_dek"
)

// UserIDFromContext extracts the authenticated user ID from the context.
func UserIDFromContext(ctx context.Context) (string, bool) {
	uid, ok := ctx.Value(userIDKey).(string)
	return uid, ok
}

// DEKFromContext extracts the user's DEK from the context. The DEK is set
// by the auth interceptor after login and enables per-user blob encryption.
// Returns nil if no DEK is available (e.g. the session was not created via
// login).
func DEKFromContext(ctx context.Context) []byte {
	dek, _ := ctx.Value(dekKey).([]byte)
	return dek
}

// AuthUnaryInterceptor returns a unary server interceptor that extracts a
// bearer token from gRPC metadata and injects the user ID into the context.
// The token is validated against the session store.
func AuthUnaryInterceptor(session auth.Session) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		uid, err := authenticate(ctx, session)
		if err != nil {
			return nil, err
		}
		ctx = context.WithValue(ctx, userIDKey, uid)
		if dek := session.GetDEK(uid); dek != nil {
			ctx = context.WithValue(ctx, dekKey, dek)
			ctx = context.WithValue(ctx, store.UserBlobUserIDKey, uid)
			ctx = context.WithValue(ctx, store.UserBlobDEKKey, dek)
		}
		return handler(ctx, req)
	}
}

// AuthStreamInterceptor returns a stream server interceptor that extracts a
// bearer token from gRPC metadata and injects the user ID into the stream
// context.
func AuthStreamInterceptor(session auth.Session) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		uid, err := authenticate(ss.Context(), session)
		if err != nil {
			return err
		}
		ctx := context.WithValue(ss.Context(), userIDKey, uid)
		if dek := session.GetDEK(uid); dek != nil {
			ctx = context.WithValue(ctx, dekKey, dek)
			ctx = context.WithValue(ctx, store.UserBlobUserIDKey, uid)
			ctx = context.WithValue(ctx, store.UserBlobDEKKey, dek)
		}
		wrapped := &authStream{ServerStream: ss, ctx: ctx}
		return handler(srv, wrapped)
	}
}

func authenticate(ctx context.Context, session auth.Session) (string, error) {
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
	rawToken := token[len(prefix):]

	uid, err := session.Validate(rawToken)
	if err != nil {
		return "", status.Error(codes.Unauthenticated, "invalid or expired token")
	}
	return uid, nil
}

type authStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authStream) Context() context.Context { return s.ctx }
