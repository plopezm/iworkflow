package inbound

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
)

// memoryListerFake answers the memory feed of the task and remembers the input it was asked with.
type memoryListerFake struct {
	feed   models.TaskMemoryFeed
	err    error
	inputs []application.TaskMemoryInput
}

func (f *memoryListerFake) Execute(_ context.Context, input application.TaskMemoryInput) (models.TaskMemoryFeed, error) {
	f.inputs = append(f.inputs, input)
	return f.feed, f.err
}

// memoryPanelTaskForTest is the task the memory panel tests open: its workspace is the one the feed is read for.
func memoryPanelTaskForTest(t *testing.T) models.Task {
	t.Helper()
	return newRunningTaskForTabs(t)
}

// serveDetailPageForTest renders the detail page of the task the way the dock does, answering 200 when it renders.
func serveDetailPageForTest(t *testing.T, h *DetailHandlers, id string) *httptest.ResponseRecorder {
	t.Helper()
	var route dock.Route
	for _, candidate := range h.Tabs() {
		if candidate.Name == TaskDetailRouteName {
			route = candidate
		}
	}
	require.NotEmpty(t, route.Name)
	router := chi.NewRouter()
	router.Get("/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		web.Render(w, r, http.StatusOK, route.Render(r.Context(), dock.Params{"id": chi.URLParam(r, "id")}))
	})
	request := httptest.NewRequest(http.MethodGet, "/tasks/"+id, nil)
	request = request.WithContext(inLanguage(i18n.En))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

// openTagOf returns the opening tag of the first element that starts with prefix.
func openTagOf(t *testing.T, body string, prefix string) string {
	t.Helper()
	start := strings.Index(body, prefix)
	require.GreaterOrEqual(t, start, 0, "no %s element in the page", prefix)
	end := strings.Index(body[start:], ">")
	require.GreaterOrEqual(t, end, 0)
	return body[start : start+end]
}

// memoryPanelHandlersForTest builds the detail handlers over fakes, with the memory lister injected or not.
func memoryPanelHandlersForTest(task models.Task, memory TaskMemoryLister) *DetailHandlers {
	return NewDetailHandlers(DetailUseCases{
		Get:             &tabsTaskFake{task: task},
		Events:          tabsHistoryFake{},
		ListAttachments: tabsAttachmentsFake{},
		Memory:          memory,
	}, time.Now)
}

func TestDetailPageShowsTheMemoryPanelWithTypeTitleAndDate(t *testing.T) {
	t.Parallel()
	task := memoryPanelTaskForTest(t)
	lister := &memoryListerFake{feed: models.TaskMemoryFeed{Available: true, Memories: []models.TaskMemory{
		{Type: "decision", Title: "Use sqlc for queries", LastSeenAt: time.Date(2026, 11, 10, 9, 30, 0, 0, time.UTC)},
		{Type: "bugfix", Title: "Fix the stream reconnect", LastSeenAt: time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)},
	}}}
	h := memoryPanelHandlersForTest(task, lister)

	recorder := serveDetailPageForTest(t, h, task.ID.String())

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `data-task-memory`)
	assert.Contains(t, body, "Recent memories of this project (Engram)")
	assert.NotContains(t, openTagOf(t, body, "<details"), " open", "the panel is collapsed by default")
	assert.Contains(t, body, "Memory")
	assert.Contains(t, body, "decision")
	assert.Contains(t, body, "Use sqlc for queries")
	assert.Contains(t, body, "2026-11-10")
	assert.Contains(t, body, "bugfix")
	assert.Contains(t, body, "Fix the stream reconnect")
	assert.Contains(t, body, "2026-11-09")
	require.Len(t, lister.inputs, 1)
	assert.Equal(t, task.WorkspaceID, lister.inputs[0].WorkspaceID)
}

func TestDetailPageHidesTheMemoryPanelWhenEngramIsUnavailableOrEmpty(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		memory TaskMemoryLister
	}{
		"engram unavailable":       {memory: &memoryListerFake{feed: models.TaskMemoryFeed{Available: false}}},
		"project without memories": {memory: &memoryListerFake{feed: models.TaskMemoryFeed{Available: true}}},
		"port error":               {memory: &memoryListerFake{err: errors.New("boom")}},
		"no memory use case wired": {memory: nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			task := memoryPanelTaskForTest(t)
			h := memoryPanelHandlersForTest(task, tc.memory)

			recorder := serveDetailPageForTest(t, h, task.ID.String())

			require.Equal(t, http.StatusOK, recorder.Code, "the page renders in every case")
			assert.NotContains(t, recorder.Body.String(), "data-task-memory")
			assert.NotContains(t, recorder.Body.String(), "Recent memories of this project")
		})
	}
}

// blockingMemoryLister stands in for an Engram that never answers: it waits for its context. A safety timer keeps a
// broken implementation from hanging the test; it then answers a populated panel, so the test fails on the panel.
type blockingMemoryLister struct{}

func (blockingMemoryLister) Execute(ctx context.Context, _ application.TaskMemoryInput) (models.TaskMemoryFeed, error) {
	select {
	case <-ctx.Done():
		return models.TaskMemoryFeed{}, ctx.Err()
	case <-time.After(5 * time.Second):
		return models.TaskMemoryFeed{Available: true, Memories: []models.TaskMemory{
			{Type: "decision", Title: "late memory", LastSeenAt: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)},
		}}, nil
	}
}

// The task detail page waits for Engram at most one second; past that the panel is absent and the page renders.
func TestDetailPageAnswersWithinOneSecondWhenEngramBlocks(t *testing.T) {
	task := memoryPanelTaskForTest(t)
	h := memoryPanelHandlersForTest(task, blockingMemoryLister{})

	started := time.Now()
	recorder := serveDetailPageForTest(t, h, task.ID.String())
	elapsed := time.Since(started)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Less(t, elapsed, 1500*time.Millisecond, "the page must not wait for Engram past its one-second bound")
	assert.NotContains(t, recorder.Body.String(), "data-task-memory")
	assert.NotContains(t, recorder.Body.String(), "late memory")
}
