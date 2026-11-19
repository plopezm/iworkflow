package inbound

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/domain/models"
)

// finishFake answers the processes that use the worktree and records what the finish was asked to do.
type finishFake struct {
	processes  []models.WorktreeProcess
	previewErr error
	executeErr error
	executed   int
	closing    bool
	closed     []models.WorktreeProcess
	previews   int
}

func (f *finishFake) Execute(_ context.Context, _ models.TaskID) error {
	f.executed++
	return f.executeErr
}

func (f *finishFake) Preview(_ context.Context, _ models.TaskID) ([]models.WorktreeProcess, error) {
	f.previews++
	return f.processes, f.previewErr
}

func (f *finishFake) ExecuteClosing(_ context.Context, _ models.TaskID, selected []models.WorktreeProcess) error {
	f.closing = true
	f.closed = selected
	return f.executeErr
}

// finishHandlerRouter mounts the detail handlers over the finish fake and a task that exists.
func finishHandlerRouter(t *testing.T, finish *finishFake) *chi.Mux {
	t.Helper()
	h := NewDetailHandlers(DetailUseCases{
		Get:             &diffTaskFake{task: newDiffTaskForTest(t)},
		Events:          detailHistoryFake{},
		ListAttachments: detailAttachmentsFake{},
		Finish:          finish,
	}, time.Now)
	router := chi.NewRouter()
	h.Mount(router)
	return router
}

