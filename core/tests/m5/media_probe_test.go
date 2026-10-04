package m5_test

import (
	"bytes"
	"context"
	"sync"

	"github.com/nem-git/abcmovies/core/internal/store"
)

// mediaProbe wraps every store the M5 stack persists through and records the
// values that land in them. It is the harness's evidence for PLAN.md §2.4's
// storage rule — caches, vault, jobs, sessions, and friends are allowed
// metadata and keys only, never the audio/video this system moves so metres
// away from them.
type mediaProbe struct {
	mu   sync.Mutex
	puts [][]byte
}

func newMediaProbe() *mediaProbe { return &mediaProbe{} }

func (p *mediaProbe) wrap(s store.Store) store.Store { return &probedStore{Store: s, probe: p} }

// capture marks the byte slice at Put time, before the recorder touches it —
// a store that mutates shared buffers must still be observed.
func (p *mediaProbe) capture(v []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.puts = append(p.puts, bytes.Clone(v))
}

// contains returns true if any Put captured any of the needles.
func (p *mediaProbe) contains(needles ...[]byte) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, haystack := range p.puts {
		for _, needle := range needles {
			if bytes.Contains(haystack, needle) {
				return true
			}
		}
	}
	return false
}

// count is the number of Puts observed, so a test can prove the probe
// actually taps every store instead of vacuously passing.
func (p *mediaProbe) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.puts)
}

type probedStore struct {
	store.Store
	probe *mediaProbe
}

func (p *probedStore) Put(ctx context.Context, key string, value []byte) error {
	p.probe.capture(value)
	return p.Store.Put(ctx, key, value)
}
