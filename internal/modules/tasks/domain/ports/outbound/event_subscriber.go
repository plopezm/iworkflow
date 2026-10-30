package outbound

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
)

// TaskEventSubscriber gives the live observers (the SSE streams) a channel of the published events.
// Implemented by the event bus; the transports depend on this contract only.
type TaskEventSubscriber interface {
	// Subscribe registers an observer until the context is done or the subscription is closed.
	Subscribe(ctx context.Context) TaskEventSubscription
}

// TaskEventSubscription is one observer of the live events.
//
// Events is closed when the subscription ends. It ends for one of two reasons: Close (or the context
// ending) which is a normal stop, or the observer falling behind the bounded buffer, which Overflowed
// reports. An overflowed observer has lost events and must resynchronize from the persisted history.
type TaskEventSubscription interface {
	Events() <-chan models.TaskEvent
	Overflowed() bool
	Close()
}
