package tasks_test

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/tasks/infrastructure/di"
)

// frame is one server-sent event as the browser reads it.
type frame struct {
	id    string
	event string
	data  string
}

// frameReader parses the stream on its own goroutine. The goroutine ends when the body is closed, which
// the test does in its cleanup through the request context.
type frameReader struct {
	frames chan frame
}

func startFrameReader(body io.Reader) *frameReader {
	reader := &frameReader{frames: make(chan frame, 256)}
	go func() {
		defer close(reader.frames)
		scanner := bufio.NewScanner(body)
		var current frame
		hasFields := false
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case line == "":
				if hasFields {
					reader.frames <- current
				}
				current, hasFields = frame{}, false
			case strings.HasPrefix(line, ":"):
				continue
			case strings.HasPrefix(line, "id: "):
				current.id, hasFields = strings.TrimPrefix(line, "id: "), true
			case strings.HasPrefix(line, "event: "):
				current.event, hasFields = strings.TrimPrefix(line, "event: "), true
			case strings.HasPrefix(line, "data: "):
				current.data, hasFields = strings.TrimPrefix(line, "data: "), true
			}
		}
	}()
	return reader
}

// next returns the next frame, or fails the test after a timeout. ok is false when the stream ended.
func (r *frameReader) next(t *testing.T) (frame, bool) {
	t.Helper()
	select {
	case next, open := <-r.frames:
		return next, open
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for a stream frame")
		return frame{}, false
	}
}

// expectNext reads the next frame and checks its id, name and data.
func (r *frameReader) expectNext(t *testing.T, id string, event string, dataContains string) frame {
	t.Helper()
	next, open := r.next(t)
	require.True(t, open, "the stream ended before the expected %q frame", event)
	assert.Equal(t, id, next.id, "id of the %q frame", event)
	assert.Equal(t, event, next.event)
	assert.Contains(t, next.data, dataContains)
	return next
}

// streamServer mounts the task and board streams of the module on a local router, the way the composition
// root will mount them.
func streamServer(t *testing.T, module *di.Module) *httptest.Server {
	t.Helper()
	router := chi.NewRouter()
	router.Get(inbound.TaskEventsRoute, module.Streams.TaskEvents)
	router.Get(inbound.BoardEventsRoute, module.Streams.BoardEvents)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server
}

// openStream opens a stream request. Cancelling the test context ends the stream.
func openStream(t *testing.T, url string, headers map[string]string) *frameReader {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err)
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	t.Cleanup(func() { _ = response.Body.Close() })
	return startFrameReader(response.Body)
}

// recordTask stores an agent event through the lifecycle, which persists it and publishes it, as the
// use cases do. It returns once the event is in the history.
func recordTask(t *testing.T, module *di.Module, id models.TaskID, event models.AgentEvent) {
	t.Helper()
	require.NoError(t, module.Lifecycle.Record(bg(), id, event))
}

// Rust: sse (fresh stream: reset, persisted history with ids, then live events)
func TestTaskStreamSendsResetHistoryThenLiveEvents(t *testing.T) {
	t.Parallel()
	_, module, task := runningTaskWithModule(t)
	recordTask(t, module, task.ID, models.UserMessage{Text: "uno"})
	recordTask(t, module, task.ID, models.UserMessage{Text: "dos"})
	server := streamServer(t, module)

	reader := openStream(t, server.URL+"/api/tasks/"+task.ID.String()+"/events", nil)
	reader.expectNext(t, "", "reset", "reset")
	reader.expectNext(t, "1", "message", `"text":"uno"`)
	reader.expectNext(t, "2", "message", `"text":"dos"`)

	recordTask(t, module, task.ID, models.UserMessage{Text: "en vivo"})
	reader.expectNext(t, "3", "message", `"text":"en vivo"`)
}

// Live events that are not persisted carry no id, and events of other tasks are not delivered.
func TestTaskStreamDeliversOnlyItsOwnLiveEvents(t *testing.T) {
	t.Parallel()
	repo, module, task := runningTaskWithModule(t)
	other := newTask(t, insertWorkspace(t, repo.DB, "/repos/other"), "otra", models.StatusRunning)
	insertTask(t, repo, other)
	server := streamServer(t, module)

	reader := openStream(t, server.URL+"/api/tasks/"+task.ID.String()+"/events", nil)
	reader.expectNext(t, "", "reset", "reset")

	recordTask(t, module, other.ID, models.UserMessage{Text: "ajena"})
	recordTask(t, module, task.ID, models.TextDelta{Delta: "tecleo"})
	reader.expectNext(t, "", "message", `"delta":"tecleo"`)
}

