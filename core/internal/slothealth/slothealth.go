// Package slothealth tracks the health of provider slots over time, as
// discovered by the credential probes. The write path is the probe result;
// reads are exposed via the API. A slot crossing the healthy/unhealthy
// boundary fires an optional transition hook (Batch 5): best-effort and
// observational — the record stays the system of truth.
package slothealth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nem-git/abcmovies/core/internal/store"
)

const storageKeyPrefix = "slot:"

// Record is the health record for one slot instance (keyed by slot id).
type Record struct {
	SlotID           string
	Healthy          bool
	LastProbeAt      time.Time
	LastSuccessAt    time.Time
	LastFailureAt    time.Time
	FailureCount     int64
	LastErrorKind    string
	LastErrorMessage string
}

// Transition reports one slot crossing the healthy/unhealthy boundary. It is
// the payload of the operational event the bus carries (Batch 5); the probe
// result itself remains the system of record.
type Transition struct {
	SlotID           string
	Healthy          bool
	WasHealthy       bool
	LastErrorKind    string
	LastErrorMessage string
	At               time.Time
}

// Provider holds the durable slot-health records in memory and the store.
type Provider struct {
	mu      sync.RWMutex
	backend store.Store
	// onTransition, when set, is called after a successful persist that
	// crossed the healthy/unhealthy boundary. Best-effort by contract: it is
	// called after the record is durable and its failure (if any) is not the
	// probe writer's concern.
	onTransition func(Transition)
}

// SetTransitionHook wires the observer of health-state transitions. A first
// observation counts as a transition only when the slot is found unhealthy:
// a brand-new slot being reachable is expected, not news.
func (p *Provider) SetTransitionHook(fn func(Transition)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onTransition = fn
}

// New creates a Provider over backend. Absence is normal.
func New(backend store.Store) (*Provider, error) {
	if backend == nil {
		return nil, errors.New("slothealth: store is required")
	}
	return &Provider{backend: backend}, nil
}

func key(slotID string) string { return storageKeyPrefix + slotID }

type stored struct {
	Healthy          bool   `json:"healthy"`
	LastProbeAt      int64  `json:"last_probe_at_ns"`
	LastSuccessAt    int64  `json:"last_success_at_ns"`
	LastFailureAt    int64  `json:"last_failure_at_ns"`
	FailureCount     int64  `json:"failure_count"`
	LastErrorKind    string `json:"last_error_kind"`
	LastErrorMessage string `json:"last_error_message"`
}

func encode(r Record) ([]byte, error) {
	return json.Marshal(stored{
		Healthy:          r.Healthy,
		LastProbeAt:      r.LastProbeAt.UnixNano(),
		LastSuccessAt:    r.LastSuccessAt.UnixNano(),
		LastFailureAt:    r.LastFailureAt.UnixNano(),
		FailureCount:     r.FailureCount,
		LastErrorKind:    r.LastErrorKind,
		LastErrorMessage: r.LastErrorMessage,
	})
}

func decode(slotID string, b []byte) (Record, error) {
	var s stored
	if err := json.Unmarshal(b, &s); err != nil {
		return Record{}, fmt.Errorf("slothealth: decode: %w", err)
	}
	return Record{
		SlotID:           slotID,
		Healthy:          s.Healthy,
		LastProbeAt:      time.Unix(0, s.LastProbeAt),
		LastSuccessAt:    time.Unix(0, s.LastSuccessAt),
		LastFailureAt:    time.Unix(0, s.LastFailureAt),
		FailureCount:     s.FailureCount,
		LastErrorKind:    s.LastErrorKind,
		LastErrorMessage: s.LastErrorMessage,
	}, nil
}

