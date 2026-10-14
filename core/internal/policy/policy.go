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
// In v1 only concurrentStreams and the pacing keys are *enforced*. Every
// other recognised key is validated, then stamped on the delivery's recorded
// policy so auditing and future milestones (quotas) see exactly what was in
// effect, without claiming an enforcement the core cannot honestly perform
// yet (no manifest bitrate is available to police bandwidth, and
// window/quota
// semantics belong to later milestones). An unknown key is always a startup
// failure, never silently ignored: an operator who typos a key should hear
// about it immediately rather than discover it means no limit.
package policy

import (
	"fmt"
	"strconv"
	"strings"
	"time"
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

// Shipped default policy values (TECHNICAL-DECISIONS.md §1.14).
const (
	// DefaultConcurrentStreamsValue is the shipped default for concurrent streams.
	DefaultConcurrentStreamsValue = "3"
	// DefaultBandwidthValue is the shipped default for bandwidth.
	DefaultBandwidthValue = "unlimited"
	// DefaultEncodeValue is the shipped default for encode.
	DefaultEncodeValue = "disabled"
)

var defaults = map[string]string{
	KeyConcurrentStreams: DefaultConcurrentStreamsValue,
	KeyBandwidth:         DefaultBandwidthValue,
	KeyEncode:            DefaultEncodeValue,
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

// ProviderCaps is the provider-declared ceiling on one account's concurrent
// streams: the second operand of admission's min(policy, provider_cap)
// (PLAN.md §7.2). It is deliberately a distinct type from Set. A Set is the
// operator's policy vocabulary — many keys, defaults, overlays, optional —
// while a cap is a single declared number that can only ever pull the limit
// down, so the two operands must not be swappable at a call site without the
// compiler noticing. No wire change: provider_cap stays a Policy on the
// contract, rendered back through Limits.
type ProviderCaps struct {
	// streams is the declared ceiling. declared distinguishes "no cap was
	// declared" from "a cap of zero", which would mean admit nothing —
	// only the former is the unbound answer.
	streams  int
	declared bool
}

// CapsFor builds caps from a declared max-concurrent-streams value: zero is
// the documented "unbound" spelling (TECHNICAL-DECISIONS.md §1.42), so it
// yields undeclared caps and the instance policy governs alone.
func CapsFor(n uint32) ProviderCaps {
	if n == 0 {
		return ProviderCaps{}
	}
	return ProviderCaps{streams: int(n), declared: true}
}

// ParseCaps parses the map spelling of a cap — the shape config extensions
// and the provider_cap round-trip both use — into caps. Unlike ParseInstance
// it carries no defaults and accepts no other key: a provider cap declares
// what the provider allows, not how the instance runs, so a map that tries
// to smuggle a policy key into it is refused rather than silently ignored.
func ParseCaps(raw map[string]string) (ProviderCaps, error) {
	var c ProviderCaps
	for k, v := range raw {
		switch k {
		case KeyConcurrentStreams:
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return ProviderCaps{}, fmt.Errorf("provider cap %q: concurrentStreams must be a positive integer, got %q", k, v)
			}
			c = ProviderCaps{streams: n, declared: true}
		default:
			return ProviderCaps{}, fmt.Errorf("provider cap: %q is a policy key, not a cap — refusing rather than ignore it", k)
		}
	}
	return c, nil
}

// Streams reads the declared ceiling; the bool is false when no cap was
// declared, which callers map to "the other operand decides".
func (c ProviderCaps) Streams() (int, bool) { return c.streams, c.declared }

// Limits renders caps back to the contract's limit map for the recorded
// provider_cap (PLAN.md §2.4 storage shape); undeclared caps render nil, so
// an unbound account records exactly what it is.
func (c ProviderCaps) Limits() map[string]string {
	if !c.declared {
		return nil
	}
	return map[string]string{KeyConcurrentStreams: strconv.Itoa(c.streams)}
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
// override (`defaults.With(override)`); caps is the provider's own declared
// ceiling, whose absence means the policy decides alone. The parameter types
// keep the two operands distinct: Set is policy, ProviderCaps is a cap.
func EffectiveAccountLimit(accountPolicy Set, caps ProviderCaps) int {
	limit, hasLimit := accountPolicy.Streams()
	if capLimit, hasCap := caps.Streams(); hasCap && (!hasLimit || capLimit < limit) {
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
		case KeyTimeWindow, KeyMemberMonthlyQuota:
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("policy %q: value may not be empty", k)
			}
		case KeyPacingRequestsPerSec:
			f, err := strconv.ParseFloat(v, 64)
			if err != nil || f <= 0 {
				return fmt.Errorf("policy %q: pacingRequestsPerSecond must be a positive number, got %q", k, v)
			}
		case KeyPacingMaxPulls:
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return fmt.Errorf("policy %q: pacingMaxConcurrentPulls must be a positive integer, got %q", k, v)
			}
		case KeyPacingDelay:
			d, err := time.ParseDuration(v)
			if err != nil || d <= 0 {
				return fmt.Errorf("policy %q: pacingInterRequestDelay must be a positive duration, got %q", k, v)
			}
		case KeyPacingRetries:
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return fmt.Errorf("policy %q: pacingRetries must be a non-negative integer, got %q", k, v)
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
