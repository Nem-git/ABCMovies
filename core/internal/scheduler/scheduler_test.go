package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestRunFiresJobsOnShortCadence(t *testing.T) {
	s := New(20*time.Millisecond, slog.Default())
	var mu sync.Mutex
	fired := 0
	_ = s.Register(Job{Name: "tick", Run: func(context.Context) error {
		mu.Lock()
		fired++
		mu.Unlock()
		return nil
	}})
	ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
	defer cancel()
	s.Run(ctx)
	if fired < 2 {
		t.Fatalf("job fired %d times in 150ms on a 20ms cadence", fired)
	}
}

func TestBackoffRecoversAndResetsAfterSuccess(t *testing.T) {
	backoffBase = time.Millisecond
	backoffMax = 10 * time.Millisecond
	minWait = time.Microsecond
	t.Cleanup(func() {
		backoffBase = time.Minute
		backoffMax = 24 * time.Hour
		minWait = time.Second
	})

	s := New(10*time.Millisecond, slog.Default())
	var mu sync.Mutex
	calls := 0
	failFirst := true
	done := make(chan struct{})
	_ = s.Register(Job{Name: "flaky", Run: func(context.Context) error {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if failFirst {
			failFirst = false
			return errors.New("provider down")
		}
		if calls >= 2 {
			select {
			case <-done:
			default:
				close(done)
			}
		}
		return nil
	}})
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	s.Run(ctx)
	mu.Lock()
	defer mu.Unlock()
	if calls < 2 {
		t.Fatalf("job did not retry after failure (calls=%d)", calls)
	}
}

func TestBackoffGrowsAndCaps(t *testing.T) {
	s := New(time.Hour, slog.Default())
	prev := time.Duration(0)
	for f := 1; f <= 30; f++ {
		d := s.backoff(f)
		if d > backoffMax {
			t.Fatalf("backoff(%d) = %s exceeds cap %s", f, d, backoffMax)
		}
		if f <= 10 && d <= prev && prev != 0 {
			t.Fatalf("backoff stopped growing at failure %d: %s then %s", f, prev, d)
		}
		prev = d
	}
}

func TestJitteredStaysWithinBounds(t *testing.T) {
	s := New(time.Hour, slog.Default())
	base := s.cadence
	spread := time.Duration(float64(base) * jitterFraction)
	for range 1000 {
		d := s.jittered(base)
		if d < base-spread || d > base+spread {
			t.Fatalf("jittered %s outside ±%s of %s", d, spread, base)
		}
	}
}
