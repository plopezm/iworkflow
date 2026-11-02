// Package notify is a tiny in-process broadcast of named notices. Modules publish that something
// changed and the live stream tells every open page to refetch it; the notice carries no data.
package notify

import "sync"

// subscriberBuffer is how many notices a subscriber may hold. Notices are idempotent ("refetch"),
// so a full buffer drops the new one: the pending notice already makes the page refetch.
const subscriberBuffer = 8

// Hub fans a published notice out to every subscriber. The zero value is not usable; use NewHub.
type Hub struct {
	mu   sync.Mutex
	subs map[chan string]struct{}
}

// NewHub builds an empty hub.
func NewHub() *Hub {
	return &Hub{subs: make(map[chan string]struct{})}
}

// Publish sends name to every subscriber without blocking. A nil hub ignores it.
func (h *Hub) Publish(name string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for sub := range h.subs {
		select {
		case sub <- name:
		default:
		}
	}
}

// Subscribe returns the channel of notices and the function that ends the subscription. The channel
// is never closed: the owner stops reading when it calls cancel.
func (h *Hub) Subscribe() (<-chan string, func()) {
	sub := make(chan string, subscriberBuffer)
	if h == nil {
		return sub, func() {}
	}
	h.mu.Lock()
	h.subs[sub] = struct{}{}
	h.mu.Unlock()
	return sub, func() {
		h.mu.Lock()
		delete(h.subs, sub)
		h.mu.Unlock()
	}
}
