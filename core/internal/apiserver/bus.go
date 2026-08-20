package apiserver

import (
	"sync"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

// Bus defines the event bus operations used by the API layer.
type Bus interface {
	Subscribe(id string) <-chan *corev1.EventEnvelope
	Unsubscribe(id string)
}

// InMemoryBus is an in-memory event bus. It is ephemeral: a lost notification
// is free to lose (PLAN.md §2.4).
type InMemoryBus struct {
	mu          sync.Mutex
	subscribers map[string]chan *corev1.EventEnvelope
}

// NewInMemoryBus returns a new, empty event bus.
func NewInMemoryBus() *InMemoryBus {
	return &InMemoryBus{subscribers: make(map[string]chan *corev1.EventEnvelope)}
}

// Publish sends an event to all current subscribers.
func (b *InMemoryBus) Publish(event *corev1.EventEnvelope) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

// Subscribe registers a subscriber and returns a channel that receives events.
// The channel is buffered; slow subscribers miss events rather than block the
// publisher.
func (b *InMemoryBus) Subscribe(id string) <-chan *corev1.EventEnvelope {
	ch := make(chan *corev1.EventEnvelope, 64)
	b.mu.Lock()
	b.subscribers[id] = ch
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber. Safe to call with an unknown id.
func (b *InMemoryBus) Unsubscribe(id string) {
	b.mu.Lock()
	if ch, ok := b.subscribers[id]; ok {
		close(ch)
		delete(b.subscribers, id)
	}
	b.mu.Unlock()
}

// Close removes all subscribers.
func (b *InMemoryBus) Close() {
	b.mu.Lock()
	for id, ch := range b.subscribers {
		close(ch)
		delete(b.subscribers, id)
	}
	b.mu.Unlock()
}
