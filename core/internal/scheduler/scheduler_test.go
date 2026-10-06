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

func TestLateRegisteredJobRuns(t *testing.T) {
	s := New(20*time.Millisecond, slog.Default())
	var mu sync.Mutex
	bootFired, lateFired := 0, 0
	_ = s.Register(Job{Name: "boot", Run: func(context.Context) error {
		mu.Lock()
		bootFired++
		mu.Unlock()
		return nil
	}})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()

	// Let the boot job start, then register one more mid-run: it must begin
	// firing without a restart or a second Run call.
	time.Sleep(60 * time.Millisecond)
	_ = s.Register(Job{Name: "late", Run: func(context.Context) error {
		mu.Lock()
		lateFired++
		mu.Unlock()
		return nil
	}})
	time.Sleep(80 * time.Millisecond)
	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()
	if bootFired < 1 {
		t.Fatalf("boot job never fired (%d)", bootFired)
	}
	if lateFired < 1 {
		t.Fatalf("late-registered job never fired (%d)", lateFired)
	}
}

func TestRemoveStopsALateRegisteredJob(t *testing.T) {
	s := New(20*time.Millisecond, slog.Default())
	var mu sync.Mutex
	lateFired := 0
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()

	late := Job{Name: "late", Run: func(context.Context) error {
		mu.Lock()
		lateFired++
		mu.Unlock()
		return nil
	}}
	time.Sleep(30 * time.Millisecond)
	_ = s.Register(late)
	time.Sleep(60 * time.Millisecond)
	s.Remove("late")
	before := 0
	mu.Lock()
	before = lateFired
	mu.Unlock()
	time.Sleep(60 * time.Millisecond)
	mu.Lock()
	after := lateFired
	mu.Unlock()
	if before == 0 {
		t.Fatalf("late job never fired before removal")
	}
	if after > before {
		t.Fatalf("job fired %d times after removal (was %d)", after, before)
	}
	cancel()
	<-done
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