// Rust: sse (Last-Event-ID resumes after the cursor, without a reset and without duplicates)
func TestTaskStreamResumesAfterLastEventID(t *testing.T) {
	t.Parallel()
	_, module, task := runningTaskWithModule(t)
	for _, text := range []string{"uno", "dos", "tres"} {
		recordTask(t, module, task.ID, models.UserMessage{Text: text})
	}
	server := streamServer(t, module)

	reader := openStream(t, server.URL+"/api/tasks/"+task.ID.String()+"/events", map[string]string{"Last-Event-ID": "1"})
	reader.expectNext(t, "2", "message", `"text":"dos"`)
	reader.expectNext(t, "3", "message", `"text":"tres"`)

	recordTask(t, module, task.ID, models.UserMessage{Text: "cuatro"})
	reader.expectNext(t, "4", "message", `"text":"cuatro"`)
}

// ?after= is the query alternative; a Last-Event-ID that is not a sequence number is ignored.
func TestTaskStreamAcceptsAfterQueryAndIgnoresAnInvalidLastEventID(t *testing.T) {
	t.Parallel()
	_, module, task := runningTaskWithModule(t)
	for _, text := range []string{"uno", "dos", "tres"} {
		recordTask(t, module, task.ID, models.UserMessage{Text: text})
	}
	server := streamServer(t, module)
	url := server.URL + "/api/tasks/" + task.ID.String() + "/events?after=2"

	reader := openStream(t, url, map[string]string{"Last-Event-ID": "not-a-number"})
	reader.expectNext(t, "3", "message", `"text":"tres"`)
}

func TestTaskStreamRejectsMalformedAndUnknownTasks(t *testing.T) {
	t.Parallel()
	_, module, _ := runningTaskWithModule(t)
	server := streamServer(t, module)

	bad, err := http.Get(server.URL + "/api/tasks/not-an-id/events")
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, bad.StatusCode)
	require.NoError(t, bad.Body.Close())

	unknown := newTaskID(t)
	missing, err := http.Get(server.URL + "/api/tasks/" + unknown.String() + "/events")
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, missing.StatusCode)
	require.NoError(t, missing.Body.Close())
}

// A subscriber that fell behind is told to reconnect ("lagged") and the stream ends. The client then
// resumes from its last id, so the events it missed come from the history.
func TestTaskStreamEndsWithLaggedWhenTheSubscriberOverflowed(t *testing.T) {
	t.Parallel()
	_, module, task := runningTaskWithModule(t)
	recordTask(t, module, task.ID, models.UserMessage{Text: "uno"})
	overflowed := overflowingSubscriber{}
	streams := inbound.NewTaskStreams(module.Get, module.GetEvents, overflowed)
	router := chi.NewRouter()
	router.Get(inbound.TaskEventsRoute, streams.TaskEvents)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	reader := openStream(t, server.URL+"/api/tasks/"+task.ID.String()+"/events", nil)
	reader.expectNext(t, "", "reset", "reset")
	reader.expectNext(t, "1", "message", `"text":"uno"`)
	reader.expectNext(t, "", "lagged", "lagged")
	_, open := reader.next(t)
	assert.False(t, open, "the stream must end after lagged")
}

// overflowingSubscriber is a subscriber that is already disconnected for overflow.
type overflowingSubscriber struct{}

func (overflowingSubscriber) Subscribe(context.Context) outbound.TaskEventSubscription {
	closed := make(chan models.TaskEvent)
	close(closed)
	return closedOverflowed{events: closed}
}

type closedOverflowed struct {
	events chan models.TaskEvent
}

func (s closedOverflowed) Events() <-chan models.TaskEvent { return s.events }
func (closedOverflowed) Overflowed() bool                  { return true }
func (closedOverflowed) Close()                            {}

// The board stream announces the status changes and removals of any task, and nothing else.
func TestBoardStreamAnnouncesStatusChangesAndRemovalsOnly(t *testing.T) {
	t.Parallel()
	_, module, task := runningTaskWithModule(t)
	server := streamServer(t, module)

	reader := openStream(t, server.URL+inbound.BoardEventsRoute, nil)
	reader.expectNext(t, "", "reset", "reset")

	recordTask(t, module, task.ID, models.TextDelta{Delta: "no se anuncia"})
	require.NoError(t, module.Lifecycle.Transition(bg(), task.ID, models.StatusIdle, nil))
	reader.expectNext(t, "", "changed", task.ID.String())

	require.NoError(t, module.Lifecycle.NotifyRemoved(bg(), task.ID))
	reader.expectNext(t, "", "removed", task.ID.String())
}
