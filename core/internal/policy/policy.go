// Package policy holds the limit vocabulary, validation, and resolution rules
// for ABCMovies' usage policy (PLAN.md §7.2). It is separate from delivery
// engineering: this package tells the engine *what limits apply*; the engine
// applies them.
//
// A Recorded Policy is a map of limit-type to value — the same shape it has
// in the frozen contract — so new limit types need no schema change (§7.2).
// Keys are strings; values parse to their limit's type (concurrentStreams is
// a positive integer, bandwidth is "unlimited" or "<N>Mbps").
//
// In v1 only concurrentStreams is *enforced*. Every other recognised key is
// validated, then stamped on the delivery's recorded policy so auditing and
// future milestones (pacing in M7, quotas) see exactly what was in effect,
// without claiming an enforcement the core cannot honestly perform yet (no
// manifest bitrate is available to police bandwidth, and window/quota
// semantics belong to later milestones). An unknown key is always a startup
// failure, never silently ignored: an operator who typos a key should hear
// about it immediately rather than discover it means no limit.
package policy

import (
	"fmt"
	"strconv"
	"strings"
)

// Limit-type keys recognised by the policy map. Only KeyConcurrentStreams is
// enforced in v1; the rest are carried on delivery jobs for audit and for
// the milestones that will enforce them.
const (
	KeyConcurrentStreams = "concurrentStreams"
	KeyBandwidth         = "bandwidth"
	KeyEncode            = "encode"
	// Recognised, unenforced in v1: values validated at startup, recorded on
	// every job, and acted on by the milestones that own them.
	KeyTimeWindow           = "timeWindow"
	KeyMemberMonthlyQuota   = "memberMonthlyQuotaHours"
	KeyPacingRequestsPerSec = "pacingRequestsPerSecond"
	KeyPacingMaxPulls       = "pacingMaxConcurrentPulls"
	KeyPacingDelay          = "pacingInterRequestDelay"
	KeyPacingRetries        = "pacingRetries"
)

// Set is one limit-type → value map. It satisfies the contract shape of the
// Policy proto (map<string,string>).
type Set map[string]string

var defaults = map[string]string{
	KeyConcurrentStreams: "3",
	KeyBandwidth:         "unlimited",
	KeyEncode:            "disabled",
}

// Defaults returns the shipped default policy (TECHNICAL-DECISIONS.md §1.14):
// every limit key present, each at its shipped baseline.
func Defaults() Set {
	return Clone(defaults)
}

// Clone copies a set so callers can mutate freely (Set is a map).
func Clone(m map[string]string) Set {
	out := make(Set, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// ParseInstance validates the operator's instance-level policy block and
// returns it as a full set: shipped defaults underneath, raw overrides on top.
// Unknown keys and unparseable values are errors — an operator's type is a
// contract with the server, and a silent no-op wants to be a loud refusal
// (PLAN.md §2.5).
func ParseInstance(raw map[string]string) (Set, error) {
	if err := validate(raw); err != nil {
		return nil, err
	}
	return Defaults().With(Set(raw)), nil
}

// ParseOverlay validates a per-account override/cap block. Unlike ParseInstance
// it carries no defaults: the overlay holds exactly the keys the operator set,
// meant to layer over the instance policy.
func ParseOverlay(raw map[string]string) (Set, error) {
	if err := validate(raw); err != nil {
		return nil, err
	}
	return Clone(raw), nil
}

// With returns a copy of s with overlay applied on top.
func (s Set) With(overlay Set) Set {
	out := Clone(s)
	for k, v := range overlay {
		out[k] = v
	}
	return out
}

// Streams reads the enforced concurrent-streams ceiling. The returned bool is
// false when the key is absent, which the engine maps to "no declared limit"
// rather than zero. The invariant "instance sets and overlays are
// policy-validated before the engine sees them" holds: ParseInstance and
// ParseOverlay reject unparseable values. A non-integer value returns (0,
// true) — a stricter-than-zero result — which surfaces early if a caller
// skips validation.
func (s Set) Streams() (int, bool) {
	v, ok := s[KeyConcurrentStreams]
	if !ok {
		return 0, false
	}
	n, _ := strconv.Atoi(v)
	return n, true
}

// EffectiveAccountLimit composes the policy in effect for one account with
// the provider-declared cap: an account admits no more concurrent sessions
// than the lesser of the two (PLAN.md §7.2: min(policy, provider_cap)).
// accountPolicy is the instance default already flattened with the account's
// override (`defaults.With(override)`).
func EffectiveAccountLimit(accountPolicy, providerCap Set) int {
	limit, hasLimit := accountPolicy.Streams()
	if capLimit, hasCap := providerCap.Streams(); hasCap && (!hasLimit || capLimit < limit) {
		return capLimit
	}
	if !hasLimit {
		return 0
	}
	return limit
}

// validate rejects unknown limit types and malformed values. Values for
// recognised-but-unenforced keys are checked for presence only; their formats
// become contractual the moment a milestone enforces them.
func validate(raw map[string]string) error {
	for k, v := range raw {
		switch k {
		case KeyConcurrentStreams:
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return fmt.Errorf("policy %q: concurrentStreams must be a positive integer, got %q", k, v)
			}
		case KeyBandwidth:
			if v != "unlimited" && !validBandwidth(v) {
				return fmt.Errorf(`policy %q: bandwidth must be "unlimited" or "<N>Mbps", got %q`, k, v)
			}
		case KeyEncode:
			if v != "enabled" && v != "disabled" {
				return fmt.Errorf(`policy %q: encode must be "enabled" or "disabled", got %q`, k, v)
			}
		case KeyTimeWindow, KeyMemberMonthlyQuota, KeyPacingRequestsPerSec,
			KeyPacingMaxPulls, KeyPacingDelay, KeyPacingRetries:
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("policy %q: value may not be empty", k)
			}
		default:
			return fmt.Errorf("policy: unknown limit type %q — refusing to start rather than ignore it silently", k)
		}
	}
	return nil
}

func validBandwidth(v string) bool {
	v, found := strings.CutSuffix(v, "Mbps")
	if !found || v == "" {
		return false
	}
	n, err := strconv.Atoi(v)
	return err == nil && n > 0
}
