// Package events is the in-memory bus that broadcasts the task events to the live observers.
package events

import (
	"context"
	"sync"
	"sync/atomic"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// DefaultCapacity is the buffer of each subscriber. A subscriber that falls this far behind is
// disconnected: it must resynchronize from the persisted history, which keeps the publishers fast.
const DefaultCapacity = 4096

// EventBus fans the published events out to the subscribers. Publish never blocks: a subscriber whose
// buffer is full is disconnected (its channel closes and Overflowed reports it) instead of slowing the
// publishers and the other subscribers down.
type EventBus struct {
	capacity int

	mu          sync.Mutex
	subscribers map[*subscriber]struct{}
}

var (
	_ outbound.TaskEventPublisher  = (*EventBus)(nil)
	_ outbound.TaskEventSubscriber = (*EventBus)(nil)
)

type subscriber struct {
	events     chan models.TaskEvent
	overflowed atomic.Bool
}

// NewEventBus builds a bus whose subscribers buffer capacity events each (at least 1).
func NewEventBus(capacity int) *EventBus {
	return &EventBus{
		capacity:    max(capacity, 1),
		subscribers: make(map[*subscriber]struct{}),
	}
}

// Publish delivers the event to every subscriber that has room for it. Events without a subscriber are
// dropped: the live bus is not a log, the history is persisted elsewhere.
func (b *EventBus) Publish(event models.TaskEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for sub := range b.subscribers {
		select {
		case sub.events <- event:
		default:
			sub.overflowed.Store(true)
			close(sub.events)
			delete(b.subscribers, sub)
		}
	}
}

// Subscribe registers an observer. Its subscription ends when ctx is done, or when Close is called.
// No goroutine is kept per subscriber: the cleanup runs as a context callback.
func (b *EventBus) Subscribe(ctx context.Context) outbound.TaskEventSubscription {
	sub := &subscriber{events: make(chan models.TaskEvent, b.capacity)}
	b.mu.Lock()
	b.subscribers[sub] = struct{}{}
	b.mu.Unlock()
	subscription := &subscription{bus: b, sub: sub}
	context.AfterFunc(ctx, subscription.Close)
	return subscription
}

// remove ends a subscription. It is idempotent: a subscriber already removed by an overflow is left alone.
func (b *EventBus) remove(sub *subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.subscribers[sub]; ok {
		delete(b.subscribers, sub)
		close(sub.events)
	}
}

// subscription is the observer side of one subscriber.
type subscription struct {
	bus *EventBus
	sub *subscriber
}

// Events yields the events published after the subscription started. It is closed when the subscription ends.
func (s *subscription) Events() <-chan models.TaskEvent {
	return s.sub.events
}

// Overflowed reports whether the bus disconnected this subscriber because it fell behind.
func (s *subscription) Overflowed() bool {
	return s.sub.overflowed.Load()
}

// Close ends the subscription. It is safe to call more than once and after the context ended.
func (s *subscription) Close() {
	s.bus.remove(s.sub)
}
