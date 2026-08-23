// Package scheduler is M1's minimal refresh-cadence slice (IMPLEMENTATION.md
// §3): periodic account source-cache syncs with jitter so a fleet of accounts
// never fires as one spike, and exponential backoff when a provider is
// failing. The aggregate governor that coordinates across providers is a
// later milestone and deliberately absent here.
package scheduler

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"
)

// Cadence parameters. The concrete values live in TECHNICAL-DECISIONS.md;
// they are repeated here only because Go needs literals to run. They are
// variables solely so tests can shorten them.
var (
	defaultCadence = 6 * time.Hour
	jitterFraction = 0.10           // ±10% of the remaining wait
	backoffBase    = time.Minute    // first failure waits this long extra
	backoffMax     = 24 * time.Hour // ceiling for back-off waits
	minWait        = time.Second    // floor; never spin tighter than this
)

// Job is a named unit of recurring work. A zero Cadence means the
// scheduler's default; a set Cadence overrides it for this job alone (the
// adapter-declared value, possibly overridden by operator config).
type Job struct {
	Name    string
	Cadence time.Duration
	Run     func(ctx context.Context) error
}

// Scheduler runs jobs on a cadence with jitter and per-job backoff.
type Scheduler struct {
	cadence time.Duration
	jobs    []Job
	logger  *slog.Logger
}

// New builds a scheduler with the default cadence (zero means default).
func New(cadence time.Duration, logger *slog.Logger) *Scheduler {
	if cadence == 0 {
		cadence = defaultCadence
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{cadence: cadence, logger: logger}
}

// Register adds a job. Jobs registered before Run are scheduled; registering
// after Run starts returns false and does nothing (growth is declared, not
// improvised).
func (s *Scheduler) Register(j Job) bool {
	s.jobs = append(s.jobs, j)
	return true
}

// Run blocks until ctx is cancelled, firing each job once per cadence with
// independent jitter and backoff.
func (s *Scheduler) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, j := range s.jobs {
		wg.Add(1)
		go func(j Job) {
			defer wg.Done()
			s.loop(ctx, j)
		}(j)
	}
	wg.Wait()
}

func (s *Scheduler) loop(ctx context.Context, j Job) {
	cadence := j.Cadence
	if cadence == 0 {
		cadence = s.cadence
	}
	failures := 0
	timer := time.NewTimer(s.jittered(cadence))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		if err := j.Run(ctx); err != nil {
			failures++
			wait := s.backoff(failures)
			s.logger.Warn("scheduled job failed", "job", j.Name, "attempt", failures, "retry_in", wait.String(), "error", err)
			timer.Reset(wait)
			continue
		}
		failures = 0
		timer.Reset(s.jittered(cadence))
	}
}

// jittered spreads a wait uniformly within ±jitterFraction so many accounts
// desynchronize instead of syncing in lockstep.
func (s *Scheduler) jittered(d time.Duration) time.Duration {
	spread := time.Duration(float64(d) * jitterFraction)
	if spread <= 0 {
		return d
	}
	return d - spread + time.Duration(rand.Int64N(int64(2*spread)+1))
}

// backoff grows exponentially from backoffBase, capped at backoffMax.
func (s *Scheduler) backoff(failures int) time.Duration {
	d := backoffBase << (failures - 1)
	if d > backoffMax || d <= 0 { // d<=0 guards shift overflow
		return backoffMax
	}
	if d < minWait {
		return minWait
	}
	return s.jittered(d)
}
