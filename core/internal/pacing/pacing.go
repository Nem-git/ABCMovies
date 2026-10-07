// Package pacing enforces PLAN.md §7.2: per-account pacing budgets and the
// per-provider aggregate governor. It deliberately depends on the standard
// library only — the budget is small enough to own.
package pacing

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Budget is one account's pacing allowance: requests/sec, concurrent pulls,
// inter-request delay, and retries. A zero value field means "no limit" for
// that axis, and the zero Budget admits everything — pacing is off by default
// (§7.2) until a host or account enables it.
type Budget struct {
	RequestsPerSec float64
	MaxConcurrent  int
	MinSpacing     time.Duration
	MaxRetries     int
}

var (
	ErrOverBudget   = errors.New("pacing: budget exhausted for this account")
	ErrDegraded     = errors.New("pacing: provider degraded (backoff active)")
	ErrGovernorBusy = errors.New("pacing: per-provider aggregate governor at capacity")
)

// Limiter admits requests one at a time against a Budget. The pacing budget
// is shared, so background refresh and enrichment draw from — and can never
// exceed — the same allowance as user traffic.
type Limiter struct {
	budget Budget
	now    func() time.Time

	mu       sync.Mutex
	inflight int
	nextOK   time.Time
}

// NewLimiter builds a Limiter. now is injectable for tests.
func NewLimiter(b Budget, now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	return &Limiter{budget: b, now: now}
}

// Allow blocks until the Budget admits one more request or ctx is done. Call
// Release exactly once per successful Allow.
func (l *Limiter) Allow(ctx context.Context) error {
	for {
		l.mu.Lock()
		if l.budget.MaxConcurrent > 0 && l.inflight >= l.budget.MaxConcurrent {
			l.mu.Unlock()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Millisecond):
				continue
			}
		}
		var wait time.Duration
		now := l.now()
		switch {
		case l.budget.RequestsPerSec > 0:
			interval := time.Duration(float64(time.Second) / l.budget.RequestsPerSec)
			if now.Before(l.nextOK) {
				wait = l.nextOK.Sub(now)
				l.nextOK = l.nextOK.Add(interval) // reserve the slot now; sleep into it
			} else {
				l.nextOK = now.Add(interval)
			}
		case l.budget.MinSpacing > 0:
			if now.Before(l.nextOK) {
				wait = l.nextOK.Sub(now)
				l.nextOK = l.nextOK.Add(l.budget.MinSpacing)
			} else {
				l.nextOK = now.Add(l.budget.MinSpacing)
			}
		}
		if wait <= 0 {
			l.inflight++
			l.mu.Unlock()
			return nil
		}
		l.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
			l.mu.Lock()
			l.inflight++
			l.mu.Unlock()
			return nil
		}
	}
}

// Release marks one request finished.
func (l *Limiter) Release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inflight > 0 {
		l.inflight--
	}
}

// Governor is the per-provider aggregate cap: every account of that provider
// is funneled through the same Governor, so total load is bounded regardless
// of how many accounts or members share it (§7.2). It also carries the
// per-provider error signal — one throttled account throttles all (§5.4).
type Governor struct {
	RequestsPerSec float64
	now            func() time.Time

	mu       sync.Mutex
	nextOK   time.Time
	fails    int
	backoffT time.Time
	base     time.Duration
	max      time.Duration
}

// NewGovernor builds a Governor. base and max bound the per-provider backoff.
func NewGovernor(rps float64, base, max time.Duration, now func() time.Time) *Governor {
	if now == nil {
		now = time.Now
	}
	return &Governor{RequestsPerSec: rps, now: now, base: base, max: max}
}

// Allow blocks until the aggregate cap admits one more request, or fails fast
// while the provider is in backoff.
func (g *Governor) Allow(ctx context.Context) error {
	for {
		g.mu.Lock()
		if g.now().Before(g.backoffT) {
			g.mu.Unlock()
			return ErrDegraded
		}
		var wait time.Duration
		now := g.now()
		if g.RequestsPerSec > 0 {
			interval := time.Duration(float64(time.Second) / g.RequestsPerSec)
			if now.Before(g.nextOK) {
				wait = g.nextOK.Sub(now)
				g.nextOK = g.nextOK.Add(interval)
			} else {
				g.nextOK = now.Add(interval)
			}
		}
		if wait <= 0 {
			g.mu.Unlock()
			return nil
		}
		g.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
			return nil
		}
	}
}

// NoteError grows the shared backoff — a per-provider signal (§5.4): one
// throttled account throttles every account's traffic to that provider.
func (g *Governor) NoteError() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.fails++
	d := g.base << (g.fails - 1)
	if d > g.max || d <= 0 {
		d = g.max
	}
	g.backoffT = g.now().Add(d)
}

// NoteSuccess clears the shared backoff on a clean provider response.
func (g *Governor) NoteSuccess() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.fails = 0
	g.backoffT = time.Time{}
}

// Gate runs one request under both the account Limiter and the provider
// Governor — the single fan-out, one-enforcement-point composition (§7.2) —
// and applies the retry bound from the Budget.
func Gate(ctx context.Context, l *Limiter, g *Governor, b Budget, fn func(ctx context.Context) error) error {
	var last error
	for attempt := 0; attempt <= b.MaxRetries; attempt++ {
		if err := l.Allow(ctx); err != nil {
			return err
		}
		if err := g.Allow(ctx); err != nil {
			l.Release()
			return err
		}
		err := fn(ctx)
		l.Release()
		if err == nil {
			g.NoteSuccess()
			return nil
		}
		last = err
		g.NoteError()
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
	}
	return fmt.Errorf("pacing: exhausted %d retries: %w", b.MaxRetries, last)
}
