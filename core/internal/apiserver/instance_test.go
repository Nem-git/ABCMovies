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
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/settings"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// GetInstanceInfo publishes the non-sensitive instance parameters: the
// contract version, the enabled auth methods, and the liveness cadence. A
// server the composition root never armed still publishes the shipped
// default; an armed server publishes the configured value — the same value
// the delivery engine enforces (TECHNICAL-DECISIONS.md §1.43).
func TestGetInstanceInfo(t *testing.T) {
	authenticator, session := testAuth(t)
	srv := apiserver.NewServer(apiserver.Settings{Bus: apiserver.NewInMemoryBus(), Stores: testStores(t), Authenticator: authenticator, Session: session})

	resp, err := srv.GetInstanceInfo(context.Background(), &apiv1.GetInstanceInfoRequest{})
	if err != nil {
		t.Fatalf("GetInstanceInfo: %v", err)
	}
	if resp.GetContractVersion() != apiserver.ContractVersion {
		t.Errorf("contract version = %q, want v1", resp.GetContractVersion())
	}
	if got := resp.GetAuthMethods(); len(got) != 1 || got[0] != "password" {
		t.Errorf("auth methods = %v, want [password]", got)
	}
	if got := resp.GetHeartbeatInterval().AsDuration(); got != config.DefaultHeartbeatInterval {
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

// When the server is armed with a settings provider, GetInstanceInfo reads
// live from the settings store (the single source of truth), so values never
// drift from what the pipeline enforces (TECHNICAL-DECISIONS.md §1.45).
func TestGetInstanceInfoReadsLiveSettings(t *testing.T) {
	authenticator, session := testAuth(t)
	stores := testStores(t)
	// testStores() currently omits Settings; add an in-memory one for this test.
	stores.Settings = store.NewInMemory()
	srv := apiserver.NewServer(apiserver.Settings{Bus: apiserver.NewInMemoryBus(), Stores: stores, Authenticator: authenticator, Session: session})
	// Cast back to the concrete Server to arm settings.
	as := srv
	def := settings.Snapshot{
		AuthMethods:        []string{"password"},
		HeartbeatInterval:  60 * time.Second,
		HeartbeatGrace:     120 * time.Second,
		SessionTTL:         2 * time.Hour,
		OwnerUserIDVisible: true,
	}
	prov, err := settings.New(stores.Settings, def)
	if err != nil {
		t.Fatalf("settings.New: %v", err)
	}
	as.SetInstanceSettings(prov)

	resp, err := as.GetInstanceInfo(context.Background(), &apiv1.GetInstanceInfoRequest{})
	if err != nil {
		t.Fatalf("GetInstanceInfo: %v", err)
	}
	if got := resp.GetHeartbeatInterval().AsDuration(); got != 60*time.Second {
		t.Errorf("heartbeat interval = %v, want 60s", got)
	}
}

// GetInstanceSettings is an authenticated surface that returns the full
// effective settings (superset of the public info), including the non-public
// fields: heartbeat grace, session TTL and owner-id visibility. It is not on
// the public allowlist.
func TestGetInstanceSettings_ReturnsEffectiveValues(t *testing.T) {
	authenticator, session := testAuth(t)
	stores := testStores(t)
	stores.Settings = store.NewInMemory()
	srv := apiserver.NewServer(apiserver.Settings{Bus: apiserver.NewInMemoryBus(), Stores: stores, Authenticator: authenticator, Session: session})
	def := settings.Snapshot{
		AuthMethods:        []string{"password"},
		HeartbeatInterval:  60 * time.Second,
		HeartbeatGrace:     120 * time.Second,
		SessionTTL:         2 * time.Hour,
		OwnerUserIDVisible: true,
	}
	prov, err := settings.New(stores.Settings, def)
	if err != nil {
		t.Fatalf("settings.New: %v", err)
	}
	srv.SetInstanceSettings(prov)

	pwd := getPasswordAuth(t, authenticator)
	_, err = pwd.SignUp("alice", []byte("password123"))
	if err != nil {
		t.Fatalf("signup: %v", err)
	}
	result, err := pwd.Login("alice", []byte("password123"))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	token, err := session.Mint(result.UserID)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", apiserver.BearerPrefix+token))
	resp, err := srv.GetInstanceSettings(ctx, &apiv1.GetInstanceSettingsRequest{})
	if err != nil {
		t.Fatalf("GetInstanceSettings: %v", err)
	}
	if got := resp.GetHeartbeatInterval().AsDuration(); got != 60*time.Second {
		t.Errorf("heartbeat interval = %v, want 60s", got)
	}
	if got := resp.GetHeartbeatGrace().AsDuration(); got != 120*time.Second {
		t.Errorf("heartbeat grace = %v, want 120s", got)
	}
	if got := resp.GetSessionTtl().AsDuration(); got != 2*time.Hour {
		t.Errorf("session ttl = %v, want 2h", got)
	}
	if !resp.GetOwnerUserIdVisible() {
		t.Errorf("owner_user_id_visible = false, want true")
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
		FullMethod: "/abcmovies.api.v1.CoreService/GetInstanceSettings",
	}, handler); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("GetInstanceSettings without token = %v, want Unauthenticated", err)
	}
	if _, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{
		FullMethod: "/abcmovies.api.v1.CoreService/UpdateInstanceSettings",
	}, handler); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("UpdateInstanceSettings without token = %v, want Unauthenticated", err)
	}
	if _, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{
		FullMethod: "/abcmovies.api.v1.CoreService/GetJob",
	}, handler); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("GetJob without token = %v, want Unauthenticated", err)
	}
}

