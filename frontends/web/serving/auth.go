package serving

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"connectrpc.com/connect"

	"github.com/nem-git/abcmovies/core/app"
	apiv1connect "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1/apiv1connect"
)

// publicMethods are the inbound-API methods reachable without a session
// token. It mirrors the gRPC interceptors' allowlist in the apiserver —
// the two lists must stay in sync, since both terminations of the same
// service enforce the same rule: account creation and login are how a
// caller obtains a token.
var publicMethods = map[string]bool{
	apiv1connect.CoreServiceSignUpProcedure: true,
	apiv1connect.CoreServiceLoginProcedure:  true,
}

// authInterceptor enforces bearer-token authentication for the connect
// termination of the core service, mirroring the gRPC interceptors in the
// apiserver. Errors are encoded per protocol (gRPC-Web clients receive a
// proper trailer-frame error).
type authInterceptor struct {
	session app.Session
}

func (i authInterceptor) auth(ctx context.Context, procedure string, header http.Header) (context.Context, error) {
	if publicMethods[procedure] {
		return ctx, nil
	}
	raw := header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(raw, prefix) {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing bearer token"))
	}
	uid, err := i.session.Validate(strings.TrimPrefix(raw, prefix))
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired token"))
	}
	return i.session.PrincipalContext(ctx, uid), nil
}

func (i authInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx, err := i.auth(ctx, req.Spec().Procedure, req.Header())
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i authInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i authInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := i.auth(ctx, conn.Spec().Procedure, conn.RequestHeader())
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}