// serveFinish sends one request in one language and returns the answer.
func serveFinish(t *testing.T, router *chi.Mux, locale i18n.Locale, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	req = req.WithContext(inLanguage(locale))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// finishPost posts the finish of a task with the given process values, as the dialog form does.
func finishPost(t *testing.T, router *chi.Mux, id string, processValues ...string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{"process": processValues}
	req := httptest.NewRequest(http.MethodPost, "/tasks/"+id+"/finish?scope=abc123", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return serveFinish(t, router, i18n.Es, req)
}

func TestFinishPreviewWithoutProcessesRendersTheConfirmationOnly(t *testing.T) {
	t.Parallel()
	id := newDiffTaskForTest(t).ID.String()
	finish := &finishFake{}

	rec := serveFinish(t, finishHandlerRouter(t, finish), i18n.Es, httptest.NewRequest(http.MethodGet, "/tasks/"+id+"/finish/preview?scope=abc123", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, finish.previews)
	assert.Contains(t, rec.Body.String(), `id="finish-panel-abc123"`, "the dialog is rendered open in this page instance")
	assert.Contains(t, rec.Body.String(), "¿Finalizar la tarea?")
	assert.Contains(t, rec.Body.String(), `hx-post="/tasks/`+id+`/finish?scope=abc123"`)
	assert.NotContains(t, rec.Body.String(), `name="process"`, "nothing to close, nothing to check")
}

func TestFinishPreviewRendersOneCheckedBoxPerProcess(t *testing.T) {
	t.Parallel()
	id := newDiffTaskForTest(t).ID.String()
	finish := &finishFake{processes: []models.WorktreeProcess{
		{PID: 42, Name: "node", Path: "/wt/app", Reason: models.ProcessReasonWorkingDirectory, Started: "Thu Nov 19 10:00:00 2026"},
		{PID: 77, Name: "vim", Path: "/wt/app/main.go", Reason: models.ProcessReasonOpenFile, Started: "Thu Nov 19 11:00:00 2026"},
	}}

	rec := serveFinish(t, finishHandlerRouter(t, finish), i18n.Es, httptest.NewRequest(http.MethodGet, "/tasks/"+id+"/finish/preview?scope=abc123", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Equal(t, 2, strings.Count(body, `name="process"`), "one checkbox per process")
	assert.Contains(t, body, `value="42|Thu Nov 19 10:00:00 2026"`)
	assert.Contains(t, body, `value="77|Thu Nov 19 11:00:00 2026"`)
	assert.Contains(t, body, "directorio de trabajo", "the reason of the first process in Spanish")
	assert.Contains(t, body, "archivo abierto", "the reason of the second process in Spanish")
	assert.Contains(t, body, "node · PID 42")
}

func TestFinishPreviewOfAnInvalidIdentifierShowsTheErrorInPlace(t *testing.T) {
	t.Parallel()
	finish := &finishFake{}

	rec := serveFinish(t, finishHandlerRouter(t, finish), i18n.En, httptest.NewRequest(http.MethodGet, "/tasks/no-es-un-uuid/finish/preview", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 0, finish.previews)
	assert.Contains(t, rec.Body.String(), "text-error")
	assert.NotContains(t, rec.Body.String(), `role="dialog"`)
}

func TestFinishPreviewFailureShowsTheErrorAndNoDialog(t *testing.T) {
	t.Parallel()
	id := newDiffTaskForTest(t).ID.String()
	finish := &finishFake{previewErr: errors.New("la tarea no se puede finalizar")}

	rec := serveFinish(t, finishHandlerRouter(t, finish), i18n.Es, httptest.NewRequest(http.MethodGet, "/tasks/"+id+"/finish/preview?scope=abc123", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "la tarea no se puede finalizar")
	assert.Contains(t, rec.Body.String(), "text-error")
	assert.NotContains(t, rec.Body.String(), `role="dialog"`)
}

func TestFinishWithSelectedProcessesClosesThemThroughTheUseCase(t *testing.T) {
	t.Parallel()
	id := newDiffTaskForTest(t).ID.String()
	finish := &finishFake{}

	rec := finishPost(t, finishHandlerRouter(t, finish), id, "42|Thu Nov 19 10:00:00 2026", "77|Thu Nov 19 11:00:00 2026")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, finish.closing, "the selection goes to ExecuteClosing")
	assert.Equal(t, 0, finish.executed, "a selection never runs the plain finish")
	assert.Equal(t, []models.WorktreeProcess{
		{PID: 42, Started: "Thu Nov 19 10:00:00 2026"},
		{PID: 77, Started: "Thu Nov 19 11:00:00 2026"},
	}, finish.closed)
	assert.Equal(t, boardRefreshEvent, rec.Header().Get("HX-Trigger"), "the board refreshes")
	assert.Contains(t, rec.Body.String(), `id="task-detail-abc123"`, "the answer is the page of this instance")
}

func TestFinishWithoutSelectionFinishesLikeBefore(t *testing.T) {
	t.Parallel()
	id := newDiffTaskForTest(t).ID.String()
	finish := &finishFake{}

	rec := finishPost(t, finishHandlerRouter(t, finish), id)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, finish.executed, "no process checked: the plain finish runs")
	assert.False(t, finish.closing, "and it closes nothing")
	assert.Empty(t, finish.closed)
}

func TestFinishWithAMalformedProcessValueFinishesNothing(t *testing.T) {
	t.Parallel()
	id := newDiffTaskForTest(t).ID.String()
	finish := &finishFake{}

	rec := finishPost(t, finishHandlerRouter(t, finish), id, "42|Thu Nov 19 10:00:00 2026", "not-a-process")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 0, finish.executed, "a malformed selection never finishes the task")
	assert.False(t, finish.closing, "and never closes anything")
	assert.Contains(t, rec.Body.String(), "text-error", "the refusal is shown")
	assert.Contains(t, rec.Body.String(), `id="task-detail-abc123"`)
}

func TestFinishWithADuplicatedPidFinishesNothing(t *testing.T) {
	t.Parallel()
	id := newDiffTaskForTest(t).ID.String()
	finish := &finishFake{}

	rec := finishPost(t, finishHandlerRouter(t, finish), id, "42|a", "42|b")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.False(t, finish.closing)
	assert.Equal(t, 0, finish.executed)
	assert.Contains(t, rec.Body.String(), "text-error")
}

func TestFinishFailureShowsTheErrorInPlace(t *testing.T) {
	t.Parallel()
	id := newDiffTaskForTest(t).ID.String()
	finish := &finishFake{executeErr: errors.New("no se pudo consolidar")}

	rec := finishPost(t, finishHandlerRouter(t, finish), id)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "no se pudo consolidar")
	assert.Contains(t, rec.Body.String(), "text-error")
}
