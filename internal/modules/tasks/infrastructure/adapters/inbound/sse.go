// Package inbound holds the transports of the tasks module. The handlers here are not mounted: the
// composition root mounts them under the routes below, next to the other modules.
package inbound

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// Route of the live stream.
const (
	// BoardEventsRoute is the stream of the page, the only one it opens: one "changed" or "removed" notice per task
	// that the board must refetch, and one "activity" notice per event of any task. Everything the page shows shares
	// it, because a browser allows only a few connections per host and every extra stream takes one for good.
	BoardEventsRoute = "/api/tasks/events"
)

// Names of the events of the protocol. Each one is also its own data, because browsers drop events without data.
const (
	resetEvent   = "reset"
	laggedEvent  = "lagged"
	changedEvent = "changed"
	removedEvent = "removed"
	// activityEvent announces that a task has a new event, whatever it is: the open transcripts refetch on it.
	activityEvent = "activity"
)

// TaskStreams serves the page stream. It starts with "reset", which also tells a reconnecting client to refetch what
// it shows, because the notices sent while it was away are not replayed. A subscriber that falls behind gets "lagged"
// and the stream ends: the client reconnects and refetches, so nothing is lost.
type TaskStreams struct {
	subscriber outbound.TaskEventSubscriber
}

// NewTaskStreams builds the handlers over the bus.
func NewTaskStreams(subscriber outbound.TaskEventSubscriber) *TaskStreams {
	return &TaskStreams{subscriber: subscriber}
}

// BoardEvents serves BoardEventsRoute. It announces the tasks that changed, so the board refetches them.
func (s *TaskStreams) BoardEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	subscription := s.subscriber.Subscribe(ctx)
	defer subscription.Close()
	stream, ok := web.NewSSE(w)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}
	if err := stream.Send(ctx, "", resetEvent, resetEvent); err != nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case event, open := <-subscription.Events():
			if !open {
				sendLaggedIfOverflowed(ctx, stream, subscription)
				return
			}
			if err := sendBoardNotices(ctx, stream, event); err != nil {
				return
			}
		}
	}
}

// sendBoardNotices writes the notices an event causes: the board one, when the board shows it, then "activity",
// except for a removal, which leaves nothing to refetch.
func sendBoardNotices(ctx context.Context, stream *web.SSE, event models.TaskEvent) error {
	taskID := event.TaskID.String()
	name, relevant := boardEventName(event.Kind)
	if relevant {
		if err := stream.Send(ctx, "", name, taskID); err != nil {
			return fmt.Errorf("send board notice: %w", err)
		}
	}
	if name == removedEvent {
		return nil
	}
	if err := stream.Send(ctx, "", activityEvent, taskID); err != nil {
		return fmt.Errorf("send activity notice: %w", err)
	}
	return nil
}

// boardEventName maps an event to the board notice it causes. The boolean is false for events the board
// does not show.
func boardEventName(kind models.TaskEventKind) (string, bool) {
	switch typed := kind.(type) {
	case models.StatusEventKind, models.ChangedEventKind:
		return changedEvent, true
	case models.RemovedEventKind:
		return removedEvent, true
	case models.AgentEventKind:
		switch typed.Event.(type) {
		case models.DialogRequested, models.DialogResolved, models.ProcessExited, models.SubagentsChanged:
			return changedEvent, true
		}
	}
	return "", false
}

// sendLaggedIfOverflowed tells an overflowed subscriber to reconnect. A normal stop sends nothing.
func sendLaggedIfOverflowed(ctx context.Context, stream *web.SSE, subscription outbound.TaskEventSubscription) {
	if !subscription.Overflowed() {
		return
	}
	if err := stream.Send(ctx, "", laggedEvent, laggedEvent); err != nil {
		slog.Debug("could not tell a lagging stream to reconnect", "error", err)
	}
}
