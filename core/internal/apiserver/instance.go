package apiserver

import (
	"context"
	"time"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/settings"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

// contractVersion is the api/v1 package version this service speaks
// (TECHNICAL-DECISIONS.md §1.43). Additive evolution never changes it; a
// breaking change would mean a new package, not a new string here.
const contractVersion = "v1"

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
		ContractVersion:   contractVersion,
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
