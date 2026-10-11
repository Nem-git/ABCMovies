package pacing

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// A governor shared by every account of one provider bounds the total
// requests-per-second the provider sees, whatever the per-account budgets are.
func TestGovernorBoundsAggregate(t *testing.T) {
	g := NewGovernor(100, 10*time.Millisecond, 100*time.Millisecond, nil) // 10 ms
	l := NewLimiter(Budget{}, nil)                                        // account unlimited
	ctx := context.Background()
	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 2; j++ {
				if err := Gate(ctx, l, g, Budget{}, func(context.Context) error { return nil }); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
	if el := time.Since(start); el < 40*time.Millisecond {
		t.Fatalf("6 requests through a 100 rps governor took %v, want >= ~50ms", el)
	}
}

// A per-provider error is a per-provider signal: one throttled account puts
// every account's next request into backoff (§5.4).
func TestProviderBackoffIsShared(t *testing.T) {
	g := NewGovernor(1e9, 50*time.Millisecond, time.Second, nil)
	l := NewLimiter(Budget{}, nil)
	ctx := context.Background()
	calls := atomic.Int32{}
	_ = Gate(ctx, l, g, Budget{MaxRetries: 0}, func(context.Context) error {
		calls.Add(1)
		return errors.New("429")
	})
	if calls.Load() != 1 {
		t.Fatalf("calls = %d", calls.Load())
	}
	err := Gate(ctx, l, g, Budget{MaxRetries: 0}, func(context.Context) error {
		calls.Add(1)
		return nil
	})
	if !errors.Is(err, ErrDegraded) {
		t.Fatalf("second request during backoff = %v, want ErrDegraded", err)
	}
	g.NoteSuccess()
	if err := Gate(ctx, l, g, Budget{}, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
}

// A degraded provider fails fast — and the busy answer carries no position
// hint. In v1 there is no queue behind the refusal, so a rank would be
// fiction (PLAN.md §11: "a refusal never carries a queue position"; the
// future admission mode reports position as tracked job state instead).
// The message must be the plain sentinel: any suffix like "(position N)" is
// a regression, wherever that text would surface — logs or an RPC status.
func TestDegradedBusyAnswerCarriesNoPosition(t *testing.T) {
	g := NewGovernor(1e9, time.Minute, time.Minute, nil)
	g.NoteError()
	err := Gate(context.Background(), NewLimiter(Budget{}, nil), g, Budget{MaxRetries: 0}, func(context.Context) error {
		return nil
	})
	if !errors.Is(err, ErrDegraded) {
		t.Fatalf("err = %v, want ErrDegraded", err)
	}
	var be *BusyError
	if !errors.As(err, &be) {
		t.Fatalf("err = %T %v, want a pacing BusyError", err, err)
	}
	if strings.Contains(err.Error(), "position") {
		t.Fatalf("busy answer leaks a position hint: %q", err.Error())
	}
	if err.Error() != ErrDegraded.Error() {
		t.Fatalf("busy answer text = %q, want the plain sentinel %q", err.Error(), ErrDegraded.Error())
	}
}

// The account budget and the provider governor compose: background work can
// never exceed the limits user traffic obeys, because both draw on the same
// per-account Limiter (§7.2).
func TestBackgroundSharesTheAccountBudget(t *testing.T) {
	l := NewLimiter(Budget{MinSpacing: 30 * time.Millisecond}, nil)
	g := NewGovernor(1e9, time.Millisecond, time.Second, nil)
	ctx := context.Background()
	start := time.Now()
	// One user request, one background refresh: both must pay the spacing.
	_ = Gate(ctx, l, g, Budget{}, func(context.Context) error { return nil })
	_ = Gate(ctx, l, g, Budget{}, func(context.Context) error { return nil })
	if el := time.Since(start); el < 30*time.Millisecond {
		t.Fatalf("two calls with 30ms spacing took %v", el)
	}
}

// Retries are bounded by the budget: an account that keeps failing does not
// get unbounded attempts.
func TestRetriesAreBounded(t *testing.T) {
	l := NewLimiter(Budget{}, nil)
	g := NewGovernor(1e9, 0, 0, nil) // no backoff: retry bound is observable in isolation
	calls := atomic.Int32{}
	err := Gate(context.Background(), l, g, Budget{MaxRetries: 2}, func(context.Context) error {
		calls.Add(1)
		return errors.New("boom")
	})
	if err == nil || calls.Load() != 3 {
		t.Fatalf("calls = %d err = %v, want 3 attempts then failure", calls.Load(), err)
	}
}
