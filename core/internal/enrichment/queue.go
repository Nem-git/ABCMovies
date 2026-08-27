// Enrichment work queue (TECHNICAL-DECISIONS §1.28): producers enqueue
// entry IDs; one paced worker drains. The seam mirrors the event-bus
// pattern — a small interface, an in-memory implementation first, injected
// at composition time — so heavier backends (durable SQLite, Kafka) swap in
// without producers, engine, or worker ever learning which transport runs.
package enrichment

import (
	"sync"
)

// Queue holds pending enrichment work. Enqueue is idempotent: an entry
// already pending is coalesced, not duplicated. Order is FIFO among first
// enqueues.
type Queue interface {
	Enqueue(entryID string)
	// TryNext pops the oldest pending entry, reporting ok=false when the
	// queue is empty.
	TryNext() (entryID string, ok bool)
}

// InMemoryQueue is the default Queue implementation. It is ephemeral by
// design: T1 re-marks misses on every derived-library rebuild and T2
// re-enqueues on every sync, so a lost queue self-heals regardless of what
// a future durable backend would guarantee.
type InMemoryQueue struct {
	mu      sync.Mutex
	pending []string
	seen    map[string]struct{}
}

// NewInMemoryQueue returns an empty queue.
func NewInMemoryQueue() *InMemoryQueue {
	return &InMemoryQueue{seen: map[string]struct{}{}}
}

func (q *InMemoryQueue) Enqueue(entryID string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, dup := q.seen[entryID]; dup {
		return // coalesce
	}
	q.seen[entryID] = struct{}{}
	q.pending = append(q.pending, entryID)
}

func (q *InMemoryQueue) TryNext() (string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) == 0 {
		return "", false
	}
	id := q.pending[0]
	q.pending = q.pending[1:]
	delete(q.seen, id)
	return id, true
}

// Len reports the backlog size; exposed for tests and observability.
func (q *InMemoryQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.pending)
}

// Pending returns a snapshot of the entry IDs currently awaiting enrichment,
// in FIFO order. It is a read-only observability surface; Enqueue preserves
// the worker discipline.
func (q *InMemoryQueue) Pending() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]string(nil), q.pending...)
}
