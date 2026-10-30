// Package inbound holds the transports of the tasks module. The handlers here are not mounted: the
// composition root mounts them under the routes below, next to the other modules.
package inbound

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// Routes of the live streams.
const (
	// TaskEventsRoute streams the history and the live events of one task.
	TaskEventsRoute = "/api/tasks/{id}/events"
	// BoardEventsRoute streams one "changed" or "removed" notice per task that the board must refetch.
	BoardEventsRoute = "/api/tasks/events"
)

// Names of the control events of the protocol. Each one is also its own data, because browsers drop
// events without data.
const (
	resetEvent   = "reset"
	laggedEvent  = "lagged"
	changedEvent = "changed"
	removedEvent = "removed"
	// lastEventIDHeader is the header a reconnecting EventSource sends with the last id it received.
	lastEventIDHeader = "Last-Event-ID"
	// afterQuery is the query alternative to lastEventIDHeader.
	afterQuery = "after"
)

// TaskStreams serves the task event streams. The protocol, as the Rust version defines it:
//
//   - A stream without a cursor starts with "reset". With a cursor (Last-Event-ID or ?after=) it does not,
//     because the client keeps what it already has, and only the events after the cursor are sent.
//   - Then the persisted history, each event with its sequence number as id, then the live events of the task.
//   - Events that are not persisted carry no id.
//   - A subscriber that falls behind gets "lagged" and the stream ends: the client reconnects and resumes
//     from its last id, so nothing is lost.
type TaskStreams struct {
	getTask    *application.GetTask
	getEvents  *application.GetTaskEvents
	subscriber outbound.TaskEventSubscriber
}

// NewTaskStreams builds the handlers over the use cases and the bus.
func NewTaskStreams(getTask *application.GetTask, getEvents *application.GetTaskEvents, subscriber outbound.TaskEventSubscriber) *TaskStreams {
	return &TaskStreams{getTask: getTask, getEvents: getEvents, subscriber: subscriber}
}

// TaskEvents serves TaskEventsRoute.
func (s *TaskStreams) TaskEvents(w http.ResponseWriter, r *http.Request) {
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := s.getTask.Execute(r.Context(), id); err != nil {
		if errors.Is(err, werrors.NotFound("")) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cursor := replayCursor(r.Header.Get(lastEventIDHeader), r.URL.Query().Get(afterQuery))

	// Subscribing before reading the history means an event persisted in between is delivered twice at
	// worst, and the sequence filter below drops the duplicate. It is never lost.
	subscription := s.subscriber.Subscribe(r.Context())
	defer subscription.Close()
	history, err := s.getEvents.Execute(r.Context(), id, cursor)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	stream, ok := web.NewSSE(w)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}
	ctx := r.Context()
	if cursor == nil {
		if err := stream.Send(ctx, "", resetEvent, resetEvent); err != nil {
			return
		}
	}
	lastSeq := cursor
	for _, event := range history {
		if err := sendTaskEvent(ctx, stream, event); err != nil {
			return
		}
		if event.Seq != nil {
			seq := *event.Seq
			lastSeq = &seq
		}
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
			if event.TaskID != id || !afterCursor(event, lastSeq) {
				continue
			}
			if err := sendTaskEvent(ctx, stream, event); err != nil {
				return
			}
		}
	}
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
			name, relevant := boardEventName(event.Kind)
			if !relevant {
				continue
			}
			if err := stream.Send(ctx, "", name, event.TaskID.String()); err != nil {
				return
			}
		}
	}
}

// replayCursor reads the cursor of a reconnection. Last-Event-ID wins over ?after=; an invalid value
// is ignored. A nil result means "from the beginning".
func replayCursor(lastEventID string, after string) *int64 {
	if seq, ok := parseSeq(lastEventID); ok {
		return &seq
	}
	if seq, ok := parseSeq(after); ok {
		return &seq
	}
	return nil
}

func parseSeq(raw string) (int64, bool) {
	seq, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || seq < 0 {
		return 0, false
	}
	return seq, true
}

// afterCursor reports whether a live event goes after the history already sent. Events that are not
// persisted always go through.
func afterCursor(event models.TaskEvent, lastSeq *int64) bool {
	if event.Seq == nil || lastSeq == nil {
		return true
	}
	return *event.Seq > *lastSeq
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

// sendTaskEvent writes one event with its sequence number as id, when it has one.
// A poison event that cannot be mapped or marshaled is logged and skipped, preventing reconnect loops.
func sendTaskEvent(ctx context.Context, stream *web.SSE, event models.TaskEvent) error {
	wire, err := dto.TaskEventFromDomain(event)
	if err != nil {
		slog.Warn("skipping unmappable task event", "seq", event.Seq, "error", err)
		return nil
	}
	data, err := json.Marshal(wire)
	if err != nil {
		slog.Warn("skipping unencodable task event", "seq", event.Seq, "error", err)
		return nil
	}
	id := ""
	if event.Seq != nil {
		id = strconv.FormatInt(*event.Seq, 10)
	}
	if err := stream.Send(ctx, id, "message", string(data)); err != nil {
		return fmt.Errorf("send task event: %w", err)
	}
	return nil
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
