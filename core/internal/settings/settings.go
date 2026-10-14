// Package settings owns the instance-wide operator settings: their effective
// values, their validation, and the validate -> apply live -> persist
// pipeline (TECHNICAL-DECISIONS.md §1.45). Config supplies a boot-time
// override; the store is the durable record; the pipeline is the single write
// path, so the future UpdateInstanceSettings RPC reuses it unchanged and the
// published values can never drift from the enforced ones.
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nem-git/abcmovies/core/internal/store"
)

// StorageKey is the single key the effective snapshot lives under in the
// settings store class. One record: the settings are a set, not a bag of
// independent keys, so a write is atomic across every setting.
const StorageKey = "instance"

// Snapshot is the effective instance settings (PLAN.md §8). Values are the
// resolved, validated, live-applied ones — the record a client discovers
// through GetInstanceSettings and the values the engine enforces.
type Snapshot struct {
	// AuthMethods are the enabled authentication methods.
	AuthMethods []string
	// HeartbeatInterval is how often a play session's owner must prove
	// liveness (PLAN.md §9.1).
	HeartbeatInterval time.Duration
	// HeartbeatGrace is the server-side slack before a missed heartbeat ends
	// the session.
	HeartbeatGrace time.Duration
	// SessionTTL is the zombie cap: a session with no liveness proof ends
	// after this.
	SessionTTL time.Duration
	// OwnerUserIDVisible reveals which host user owns a shared account to
	// non-owner members and guests (PLAN.md §11). Off by default: an owner
	// always sees their own id, but nobody else does unless this is set.
	OwnerUserIDVisible bool
}

// Validate rejects a snapshot that can never be applied: at least one auth
// method must be named, and every duration must be positive. A rejected value
// is refused, never silently defaulted (PLAN.md §2.5).
func (s Snapshot) Validate() error {
	if len(s.AuthMethods) == 0 {
		return errors.New("settings: auth_methods must name at least one method")
	}
	for _, m := range s.AuthMethods {
		if m == "" {
			return errors.New("settings: auth_methods must not contain an empty method")
		}
	}
	if s.HeartbeatInterval <= 0 {
		return fmt.Errorf("settings: heartbeat_interval must be positive, got %s", s.HeartbeatInterval)
	}
	if s.HeartbeatGrace <= 0 {
		return fmt.Errorf("settings: heartbeat_grace must be positive, got %s", s.HeartbeatGrace)
	}
	if s.SessionTTL <= 0 {
		return fmt.Errorf("settings: session_ttl must be positive, got %s", s.SessionTTL)
	}
	return nil
}

// clone returns a deep copy safe to hand out: the method slice is owned by the
// caller, never the Provider's live state.
func (s Snapshot) clone() Snapshot {
	c := s
	if s.AuthMethods != nil {
		c.AuthMethods = append([]string(nil), s.AuthMethods...)
	}
	return c
}

// Patch is a set of setting changes: only non-nil fields are touched. A nil
// field leaves the current value alone, which is how "absent config key"
// and "field not present in the update request" both resolve to the stored
// value rather than a shipped default.
type Patch struct {
	AuthMethods        *[]string
	HeartbeatInterval  *time.Duration
	HeartbeatGrace     *time.Duration
	SessionTTL         *time.Duration
	OwnerUserIDVisible *bool
}

// Empty reports whether the patch changes nothing.
func (p Patch) Empty() bool {
	return p.AuthMethods == nil && p.HeartbeatInterval == nil && p.HeartbeatGrace == nil &&
		p.SessionTTL == nil && p.OwnerUserIDVisible == nil
}

// LiveApply hands a resolved value to the component that consumes it, before
// the value is persisted (E2: apply live, then persist). A nil function means
// the setting has no separate live consumer — for example the account
// projection reads OwnerUserIDVisible straight from the Provider, so it needs
// no stored copy and cannot drift.
type LiveApply struct {
	AuthMethods        func([]string) error
	HeartbeatInterval  func(time.Duration) error
	HeartbeatGrace     func(time.Duration) error
	SessionTTL         func(time.Duration) error
	OwnerUserIDVisible func(bool) error
}

// Provider holds the effective settings in memory and their durable record in
// the settings store class. It is the single writer: every change runs through
// Apply, so the in-memory snapshot, the persisted record, and the live
// consumers all advance together.
type Provider struct {
	mu       sync.RWMutex
	backend  store.Store
	defaults Snapshot
	current  Snapshot
}