// RecordProbeSuccess writes a successful probe result for slotID. A flip
// from unhealthy to healthy fires the transition hook after the record is
// durable; the first ever observation of a slot fires none (reaching a new
// slot is expected, not news).
func (p *Provider) RecordProbeSuccess(ctx context.Context, slotID string) error {
	if slotID == "" {
		return errors.New("slothealth: slot id required")
	}
	p.mu.Lock()
	var fired bool
	var tr Transition
	now := time.Now()
	wasHealthy := true
	if raw, err := p.backend.Get(ctx, key(slotID)); err == nil {
		if old, err := decode(slotID, raw); err == nil {
			wasHealthy = old.Healthy
		}
	}
	r := Record{
		SlotID:        slotID,
		Healthy:       true,
		LastProbeAt:   now,
		LastSuccessAt: now,
		FailureCount:  0,
	}
	raw, err := encode(r)
	if err != nil {
		p.mu.Unlock()
		return err
	}
	if err := p.backend.Put(ctx, key(slotID), raw); err != nil {
		p.mu.Unlock()
		return err
	}
	if !wasHealthy {
		fired = true
		tr = Transition{SlotID: slotID, Healthy: true, At: now}
	}
	hook := p.onTransition
	p.mu.Unlock()
	if fired && hook != nil {
		hook(tr)
	}
	return nil
}

// RecordProbeFailure writes a failed probe result for slotID. A flip from
// healthy to unhealthy fires the transition hook after the record is durable.
// A first ever observation fires one: a slot found down is news, unlike one
// found up.
func (p *Provider) RecordProbeFailure(ctx context.Context, slotID, kind, msg string) error {
	if slotID == "" {
		return errors.New("slothealth: slot id required")
	}
	p.mu.Lock()
	now := time.Now()
	// Load existing to preserve failure count and detect the transition. A
	// missing record counts as healthy (the reachable-until-proven
	// otherwise), so the first failure of a slot is a transition.
	wasHealthy := true
	var fc int64 = 1
	if raw, err := p.backend.Get(ctx, key(slotID)); err == nil {
		if old, err := decode(slotID, raw); err == nil {
			fc = old.FailureCount + 1
			wasHealthy = old.Healthy
		}
	}
	r := Record{
		SlotID:           slotID,
		Healthy:          false,
		LastProbeAt:      now,
		LastFailureAt:    now,
		FailureCount:     fc,
		LastErrorKind:    kind,
		LastErrorMessage: msg,
	}
	raw, err := encode(r)
	if err != nil {
		p.mu.Unlock()
		return err
	}
	if err := p.backend.Put(ctx, key(slotID), raw); err != nil {
		p.mu.Unlock()
		return err
	}
	var fired bool
	var tr Transition
	if wasHealthy {
		fired = true
		tr = Transition{
			SlotID:           slotID,
			Healthy:          false,
			WasHealthy:       true,
			LastErrorKind:    kind,
			LastErrorMessage: msg,
			At:               now,
		}
	}
	hook := p.onTransition
	p.mu.Unlock()
	if fired && hook != nil {
		hook(tr)
	}
	return nil
}

// Get returns the record for slotID, or (empty,false) if absent.
func (p *Provider) Get(ctx context.Context, slotID string) (Record, bool, error) {
	if slotID == "" {
		return Record{}, false, errors.New("slothealth: slot id required")
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	raw, err := p.backend.Get(ctx, key(slotID))
	if errors.Is(err, store.ErrKeyNotFound) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, err
	}
	r, err := decode(slotID, raw)
	if err != nil {
		return Record{}, false, err
	}
	return r, true, nil
}

// List returns all records under the prefix. For each key with the
// prefix stripped, returns its decoded record. Corrupt records are
// silently skipped.
func (p *Provider) List(ctx context.Context) ([]Record, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	keys, err := p.backend.List(ctx, storageKeyPrefix)
	if err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(keys))
	for _, k := range keys {
		slotID := k[len(storageKeyPrefix):]
		raw, err := p.backend.Get(ctx, k)
		if err != nil {
			continue
		}
		r, err := decode(slotID, raw)
		if err != nil {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}
