package builtin

import (
	"context"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

// Slot is the built-in in-process slot (§4 of PLAN.md). It is the reference
// Meta-service implementation: it declares the meta-contract itself.
type Slot struct {
	corev1.UnimplementedMetaServiceServer
}

// New returns the built-in slot.
func New() *Slot {
	return &Slot{}
}

// CapabilityQuery answers the meta-contract: the built-in slot speaks the
// meta-contract at version 1.
func (s *Slot) CapabilityQuery(_ context.Context, _ *corev1.CapabilityQueryRequest) (*corev1.CapabilityQueryResponse, error) {
	return &corev1.CapabilityQueryResponse{
		Capabilities: []*corev1.Capability{
			{Name: "meta", Version: 1},
		},
	}, nil
}
