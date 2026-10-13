// Package slotvalidation validates managed slots (PLAN.md §3.1, D8). Both
// the config loader and the admin API call these rules, so a slot is valid
// exactly once regardless of how it was submitted.
package slotvalidation

import (
	"fmt"
	"strconv"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/policy"
)

// ValidateSlot checks a slot's invariants: id and kind are required, the
// config matches the kind, and kind-specific rules hold. Uniqueness is the
// store's concern (it knows the existing keys); this validates one slot.
func ValidateSlot(slot *corev1.Slot) error {
	if slot == nil {
		return fmt.Errorf("slot: nil")
	}
	if slot.GetId() == "" {
		return fmt.Errorf("slot: id is required")
	}
	switch slot.GetKind() {
	case corev1.SlotKind_SLOT_KIND_PROVIDER:
		return validateProvider(slot.GetId(), slot.GetProvider())
	case corev1.SlotKind_SLOT_KIND_SINK:
		return validateSink(slot.GetId(), slot.GetSink())
	case corev1.SlotKind_SLOT_KIND_CATALOGUE:
		return validateCatalogue(slot.GetId(), slot.GetCatalogue())
	case corev1.SlotKind_SLOT_KIND_SUBTITLE_SOURCE:
		return validateSubtitleSource(slot.GetId(), slot.GetSubtitleSource())
	case corev1.SlotKind_SLOT_KIND_DRM:
		return validateDrm(slot.GetId(), slot.GetDrm())
	case corev1.SlotKind_SLOT_KIND_UNSPECIFIED:
		return fmt.Errorf("slot %q: kind is required", slot.GetId())
	default:
		return fmt.Errorf("slot %q: unknown kind %v", slot.GetId(), slot.GetKind())
	}
}

func validateProvider(id string, p *corev1.ProviderConfig) error {
	if p == nil {
		return fmt.Errorf("provider slot %q: config is required", id)
	}
	if p.GetAdapter() == "" {
		return fmt.Errorf("provider slot %q: adapter is required", id)
	}
	if p.GetServer() == "" {
		return fmt.Errorf("provider slot %q: server is required", id)
	}
	for _, a := range p.GetAccounts() {
		if a.GetId() == "" {
			return fmt.Errorf("provider slot %q: account entry missing id", id)
		}
		if a.GetUsername() == "" {
			return fmt.Errorf("provider slot %q account %q: username is required", id, a.GetId())
		}
		if a.GetPasswordEnv() == "" {
			return fmt.Errorf("provider slot %q account %q: password_env is required", id, a.GetId())
		}
		// An account's policy overrides the instance policy for that account
		// only. A malformed value or an unknown key refuses the slot, never
		// silently falls back — the operator hears about a typo at submit
		// time instead of running with an unintended budget (§2.5).
		if _, err := policy.ParseOverlay(a.GetPolicy()); err != nil {
			return fmt.Errorf("provider slot %q account %q: %w", id, a.GetId(), err)
		}
	}
	return validateOptions(id, p.GetOptions())
}

func validateSink(id string, s *corev1.SinkConfig) error {
	if s == nil {
		return fmt.Errorf("sink slot %q: config is required", id)
	}
	if s.GetAdapter() == "" {
		return fmt.Errorf("sink slot %q: adapter is required", id)
	}
	return nil
}

func validateCatalogue(id string, c *corev1.CatalogueConfig) error {
	if c == nil {
		return fmt.Errorf("catalogue slot %q: config is required", id)
	}
	if c.GetAdapter() == "" {
		return fmt.Errorf("catalogue slot %q: adapter is required", id)
	}
	if c.GetTokenEnv() == "" {
		return fmt.Errorf("catalogue slot %q: token_env is required", id)
	}
	return nil
}

func validateSubtitleSource(id string, s *corev1.SubtitleSourceConfig) error {
	if s == nil {
		return fmt.Errorf("subtitle-source slot %q: config is required", id)
	}
	if s.GetAdapter() == "" {
		return fmt.Errorf("subtitle-source slot %q: adapter is required", id)
	}
	return nil
}

func validateDrm(id string, d *corev1.DrmConfig) error {
	if d == nil {
		return fmt.Errorf("drm slot %q: config is required", id)
	}
	if len(d.GetOperations()) == 0 {
		return fmt.Errorf("drm slot %q: at least one operation is required", id)
	}
	return nil
}

func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

func validateDuration(s string) error {
	_, err := time.ParseDuration(s)
	return err
}

// validateOptions fails when a slot's pacing knobs in the options bag are
// mis-typed: a silent fallback would run the instance with no aggregate
// bound, the opposite of what the operator asked for (§2.5; mirrors the
// strict policy-key validation).
func validateOptions(id string, options map[string]string) error {
	for k, v := range options {
		switch k {
		case policy.KeyPacingRequestsPerSec:
			f, err := parseFloat(v)
			if err != nil || f <= 0 {
				return fmt.Errorf("slot %q: options %q must be a positive number, got %q", id, k, v)
			}
		case policy.KeyPacingMaxPulls:
			n, err := parseInt(v)
			if err != nil || n < 1 {
				return fmt.Errorf("slot %q: options %q must be a positive integer, got %q", id, k, v)
			}
		case policy.KeyPacingDelay:
			if err := validateDuration(v); err != nil {
				return fmt.Errorf("slot %q: options %q: %v", id, k, err)
			}
		}
	}
	return nil
}
