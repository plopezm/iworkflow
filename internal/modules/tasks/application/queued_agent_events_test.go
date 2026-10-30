package application_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
)

// recordingSink records every event it is handed, in arrival order. When gate is non-nil, Handle waits
// until gate is closed, which lets a test fill the queue behind a stalled consumer.
type recordingSink struct {
	mu       sync.Mutex
	gate     chan struct{}
	panicOn  string
	received []string
}

func (s *recordingSink) Handle(_ context.Context, _ models.TaskID, event models.AgentEvent) {
	if s.gate != nil {
		<-s.gate
	}
	text := event.(models.UserMessage).Text
	if text == s.panicOn {
		panic("boom on " + text)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.received = append(s.received, text)
}

func (s *recordingSink) texts() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.received...)
}

func message(text string) models.AgentEvent {
	return models.UserMessage{Text: text}
}

func expectedTexts(count int) []string {
	texts := make([]string, 0, count)
	for index := range count {
		texts = append(texts, fmt.Sprintf("m%03d", index))
	}
	return texts
}

// Rust: queued_agent_event_sink (FIFO per task, every accepted event is processed once)
func TestQueuedSinkProcessesEventsInFIFOOrder(t *testing.T) {
	t.Parallel()
	inner := &recordingSink{}
	sink := application.NewQueuedAgentEventSink(inner, 8)
	id := newTaskIDValue(t)
	want := expectedTexts(100)

	for _, text := range want {
		sink.Handle(context.Background(), id, message(text))
	}
	sink.Shutdown()

	assert.Equal(t, want, inner.texts())
}

// Rust: event_queue_resilience (the reader is not blocked by a slow consumer, and nothing is lost)
func TestQueuedSinkWaitsOnlyWhenTheQueueIsFullAndLosesNothing(t *testing.T) {
	t.Parallel()
	gate := make(chan struct{})
	inner := &recordingSink{gate: gate}
	const capacity = 64
	sink := application.NewQueuedAgentEventSink(inner, capacity)
	id := newTaskIDValue(t)
	count := capacity + 1 + 16
	want := expectedTexts(count)

	// The worker takes one event and blocks on the gate; the next `capacity` fill the queue. Enqueueing
	// the rest must then wait, so it runs in its own goroutine that we release after a while.
	enqueued := make(chan struct{})
	go func() {
		defer close(enqueued)
		for _, text := range want {
			sink.Handle(context.Background(), id, message(text))
		}
	}()
	time.Sleep(50 * time.Millisecond)
	select {
	case <-enqueued:
		t.Fatal("Handle returned while the queue was full and the worker was stalled")
	default:
	}
	close(gate)
	<-enqueued
	sink.Shutdown()

	assert.Equal(t, want, inner.texts())
}

// Rust: flush_events_persists_everything_accepted_before_returning
func TestQueuedSinkFlushWaitsForEverythingAcceptedBeforeIt(t *testing.T) {
	t.Parallel()
	inner := &recordingSink{}
	sink := application.NewQueuedAgentEventSink(inner, 4)
	id := newTaskIDValue(t)
	want := expectedTexts(40)
	for _, text := range want {
		sink.Handle(context.Background(), id, message(text))
	}

	sink.Flush()

	assert.Equal(t, want, inner.texts(), "flush returned with events still queued")
	sink.Shutdown()
}

func TestQueuedSinkShutdownProcessesWhatIsQueued(t *testing.T) {
	t.Parallel()
	gate := make(chan struct{})
	inner := &recordingSink{gate: gate}
	sink := application.NewQueuedAgentEventSink(inner, 16)
	id := newTaskIDValue(t)
	want := expectedTexts(10)
	for _, text := range want {
		sink.Handle(context.Background(), id, message(text))
	}

	go func() {
		time.Sleep(20 * time.Millisecond)
		close(gate)
	}()
	sink.Shutdown()

	assert.Equal(t, want, inner.texts())
}

// After Shutdown the events are processed inline, so a stop does not lose the events of a runtime that is
// still reporting.
func TestQueuedSinkProcessesInlineAfterShutdown(t *testing.T) {
	t.Parallel()
	inner := &recordingSink{}
	sink := application.NewQueuedAgentEventSink(inner, 4)
	sink.Shutdown()
	sink.Shutdown()
	id := newTaskIDValue(t)

	sink.Handle(context.Background(), id, message("tarde"))
	sink.Flush()

	assert.Equal(t, []string{"tarde"}, inner.texts())
}

// A panic while handling one event loses that event only; the next events are processed.
func TestQueuedSinkIsolatesPanicsToTheirEvent(t *testing.T) {
	t.Parallel()
	inner := &recordingSink{panicOn: "malo"}
	sink := application.NewQueuedAgentEventSink(inner, 8)
	id := newTaskIDValue(t)

	for _, text := range []string{"antes", "malo", "despues"} {
		sink.Handle(context.Background(), id, message(text))
	}
	sink.Shutdown()

	assert.Equal(t, []string{"antes", "despues"}, inner.texts())
}

// The inner sink sees a context that a cancelled caller does not cancel: the writes of the last events
// of a stopping runtime must still happen.
func TestQueuedSinkProcessesWithTheCallerContextCancelled(t *testing.T) {
	t.Parallel()
	seen := make(chan error, 1)
	inner := sinkFunc(func(ctx context.Context) { seen <- ctx.Err() })
	sink := application.NewQueuedAgentEventSink(inner, 4)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sink.Handle(ctx, newTaskIDValue(t), message("x"))
	sink.Shutdown()

	require.NoError(t, <-seen)
}

// sinkFunc adapts a function to the inbound sink contract.
type sinkFunc func(ctx context.Context)

func (f sinkFunc) Handle(ctx context.Context, _ models.TaskID, _ models.AgentEvent) {
	f(ctx)
}
