package delivery

import (
	"sync"
)

// ForegroundGate lets background provider rounds (catalogue sync, availability
// refresh, future metadata pulls) yield the provider's funds to foreground
// traffic: a round is admitted only when no live session owns the same
// account, and a second round on the same account waits, never overlaps. It
// is the v1-cut of PLAN.md §7.2's shared pacing budget — fine-grained,
// per-request budgets land in M7; this is the honest whole-round yield.
//
// The engine is built after the slot wiring, so the gate stores its engine
// pointer through Bind the way slots.go's event mux stores the invalidator.
// Before Bind, every round is admitted: the core has no foreground yet.
type ForegroundGate struct {
	mu   sync.Mutex
	eng  *Engine
	busy map[accountKey]struct{}
}

// NewForegroundGate is ready for wiring, unbound.
func NewForegroundGate() *ForegroundGate {
	return &ForegroundGate{busy: map[accountKey]struct{}{}}
}

// Bind attaches the engine whose live sessions gate rounds. Previously
// admitted rounds are unaffected; future Admits see the current sessions.
func (g *ForegroundGate) Bind(eng *Engine) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.eng = eng
}

// Admit holds a background round for (provider, accountID). When allowed is
// false the round must be deferred, not retried immediately — retrying is the
// caller's cadence decision (e.g. the sync scheduler's next tick). The
// release func must be called when the round ends, success or failure.
func (g *ForegroundGate) Admit(provider, accountID string) (release func(), allowed bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	key := accountKey{provider: provider, accountID: accountID}
	if _, held := g.busy[key]; held {
		return nil, false
	}
	if g.eng != nil && g.eng.AccountBusy(provider, accountID) {
		return nil, false
	}
	g.busy[key] = struct{}{}
	return func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		delete(g.busy, key)
	}, true
}

// AccountBusy reports whether any live session (queued or running) is routed
// through (provider, accountID), so a background round can defer rather than
// compound on the account's capacity.
func (e *Engine) AccountBusy(provider, accountID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, s := range e.sessions {
		if s.isActive() && s.Context.GetProvider() == provider && s.Context.GetAccountId() == accountID {
			return true
		}
	}
	return false
}
