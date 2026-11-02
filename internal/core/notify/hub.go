// Package notify is a tiny in-process broadcast of named notices. A module publishes that something
// changed and the live page stream tells every open page to refetch it; the notice carries no data.
//
// To add a notice: publish its name with Hub.Publish from the module that owns the change, and make
// the view that shows it listen with hx-trigger="sse:<name>, sse:reset". The page stream forwards
// every published name as an SSE event of the same name; "reset" covers the notices missed while the
// stream was down.
package notify

import "sync"

// Hub fans a published notice out to every subscriber. The zero value is not usable; use NewHub.
type Hub struct {
	mu   sync.Mutex
	subs map[*Subscription]struct{}
}

// NewHub builds an empty hub.
func NewHub() *Hub {
	return &Hub{subs: make(map[*Subscription]struct{})}
}

// Subscription is one observer. Notices are idempotent ("refetch"), so repeated ones of the same name
// coalesce: the pending set holds each name once, which bounds memory and never loses a distinct name.
type Subscription struct {
	mu      sync.Mutex
	pending []string
	ready   chan struct{}
}

// Ready signals that Drain has notices. It never closes; stop reading when you call Cancel.
func (s *Subscription) Ready() <-chan struct{} { return s.ready }

// Drain returns the pending notice names in publication order and clears them.
func (s *Subscription) Drain() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	names := s.pending
	s.pending = nil
	return names
}

func (s *Subscription) push(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, queued := range s.pending {
		if queued == name {
			return
		}
	}
	s.pending = append(s.pending, name)
	select {
	case s.ready <- struct{}{}:
	default:
	}
}

// Publish sends name to every subscriber without blocking. A nil hub ignores it.
func (h *Hub) Publish(name string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for sub := range h.subs {
		sub.push(name)
	}
}

// Subscribe registers an observer and returns it with the function that ends the subscription.
func (h *Hub) Subscribe() (*Subscription, func()) {
	sub := &Subscription{ready: make(chan struct{}, 1)}
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
