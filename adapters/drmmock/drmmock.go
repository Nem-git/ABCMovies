// Package drmmock implements a mock DRM slot adapter for testing the DRM
// contract (PLAN.md §6.6). It returns canned keys and decrypts to canned
// clear bytes — never touching real DRM content. This is the adapter the
// fixture suite uses to verify the DRM contract end-to-end.
//
// The mock is admitted on contract-only (mock) fixtures (PLAN.md §2.5):
// it validates the license-negotiation message shapes (challenge, response,
// keys) but never decrypts real content. A real DRM slot (local CDM or
// remote key provider) would be a separate adapter.
package drmmock

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/protobuf/types/known/durationpb"

	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// Adapter is the mock DRM slot adapter. It returns canned keys and
// decrypts to canned clear bytes.
type Adapter struct {
	logger *slog.Logger
}

// NewAdapter creates a mock DRM slot adapter.
func NewAdapter(logger *slog.Logger) *Adapter {
	if logger == nil {
		logger = slog.Default()
	}
	return &Adapter{logger: logger}
}

// AcquireKeys returns canned content keys for the requested key IDs. It
// validates the request shape (PLAN.md §6.6) but never touches real DRM
// content.
func (a *Adapter) AcquireKeys(ctx context.Context, req *slotsv1.AcquireKeysRequest) (*slotsv1.AcquireKeysResponse, error) {
	if err := validateAcquireKeysRequest(req); err != nil {
		return nil, err
	}
	a.logger.Info("drmmock: acquiring keys",
		"provider", req.GetProviderId(),
		"content", req.GetContentId(),
		"system", req.GetSystem(),
		"key_ids", req.GetKeyIds(),
	)
	keys := make([]*slotsv1.ContentKey, 0, len(req.GetKeyIds()))
	for _, kid := range req.GetKeyIds() {
		keys = append(keys, &slotsv1.ContentKey{
			KeyId:       kid,
			KeyMaterial: []byte("mock-key-material"),
			Validity:    durationpb.New(3600000000000), // 1 hour in nanoseconds
		})
	}
	return &slotsv1.AcquireKeysResponse{Keys: keys}, nil
}

// Decrypt returns canned clear bytes. It validates the request shape
// (PLAN.md §6.6) but never touches real DRM content.
func (a *Adapter) Decrypt(ctx context.Context, req *slotsv1.DecryptRequest) (*slotsv1.DecryptResponse, error) {
	if err := validateDecryptRequest(req); err != nil {
		return nil, err
	}
	a.logger.Info("drmmock: decrypting", "source", req.GetSourceRef())
	return &slotsv1.DecryptResponse{ClearBytes: []byte("mock-clear-bytes")}, nil
}

func validateAcquireKeysRequest(req *slotsv1.AcquireKeysRequest) error {
	if req.GetProviderId() == "" {
		return fmt.Errorf("drmmock: provider_id is required")
	}
	if req.GetContentId() == "" {
		return fmt.Errorf("drmmock: content_id is required")
	}
	if req.GetSystem() == slotsv1.DrmSystem_DRM_SYSTEM_UNSPECIFIED {
		return fmt.Errorf("drmmock: system is required")
	}
	if len(req.GetPssh()) == 0 {
		return fmt.Errorf("drmmock: pssh is required")
	}
	if len(req.GetKeyIds()) == 0 {
		return fmt.Errorf("drmmock: at least one key_id is required")
	}
	return nil
}

func validateDecryptRequest(req *slotsv1.DecryptRequest) error {
	if req.GetSourceRef() == "" {
		return fmt.Errorf("drmmock: source_ref is required")
	}
	if len(req.GetKeys()) == 0 {
		return fmt.Errorf("drmmock: at least one key is required")
	}
	return nil
}
