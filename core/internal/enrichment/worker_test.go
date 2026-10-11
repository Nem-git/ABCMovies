package enrichment

import (
	"context"
	"testing"
	"time"
)

// One pacing system: the drain worker applies no pacing of its own — each
// catalogue's declared pace is enforced where that catalogue is called (the
// core's declared-pace wrapper), so a tick of fast, cache-local work completes
// as fast as it runs. With the retired per-entry sleep this took 200ms per
// entry (600ms for three), so the threshold fails on the old behavior and
// leaves three stub enrichments four orders of magnitude of headroom.
func TestDrainAppliesNoPacingOfItsOwn(t *testing.T) {
	q := NewInMemoryQueue()
	for _, id := range []string{"a", "b", "c"} {
		q.Enqueue(id)
	}
	var seen []string
	w := NewWorker(q, func(_ context.Context, id string) error {
		seen = append(seen, id)
		return nil
	}, nil)

	start := time.Now()
	if err := w.Job(0).Run(context.Background()); err != nil {
		t.Fatalf("drain: %v", err)
	}
	elapsed := time.Since(start)

	if len(seen) != 3 {
		t.Fatalf("enriched %v, want 3 entries", seen)
	}
	if elapsed >= 200*time.Millisecond {
		t.Fatalf("drain of 3 stub entries took %v; the worker must not sleep between entries (the catalogue wrapper is the one pacing system)", elapsed)
	}
}
