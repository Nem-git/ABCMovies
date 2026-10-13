package schema

import (
	"fmt"

	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// ValidateAcquireKeysRequest checks an AcquireKeysRequest (PLAN.md §6.6):
// the provider, content id, DRM system, PSSH, and at least one key id are
// required. The auth context is engine-side only and never validated for
// content — it is opaque to the core.
func ValidateAcquireKeysRequest(req *slotsv1.AcquireKeysRequest) error {
	if req == nil {
		return fmt.Errorf("acquire keys request: nil")
	}
	if req.GetProviderId() == "" {
		return fmt.Errorf("acquire keys request: provider_id is required")
	}
	if req.GetContentId() == "" {
		return fmt.Errorf("acquire keys request: content_id is required")
	}
	if req.GetSystem() == slotsv1.DrmSystem_DRM_SYSTEM_UNSPECIFIED {
		return fmt.Errorf("acquire keys request: system is required")
	}
	if len(req.GetPssh()) == 0 {
		return fmt.Errorf("acquire keys request: pssh is required")
	}
	if len(req.GetKeyIds()) == 0 {
		return fmt.Errorf("acquire keys request: at least one key_id is required")
	}
	return nil
}

// ValidateAcquireKeysResponse checks an AcquireKeysResponse (PLAN.md §6.6):
// at least one key is required, and every key must carry a non-empty key id
// and key material. A partial or empty response is a failure, never a
// downgrade.
func ValidateAcquireKeysResponse(resp *slotsv1.AcquireKeysResponse) error {
	if resp == nil {
		return fmt.Errorf("acquire keys response: nil")
	}
	if len(resp.GetKeys()) == 0 {
		return fmt.Errorf("acquire keys response: at least one key is required")
	}
	for i, k := range resp.GetKeys() {
		if k == nil {
			return fmt.Errorf("acquire keys response: key %d is nil", i)
		}
		if k.GetKeyId() == "" {
			return fmt.Errorf("acquire keys response: key %d: key_id is required", i)
		}
		if len(k.GetKeyMaterial()) == 0 {
			return fmt.Errorf("acquire keys response: key %d: key_material is required", i)
		}
	}
	return nil
}

// ValidateDecryptRequest checks a DecryptRequest (PLAN.md §6.6): exactly one
// source (inline bytes or a source reference) and at least one key are
// required.
func ValidateDecryptRequest(req *slotsv1.DecryptRequest) error {
	if req == nil {
		return fmt.Errorf("decrypt request: nil")
	}
	switch req.GetSource().(type) {
	case *slotsv1.DecryptRequest_InlineBytes:
		if len(req.GetInlineBytes()) == 0 {
			return fmt.Errorf("decrypt request: inline_bytes must be non-empty")
		}
	case *slotsv1.DecryptRequest_SourceRef:
		if req.GetSourceRef() == "" {
			return fmt.Errorf("decrypt request: source_ref must be non-empty")
		}
	default:
		return fmt.Errorf("decrypt request: source is required")
	}
	if len(req.GetKeys()) == 0 {
		return fmt.Errorf("decrypt request: at least one key is required")
	}
	for i, k := range req.GetKeys() {
		if k == nil {
			return fmt.Errorf("decrypt request: key %d is nil", i)
		}
		if k.GetKeyId() == "" {
			return fmt.Errorf("decrypt request: key %d: key_id is required", i)
		}
		if len(k.GetKeyMaterial()) == 0 {
			return fmt.Errorf("decrypt request: key %d: key_material is required", i)
		}
	}
	return nil
}

// ValidateDecryptResponse checks a DecryptResponse (PLAN.md §6.6): the
// clear bytes must be non-empty. An empty response is a failure, never a
// downgrade.
func ValidateDecryptResponse(resp *slotsv1.DecryptResponse) error {
	if resp == nil {
		return fmt.Errorf("decrypt response: nil")
	}
	if len(resp.GetClearBytes()) == 0 {
		return fmt.Errorf("decrypt response: clear_bytes must be non-empty")
	}
	return nil
}
