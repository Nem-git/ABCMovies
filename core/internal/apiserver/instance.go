package apiserver

import (
	"context"
	"strings"
	"time"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/settings"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

// ContractVersion is the api/v1 package version this service speaks
// (TECHNICAL-DECISIONS.md §1.43). Additive evolution never changes it; a
// breaking change would mean a new package, not a new string here.
const ContractVersion = "v1"

// SetInstanceSettings arms the live instance-settings source. From then on it
// is authoritative for the instance RPCs and the owner-id visibility
// projection; the construction-time heartbeatInterval and auth fallback are
// used only by a server built without the composition root.
func (s *Server) SetInstanceSettings(src InstanceSettings) {
	s.instanceSettings = src
}

// SetHeartbeatInterval arms the play-session liveness cadence published by
// GetInstanceInfo (PLAN.md §9.1). The composition root arms it from the same
// parsed config the delivery engine enforces, so the published value and the
// enforced value can never drift apart. It is a fallback for a server built
// without the live settings source.
func (s *Server) SetHeartbeatInterval(d time.Duration) {
	if d <= 0 {
		return
	}
	s.heartbeatInterval = d
}

// instanceSnapshot returns the effective instance settings. When the live
// source is armed it is authoritative — read straight from the settings
// store, so it can never drift from what the pipeline enforces. Otherwise a
// server built without the composition root reports the methods and cadence
// it was armed with over the shipped defaults.
func (s *Server) instanceSnapshot(ctx context.Context) (settings.Snapshot, error) {
	if s.instanceSettings != nil {
		return s.instanceSettings.Get(ctx)
	}
	snap := config.DefaultInstanceSettings()
	if s.auth != nil {
		snap.AuthMethods = s.auth.Methods()
	}
	if s.heartbeatInterval > 0 {
		snap.HeartbeatInterval = s.heartbeatInterval
	}
	return snap, nil
}

// GetInstanceInfo returns the non-sensitive instance parameters a client
// needs before it can behave correctly: which auth methods to render at
// login and how often to heartbeat a play session (PLAN.md §9.1,
// TECHNICAL-DECISIONS.md §1.43). It is public — reachable without a token —
// and instance-wide by contract: nothing member-scoped or operator-sensitive
// may ever be added to this response; per-user discovery is a separate,
// authenticated surface.
func (s *Server) GetInstanceInfo(ctx context.Context, _ *apiv1.GetInstanceInfoRequest) (*apiv1.GetInstanceInfoResponse, error) {
	snap, err := s.instanceSnapshot(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "instance settings unavailable")
	}
	return &apiv1.GetInstanceInfoResponse{
		ContractVersion:   ContractVersion,
		AuthMethods:       snap.AuthMethods,
		HeartbeatInterval: durationpb.New(snap.HeartbeatInterval),
	}, nil
}

// GetInstanceSettings returns the effective instance settings to an
// authenticated caller (TECHNICAL-DECISIONS.md §1.45). It mirrors what
// GetInstanceInfo publishes — the auth methods and the heartbeat cadence —
// and adds the settings that are not client-facing: the heartbeat grace and
// session TTL the engine enforces, and whether a shared account's owner id is
// visible to non-owner members. Unlike GetInstanceInfo it is authenticated:
// the values are instance configuration, not login-bootstrap parameters.
func (s *Server) GetInstanceSettings(ctx context.Context, _ *apiv1.GetInstanceSettingsRequest) (*apiv1.GetInstanceSettingsResponse, error) {
	snap, err := s.instanceSnapshot(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "instance settings unavailable")
	}
	return &apiv1.GetInstanceSettingsResponse{
		AuthMethods:        snap.AuthMethods,
		HeartbeatInterval:  durationpb.New(snap.HeartbeatInterval),
		HeartbeatGrace:     durationpb.New(snap.HeartbeatGrace),
		SessionTtl:         durationpb.New(snap.SessionTTL),
		OwnerUserIdVisible: snap.OwnerUserIDVisible,
	}, nil
}

// defaultHeartbeatInterval resolves the shipped default so a server built
// without the composition root (unit tests) still publishes a sane value;
// the number's single home is the config package.
func defaultHeartbeatInterval() time.Duration {
	t, err := config.ParseDeliveryTiming(config.DeliveryConfig{})
	if err != nil {
		return config.DefaultHeartbeatInterval
	}
	return t.HeartbeatInterval
}

// settingsApplier is the write side of the instance-settings seam: the
// settings Provider implements it, so the API layer reaches the same
// validate -> apply live -> persist pipeline the boot path uses (E2)
// without importing the Provider directly.
type settingsApplier interface {
	Apply(context.Context, settings.Patch, settings.LiveApply) (settings.Snapshot, error)
}

// requireOperator refuses the call unless the authenticated caller carries
// the operator role. Operator-gated surfaces (settings updates, slot health,
// and later the admin slot surface) share this check, so the rule lives in
// one place (D3: minimal operator model — role on the user record, bootstrap
// first user to OPERATOR).
func (s *Server) requireOperator(ctx context.Context) error {
	uid, ok := UserIDFromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "authentication required")
	}
	username, found := strings.CutPrefix(uid, "user:")
	if !found {
		return status.Error(codes.PermissionDenied, "operator role required")
	}
	if s.stores.Users == nil {
		return status.Error(codes.FailedPrecondition, "user store unavailable")
	}
	data, err := auth.NewStoreUserStore(s.stores.Users).GetUser(username)
	if err != nil {
		return status.Error(codes.PermissionDenied, "operator role required")
	}
	if data.Role != auth.RoleOperator {
		return status.Error(codes.PermissionDenied, "operator role required")
	}
	return nil
}

// applySettingsPatch runs one update through the settings pipeline. A
// server built without the composition root (no armed instanceSettings)
// has nothing to write to, so an update is refused rather than silently
// dropped.
func (s *Server) applySettingsPatch(patch settings.Patch) error {
	p, ok := s.instanceSettings.(settingsApplier)
	if !ok {
		return status.Error(codes.FailedPrecondition, "instance settings are not writable")
	}
	live := settings.LiveApply{}
	if s.auth != nil {
		live.AuthMethods = s.auth.SetMethods
	}
	_, err := p.Apply(context.Background(), patch, live)
	return err
}

func (s *Server) UpdateInstanceSettings(ctx context.Context, req *apiv1.UpdateInstanceSettingsRequest) (*apiv1.UpdateInstanceSettingsResponse, error) {
	if err := s.requireOperator(ctx); err != nil {
		return nil, err
	}
	patch := settings.Patch{}
	if req.OwnerUserIdVisible != nil {
		patch.OwnerUserIDVisible = req.OwnerUserIdVisible
	}
	if err := s.applySettingsPatch(patch); err != nil {
		return nil, err
	}
	return &apiv1.UpdateInstanceSettingsResponse{}, nil
}
