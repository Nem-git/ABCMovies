package apiserver_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
)

// GetInstanceInfo publishes the non-sensitive instance parameters: the
// contract version, the enabled auth methods, and the liveness cadence. A
// server the composition root never armed still publishes the shipped
// default; an armed server publishes the configured value — the same value
// the delivery engine enforces (TECHNICAL-DECISIONS.md §1.43).
func TestGetInstanceInfo(t *testing.T) {
	authenticator, session := testAuth(t)
	srv := apiserver.NewServer(apiserver.NewInMemoryBus(), testStores(t), authenticator, session)

	resp, err := srv.GetInstanceInfo(context.Background(), &apiv1.GetInstanceInfoRequest{})
	if err != nil {
		t.Fatalf("GetInstanceInfo: %v", err)
	}
	if resp.GetContractVersion() != "v1" {
		t.Errorf("contract version = %q, want v1", resp.GetContractVersion())
	}
	if got := resp.GetAuthMethods(); len(got) != 1 || got[0] != "password" {
		t.Errorf("auth methods = %v, want [password]", got)
	}
	if got := resp.GetHeartbeatInterval().AsDuration(); got != 30*time.Second {
		t.Errorf("heartbeat interval = %v, want shipped default 30s", got)
	}

	srv.SetHeartbeatInterval(45 * time.Second)
	resp, err = srv.GetInstanceInfo(context.Background(), &apiv1.GetInstanceInfoRequest{})
	if err != nil {
		t.Fatalf("GetInstanceInfo armed: %v", err)
	}
	if got := resp.GetHeartbeatInterval().AsDuration(); got != 45*time.Second {
		t.Errorf("armed heartbeat interval = %v, want 45s", got)
	}
}

// GetInstanceInfo sits on the public allowlist: a client must learn the auth
// methods and the liveness cadence before it owns a token — but a
// non-public method without a token is still rejected.
func TestGetInstanceInfoIsPublic(t *testing.T) {
	_, session := testAuth(t)
	interceptor := apiserver.AuthUnaryInterceptor(session)
	handler := func(context.Context, any) (any, error) { return "ok", nil }
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs())

	if _, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{
		FullMethod: "/abcmovies.api.v1.CoreService/GetInstanceInfo",
	}, handler); err != nil {
		t.Fatalf("GetInstanceInfo without token: %v", err)
	}
	if _, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{
		FullMethod: "/abcmovies.api.v1.CoreService/GetJob",
	}, handler); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("GetJob without token = %v, want Unauthenticated", err)
	}
}
