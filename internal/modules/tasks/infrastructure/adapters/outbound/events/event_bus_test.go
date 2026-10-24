package events_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/events"
)

// received reads the next event or fails the test after a short wait.
func received(t *testing.T, events <-chan models.TaskEvent) models.TaskEvent {
	t.Helper()
	select {
	case event, ok := <-events:
		require.True(t, ok, "the subscription ended unexpectedly")
		return event
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for an event")
		return models.TaskEvent{}
	}
}

func runEvent(t *testing.T, kind models.AgentEvent) models.TaskEvent {
	t.Helper()
	id, err := models.NewTaskID()
	require.NoError(t, err)
	return models.EphemeralTaskEvent(id, models.AgentEventKind{Event: kind}, time.Now())
}

// Rust: broadcast_task_event_bus::subscribers_receive_published_events
func TestSubscribersReceivePublishedEvents(t *testing.T) {
	t.Parallel()
	bus := events.NewEventBus(events.DefaultCapacity)
	subscription := bus.Subscribe(context.Background())
	defer subscription.Close()
	event := runEvent(t, models.RunStarted{})

	bus.Publish(event)

	assert.Equal(t, event, received(t, subscription.Events()))
}

// Rust: broadcast_task_event_bus::publishing_without_subscribers_is_harmless
func TestPublishingWithoutSubscribersIsHarmless(t *testing.T) {
	t.Parallel()
	bus := events.NewEventBus(events.DefaultCapacity)

	bus.Publish(runEvent(t, models.RunSettled{}))
}

func TestCancelledContextEndsTheSubscriptionAndStopsDelivery(t *testing.T) {
	t.Parallel()
	bus := events.NewEventBus(events.DefaultCapacity)
	ctx, cancel := context.WithCancel(context.Background())
	subscription := bus.Subscribe(ctx)

	cancel()

	select {
	case _, ok := <-subscription.Events():
		assert.False(t, ok, "the channel must be closed")
	case <-time.After(5 * time.Second):
		t.Fatal("the channel was not closed after the context ended")
	}
	assert.False(t, subscription.Overflowed(), "a normal stop is not an overflow")
	bus.Publish(runEvent(t, models.RunStarted{}))
}

func TestCloseIsIdempotent(t *testing.T) {
	t.Parallel()
	bus := events.NewEventBus(events.DefaultCapacity)
	subscription := bus.Subscribe(context.Background())

	subscription.Close()
	subscription.Close()

	_, ok := <-subscription.Events()
	assert.False(t, ok)
}

// A subscriber that does not read is disconnected once its buffer is full. The publisher never waits
// for it, and a subscriber that keeps up still receives every event. The reader is consumed between
// publishes, so the outcome does not depend on scheduling.
func TestLaggingSubscriberIsDisconnectedWithoutBlockingOthers(t *testing.T) {
	t.Parallel()
	const capacity = 4
	const published = 3 * capacity
	bus := events.NewEventBus(capacity)
	slow := bus.Subscribe(context.Background())
	fast := bus.Subscribe(context.Background())
	defer fast.Close()

	started := time.Now()
	for index := range published {
		event := runEvent(t, models.RunStarted{})
		bus.Publish(event)
		assert.Equal(t, event, received(t, fast.Events()), "event %d", index)
	}

	assert.Less(t, time.Since(started), 5*time.Second, "publishing must not wait for a subscriber")
	assert.True(t, slow.Overflowed(), "the lagging subscriber must be disconnected")
	drained := 0
	for range slow.Events() {
		drained++
	}
	assert.Equal(t, capacity, drained, "the lagging subscriber keeps only what fitted in its buffer")
	assert.False(t, fast.Overflowed())
}

// Many publishers and subscribers at once; every subscriber with room sees each event exactly once.
// Run under -race: it checks the bus is safe to publish and subscribe from many goroutines.
func TestConcurrentPublishersAndSubscribersSeeEveryEvent(t *testing.T) {
	t.Parallel()
	const publishers = 8
	const perPublisher = 200
	const subscribers = 4
	bus := events.NewEventBus(publishers * perPublisher)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var readers sync.WaitGroup
	counts := make([]int, subscribers)
	for index := range subscribers {
		subscription := bus.Subscribe(ctx)
		readers.Add(1)
		go func() {
			defer readers.Done()
			for range subscription.Events() {
				counts[index]++
			}
		}()
		defer subscription.Close()
	}

	var writers sync.WaitGroup
	for range publishers {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for range perPublisher {
				bus.Publish(runEvent(t, models.RunStarted{}))
			}
		}()
	}
	writers.Wait()
	cancel()
	readers.Wait()

	for index, count := range counts {
		assert.Equal(t, publishers*perPublisher, count, "subscriber %d missed events", index)
	}
}

// Subscribing and cancelling while events are published must not race or leak a subscriber.
func TestSubscribeAndCancelWhilePublishing(t *testing.T) {
	t.Parallel()
	bus := events.NewEventBus(events.DefaultCapacity)
	stop := make(chan struct{})
	var publisher sync.WaitGroup
	publisher.Add(1)
	go func() {
		defer publisher.Done()
		for {
			select {
			case <-stop:
				return
			default:
				bus.Publish(runEvent(t, models.TextDelta{Delta: "x"}))
			}
		}
	}()

	for range 200 {
		ctx, cancel := context.WithCancel(context.Background())
		subscription := bus.Subscribe(ctx)
		cancel()
		for range subscription.Events() {
		}
	}
	close(stop)
	publisher.Wait()
}
