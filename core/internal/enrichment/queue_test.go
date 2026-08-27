package enrichment

import (
	"reflect"
	"testing"
)

func TestInMemoryQueuePendingSnapshot(t *testing.T) {
	q := NewInMemoryQueue()
	if got := q.Pending(); len(got) != 0 {
		t.Fatalf("fresh queue Pending = %v, want empty", got)
	}

	q.Enqueue("le_a")
	q.Enqueue("le_b")
	q.Enqueue("le_a") // coalesced, not duplicated

	if got := q.Pending(); !reflect.DeepEqual(got, []string{"le_a", "le_b"}) {
		t.Fatalf("Pending = %v, want FIFO [le_a le_b]", got)
	}

	// The snapshot must not alias the live queue: popping the queued head
	// must not have been a no-op because the caller mutated the slice.
	got := q.Pending()
	got[0] = "tampered"
	if live := q.Pending(); live[0] != "le_a" {
		t.Fatalf("Pending snapshot aliased the queue: %v", live)
	}

	if _, ok := q.TryNext(); !ok {
		t.Fatal("TryNext on non-empty queue returned nothing")
	}
	if got := q.Pending(); !reflect.DeepEqual(got, []string{"le_b"}) {
		t.Fatalf("Pending after pop = %v, want [le_b]", got)
	}
}