// New builds a Provider over backend. A persisted record is loaded and
// validated; absence means the shipped defaults apply. A corrupt or invalid
// record is a boot failure, never a silent fallback to defaults — that would
// quietly discard the operator's settings.
func New(backend store.Store, defaults Snapshot) (*Provider, error) {
	if backend == nil {
		return nil, errors.New("settings: store is required")
	}
	if err := defaults.Validate(); err != nil {
		return nil, err
	}
	p := &Provider{backend: backend, defaults: defaults.clone(), current: defaults.clone()}
	raw, err := backend.Get(context.Background(), StorageKey)
	switch {
	case errors.Is(err, store.ErrKeyNotFound):
		return p, nil
	case err != nil:
		return nil, fmt.Errorf("settings: read: %w", err)
	}
	snap, err := decode(raw)
	if err != nil {
		return nil, err
	}
	if err := snap.Validate(); err != nil {
		return nil, fmt.Errorf("settings: stored record: %w", err)
	}
	p.current = snap
	return p, nil
}

// Get returns the effective settings. The value is a copy; mutating it never
// changes the Provider.
func (p *Provider) Get(_ context.Context) (Snapshot, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.current.clone(), nil
}

// Apply resolves the patch against the current settings, validates the result,
// hands each resolved value to its live consumer, and only then persists it.
// On any rejection nothing is persisted and the current settings are unchanged
// (the live consumers may have been partially advanced only if a later apply
// fails, which Validate makes a programming error, not an input error).
func (p *Provider) Apply(ctx context.Context, patch Patch, live LiveApply) (Snapshot, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	resolved := p.current
	if patch.AuthMethods != nil {
		resolved.AuthMethods = append([]string(nil), (*patch.AuthMethods)...)
	}
	if patch.HeartbeatInterval != nil {
		resolved.HeartbeatInterval = *patch.HeartbeatInterval
	}
	if patch.HeartbeatGrace != nil {
		resolved.HeartbeatGrace = *patch.HeartbeatGrace
	}
	if patch.SessionTTL != nil {
		resolved.SessionTTL = *patch.SessionTTL
	}
	if patch.OwnerUserIDVisible != nil {
		resolved.OwnerUserIDVisible = *patch.OwnerUserIDVisible
	}

	if err := resolved.Validate(); err != nil {
		return Snapshot{}, err
	}

	// Apply live before persisting: a value the running system cannot take is
	// rejected, not written down to bite at the next restart.
	if live.AuthMethods != nil {
		if err := live.AuthMethods(resolved.AuthMethods); err != nil {
			return Snapshot{}, fmt.Errorf("settings: apply auth_methods: %w", err)
		}
	}
	if live.HeartbeatInterval != nil {
		if err := live.HeartbeatInterval(resolved.HeartbeatInterval); err != nil {
			return Snapshot{}, fmt.Errorf("settings: apply heartbeat_interval: %w", err)
		}
	}
	if live.HeartbeatGrace != nil {
		if err := live.HeartbeatGrace(resolved.HeartbeatGrace); err != nil {
			return Snapshot{}, fmt.Errorf("settings: apply heartbeat_grace: %w", err)
		}
	}
	if live.SessionTTL != nil {
		if err := live.SessionTTL(resolved.SessionTTL); err != nil {
			return Snapshot{}, fmt.Errorf("settings: apply session_ttl: %w", err)
		}
	}
	if live.OwnerUserIDVisible != nil {
		if err := live.OwnerUserIDVisible(resolved.OwnerUserIDVisible); err != nil {
			return Snapshot{}, fmt.Errorf("settings: apply owner_user_id_visible: %w", err)
		}
	}

	raw, err := encode(resolved)
	if err != nil {
		return Snapshot{}, err
	}
	if err := p.backend.Put(ctx, StorageKey, raw); err != nil {
		return Snapshot{}, fmt.Errorf("settings: persist: %w", err)
	}
	p.current = resolved
	return resolved.clone(), nil
}

// record is the on-disk wire shape. Durations are nanoseconds so the record is
// independent of Go's duration formatting.
type record struct {
	AuthMethods        []string `json:"auth_methods"`
	HeartbeatInterval  int64    `json:"heartbeat_interval_ns"`
	HeartbeatGrace     int64    `json:"heartbeat_grace_ns"`
	SessionTTL         int64    `json:"session_ttl_ns"`
	OwnerUserIDVisible bool     `json:"owner_user_id_visible"`
}

func encode(s Snapshot) ([]byte, error) {
	return json.Marshal(record{
		AuthMethods:        s.AuthMethods,
		HeartbeatInterval:  int64(s.HeartbeatInterval),
		HeartbeatGrace:     int64(s.HeartbeatGrace),
		SessionTTL:         int64(s.SessionTTL),
		OwnerUserIDVisible: s.OwnerUserIDVisible,
	})
}

func decode(raw []byte) (Snapshot, error) {
	var r record
	if err := json.Unmarshal(raw, &r); err != nil {
		return Snapshot{}, fmt.Errorf("settings: decode stored record: %w", err)
	}
	return Snapshot{
		AuthMethods:        r.AuthMethods,
		HeartbeatInterval:  time.Duration(r.HeartbeatInterval),
		HeartbeatGrace:     time.Duration(r.HeartbeatGrace),
		SessionTTL:         time.Duration(r.SessionTTL),
		OwnerUserIDVisible: r.OwnerUserIDVisible,
	}, nil
}
