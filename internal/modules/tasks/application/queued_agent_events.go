package application

import (
	"context"
	"log/slog"
	"sync"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
)

// DefaultEventQueueCapacity is the number of events accepted before the runtime waits.
const DefaultEventQueueCapacity = 4096

// queueItem is one event to process, or a flush marker (flushed non-nil) that is answered once every
// event queued before it has been processed.
type queueItem struct {
	taskID  models.TaskID
	event   models.AgentEvent
	ctx     context.Context
	flushed chan struct{}
}

// QueuedAgentEventSink decouples the readers of the agent processes from the database. Handle only
// enqueues; one worker goroutine drains the queue in FIFO order, which keeps the order of each task.
//
// When the queue is full, Handle waits (backpressure): no event is dropped and memory stays bounded.
// A panic while processing one event loses only that event. Events are processed with a context that
// ignores cancellation of the caller, so a runtime that stops does not abort the writes of its last events.
// After Shutdown, Handle processes the event inline so that nothing is lost during the stop.
//
// The worker goroutine is owned by this value: Shutdown waits for it. Construct one per application.
type QueuedAgentEventSink struct {
	inner inbound.AgentEventSink
	items chan queueItem
	done  chan struct{}

	// mu guards closed and the close of items; Handle holds it for reading while it enqueues.
	mu     sync.RWMutex
	closed bool
}

// NewQueuedAgentEventSink starts the worker over inner with a queue of the given capacity (at least 1).
func NewQueuedAgentEventSink(inner inbound.AgentEventSink, capacity int) *QueuedAgentEventSink {
	sink := &QueuedAgentEventSink{
		inner: inner,
		items: make(chan queueItem, max(capacity, 1)),
		done:  make(chan struct{}),
	}
	go sink.work()
	return sink
}

// Handle enqueues the event. It waits only while the queue is full.
func (s *QueuedAgentEventSink) Handle(ctx context.Context, taskID models.TaskID, event models.AgentEvent) {
	item := queueItem{taskID: taskID, event: event, ctx: context.WithoutCancel(ctx)}
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		s.process(item)
		return
	}
	s.items <- item
	s.mu.RUnlock()
}

// Flush waits until every event accepted so far has been processed. After Shutdown it returns at once,
// because those events were already processed inline.
func (s *QueuedAgentEventSink) Flush() {
	flushed := make(chan struct{})
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return
	}
	s.items <- queueItem{flushed: flushed}
	s.mu.RUnlock()
	<-flushed
}

// Shutdown stops accepting events in the queue, processes everything already queued and waits for the
// worker to finish. It is safe to call more than once.
func (s *QueuedAgentEventSink) Shutdown() {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		close(s.items)
	}
	s.mu.Unlock()
	<-s.done
}

// work drains the queue until Shutdown closes it. Items queued before the close are still processed.
func (s *QueuedAgentEventSink) work() {
	defer close(s.done)
	for item := range s.items {
		if item.flushed != nil {
			close(item.flushed)
			continue
		}
		s.process(item)
	}
}

// process hands one event to the inner sink. A panic is recovered and logged, so it cannot stop the worker.
func (s *QueuedAgentEventSink) process(item queueItem) {
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.Error("processing an agent event failed", "task_id", item.taskID.String(), "panic", recovered)
		}
	}()
	s.inner.Handle(item.ctx, item.taskID, item.event)
}
