package apiserver

import (
	"context"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ListSlotHealth returns the health records for all provider slots.
// Operator-only (D4): slot health is operational data about the instance's
// upstream slots, so a regular member has no use for it and the surface
// stays gated the same way instance-settings writes are. The check runs
// before the nil-store short-circuit, so an unwired server does not leak
// that the data is absent to an unauthorized caller either.
func (s *Server) ListSlotHealth(ctx context.Context, _ *apiv1.ListSlotHealthRequest) (*apiv1.ListSlotHealthResponse, error) {
	if err := s.requireOperator(ctx); err != nil {
		return nil, err
	}
	if s.slotHealth == nil {
		return &apiv1.ListSlotHealthResponse{}, nil
	}
	recs, err := s.slotHealth.List(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to read slot health")
	}
	out := make([]*apiv1.SlotHealth, 0, len(recs))
	for _, r := range recs {
		kind := apiv1.SlotHealthErrorKind_SLOT_HEALTH_ERROR_KIND_UNSPECIFIED
		switch r.LastErrorKind {
		case "AUTH":
			kind = apiv1.SlotHealthErrorKind_SLOT_HEALTH_ERROR_KIND_AUTH
		case "NETWORK":
			kind = apiv1.SlotHealthErrorKind_SLOT_HEALTH_ERROR_KIND_NETWORK
		case "TIMEOUT":
			kind = apiv1.SlotHealthErrorKind_SLOT_HEALTH_ERROR_KIND_TIMEOUT
		case "UNKNOWN":
			kind = apiv1.SlotHealthErrorKind_SLOT_HEALTH_ERROR_KIND_UNKNOWN
		}
		sh := &apiv1.SlotHealth{
			SlotId:           r.SlotID,
			Healthy:          r.Healthy,
			LastProbeAt:      timestamppb.New(r.LastProbeAt),
			LastSuccessAt:    timestamppb.New(r.LastSuccessAt),
			LastFailureAt:    timestamppb.New(r.LastFailureAt),
			FailureCount:     r.FailureCount,
			LastErrorKind:    kind,
			LastErrorMessage: r.LastErrorMessage,
		}
		out = append(out, sh)
	}
	return &apiv1.ListSlotHealthResponse{Slots: out}, nil
}
