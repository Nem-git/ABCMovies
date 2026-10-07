package apiserver

import (
	"context"
	"time"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/config"
	"google.golang.org/protobuf/types/known/durationpb"
)

// contractVersion is the api/v1 package version this service speaks
// (TECHNICAL-DECISIONS.md §1.43). Additive evolution never changes it; a
// breaking change would mean a new package, not a new string here.
const contractVersion = "v1"

// SetHeartbeatInterval arms the play-session liveness cadence published by
// GetInstanceInfo (PLAN.md §9.1). The composition root arms it from the same
// parsed config the delivery engine enforces, so the published value and the
// enforced value can never drift apart.
func (s *Server) SetHeartbeatInterval(d time.Duration) {
	if d <= 0 {
		return
	}
	s.heartbeatInterval = d
}

// GetInstanceInfo returns the non-sensitive instance parameters a client
// needs before it can behave correctly: which auth methods to render at
// login and how often to heartbeat a play session (PLAN.md §9.1,
// TECHNICAL-DECISIONS.md §1.43). It is public — reachable without a token —
// and instance-wide by contract: nothing member-scoped or operator-sensitive
// may ever be added to this response; per-user discovery is a separate,
// authenticated surface.
func (s *Server) GetInstanceInfo(_ context.Context, _ *apiv1.GetInstanceInfoRequest) (*apiv1.GetInstanceInfoResponse, error) {
	return &apiv1.GetInstanceInfoResponse{
		ContractVersion:   contractVersion,
		AuthMethods:       s.auth.Methods(),
		HeartbeatInterval: durationpb.New(s.heartbeatInterval),
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
