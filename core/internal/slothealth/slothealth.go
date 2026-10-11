// Package slothealth tracks the health of provider slots over time, as
// discovered by the credential probes. The batch-6 design keeps writes to the
// probe result path and exposes a read-only view via the API; no events or
// permission changes are introduced here.
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

// Provider holds the durable slot-health records in memory and the store.
type Provider struct {
	mu      sync.RWMutex
	backend store.Store
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

// RecordProbeSuccess writes a successful probe result for slotID.
func (p *Provider) RecordProbeSuccess(ctx context.Context, slotID string) error {
	if slotID == "" {
		return errors.New("slothealth: slot id required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	r := Record{
		SlotID:        slotID,
		Healthy:       true,
		LastProbeAt:   now,
		LastSuccessAt: now,
		FailureCount:  0,
	}
	raw, err := encode(r)
	if err != nil {
		return err
	}
	return p.backend.Put(ctx, key(slotID), raw)
}

// RecordProbeFailure writes a failed probe result for slotID.
func (p *Provider) RecordProbeFailure(ctx context.Context, slotID, kind, msg string) error {
	if slotID == "" {
		return errors.New("slothealth: slot id required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	// Load existing to preserve failure count
	var fc int64
	if raw, err := p.backend.Get(ctx, key(slotID)); err == nil {
		if old, err := decode(slotID, raw); err == nil {
			fc = old.FailureCount + 1
		} else {
			fc = 1
		}
	} else {
		fc = 1
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
		return err
	}
	return p.backend.Put(ctx, key(slotID), raw)
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
