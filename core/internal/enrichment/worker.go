package enrichment

import (
	"context"
	"log/slog"
	"time"

	"github.com/nem-git/abcmovies/core/internal/scheduler"
)

// paceInterval spaces catalogue calls inside one drain tick; it realizes
// §1.27's ~5 req/s ceiling for the serial worker.
const paceInterval = 200 * time.Millisecond

// Worker drains the enrichment queue as a scheduler job (TECHNICAL-DECISIONS
// §1.28): each tick processes every pending entry serially, paced, and never
// in a request path. Enrichment is best-effort — a failing entry logs and
// waits for the next T1/T2 marking to re-enqueue it.
type Worker struct {
	queue  Queue
	enrich func(ctx context.Context, entryID string) error
	logger *slog.Logger
}

// NewWorker builds the drain worker around an enrich callback. Returning an
// error from enrich marks that entry failed this tick; the worker keeps
// draining so one bad title cannot starve the queue.
func NewWorker(q Queue, enrich func(ctx context.Context, entryID string) error, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{queue: q, enrich: enrich, logger: logger}
}

// Job returns the scheduler job draining the queue at cadence. Cadence zero
// means the scheduler default.
func (w *Worker) Job(cadence time.Duration) scheduler.Job {
	return scheduler.Job{
		Name:    "enrichment-drain",
		Cadence: cadence,
		Run:     w.drain,
	}
}

func (w *Worker) drain(ctx context.Context) error {
	for {
		id, ok := w.queue.TryNext()
		if !ok {
			return nil
		}
		if err := ctx.Err(); err != nil {
			w.queue.Enqueue(id) // not processed — put it back for next tick
			return nil
		}
		if err := w.enrich(ctx, id); err != nil {
			w.logger.Warn("enrichment failed; will retry on next trigger",
				"entry", id, "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(paceInterval):
		}
	}
}