// UpdateInstanceSettings is operator-only: a caller without the operator role
// is refused with PermissionDenied, and an unauthenticated caller is refused
// with Unauthenticated. D3: minimal operator model.
func TestUpdateInstanceSettings_RequiresOperator(t *testing.T) {
	authenticator, session := testAuth(t)
	stores := testStores(t)
	stores.Settings = store.NewInMemory()
	srv := apiserver.NewServer(apiserver.Settings{Bus: apiserver.NewInMemoryBus(), Stores: stores, Authenticator: authenticator, Session: session})
	provider, err := settings.New(stores.Settings, config.DefaultInstanceSettings())
	if err != nil {
		t.Fatalf("settings.New: %v", err)
	}
	srv.SetInstanceSettings(provider)

	// No user in context -> Unauthenticated.
	if _, err := srv.UpdateInstanceSettings(context.Background(), &apiv1.UpdateInstanceSettingsRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("no principal = %v, want Unauthenticated", err)
	}

	// A non-operator user -> PermissionDenied.
	ctx := apiserver.WithUserID(context.Background(), "user:alice")
	if err := auth.NewStoreUserStore(stores.Users).PutUser("alice", &auth.UserData{Role: auth.RoleUser}); err != nil {
		t.Fatalf("PutUser alice: %v", err)
	}
	if _, err := srv.UpdateInstanceSettings(ctx, &apiv1.UpdateInstanceSettingsRequest{}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("regular user = %v, want PermissionDenied", err)
	}
}

// An operator can update settings, and the change is applied live through the
// same pipeline the boot path uses (E2): the value is readable back through
// GetInstanceSettings immediately after the update.
func TestUpdateInstanceSettings_AppliesLive(t *testing.T) {
	authenticator, session := testAuth(t)
	stores := testStores(t)
	stores.Settings = store.NewInMemory()
	srv := apiserver.NewServer(apiserver.Settings{Bus: apiserver.NewInMemoryBus(), Stores: stores, Authenticator: authenticator, Session: session})
	provider, err := settings.New(stores.Settings, config.DefaultInstanceSettings())
	if err != nil {
		t.Fatalf("settings.New: %v", err)
	}
	srv.SetInstanceSettings(provider)

	if err := auth.NewStoreUserStore(stores.Users).PutUser("op", &auth.UserData{Role: auth.RoleOperator}); err != nil {
		t.Fatalf("PutUser op: %v", err)
	}
	ctx := apiserver.WithUserID(context.Background(), "user:op")

	visible := true
	if _, err := srv.UpdateInstanceSettings(ctx, &apiv1.UpdateInstanceSettingsRequest{OwnerUserIdVisible: &visible}); err != nil {
		t.Fatalf("UpdateInstanceSettings as operator: %v", err)
	}
	resp, err := srv.GetInstanceSettings(ctx, &apiv1.GetInstanceSettingsRequest{})
	if err != nil {
		t.Fatalf("GetInstanceSettings: %v", err)
	}
	if !resp.GetOwnerUserIdVisible() {
		t.Errorf("owner_user_id_visible = false, want true after operator update")
	}
}
