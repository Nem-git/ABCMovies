package apiserver

import (
	"sync"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

// Bus defines the event bus operations used by the API layer. Subscribers
// register under their authenticated identity: delivery is an authorization
// boundary at the bus, not a convenience flag (PLAN.md §9.2) — a subscriber
// only ever receives events whose audience includes them.
type Bus interface {
	// Subscribe registers a subscriber identified by id, entitled to events
	// addressed to uid, and returns a channel that receives routed events.
	Subscribe(id string, uid string) <-chan *corev1.EventEnvelope

	// Unsubscribe removes a subscriber. Safe to call with an unknown id.
	Unsubscribe(id string)

	// Publish routes an event to the subscribers its audience entitles.
	Publish(event *corev1.EventEnvelope)
}

type busSubscriber struct {
	uid string
	ch  chan *corev1.EventEnvelope
}

// InMemoryBus is an in-memory event bus. It is ephemeral: a lost notification
// is free to lose (PLAN.md §2.4).
type InMemoryBus struct {
	mu          sync.Mutex
	subscribers map[string]busSubscriber
}

// NewInMemoryBus returns a new, empty event bus.
func NewInMemoryBus() *InMemoryBus {
	return &InMemoryBus{subscribers: make(map[string]busSubscriber)}
}

// Publish routes the event to every subscriber entitled to it (PLAN.md §9.2):
//
//   - USER and OWNER audiences carry a user_id and are delivered only to that
//     user's subscriptions;
//   - the ACCOUNT audience is account-scoped fan-out; no M0 emitter produces
//     it, and until fan-out exists it is delivered to nobody.
//
// Delivery is best-effort: the channel is buffered, and a slow subscriber
// misses events rather than blocking the publisher.
func (b *InMemoryBus) Publish(event *corev1.EventEnvelope) {
	if event == nil {
		return
	}
	var uid string
	switch event.GetAudience() {
	case corev1.EventAudience_EVENT_AUDIENCE_USER, corev1.EventAudience_EVENT_AUDIENCE_OWNER:
		uid = event.GetUserId()
	default:
		return
	}
	if uid == "" {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	for _, sub := range b.subscribers {
		if sub.uid != uid {
			continue
		}
		select {
		case sub.ch <- event:
		default:
		}
	}
}

// Subscribe registers a subscriber and returns a channel that receives events.
// The channel is buffered; slow subscribers miss events rather than block the
// publisher.
func (b *InMemoryBus) Subscribe(id string, uid string) <-chan *corev1.EventEnvelope {
	ch := make(chan *corev1.EventEnvelope, 64)
	b.mu.Lock()
	b.subscribers[id] = busSubscriber{uid: uid, ch: ch}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber. Safe to call with an unknown id.
func (b *InMemoryBus) Unsubscribe(id string) {
	b.mu.Lock()
	if sub, ok := b.subscribers[id]; ok {
		close(sub.ch)
		delete(b.subscribers, id)
	}
	b.mu.Unlock()
}

// Close removes all subscribers.
func (b *InMemoryBus) Close() {
	b.mu.Lock()
	for id, sub := range b.subscribers {
		close(sub.ch)
		delete(b.subscribers, id)
	}
	b.mu.Unlock()
}
