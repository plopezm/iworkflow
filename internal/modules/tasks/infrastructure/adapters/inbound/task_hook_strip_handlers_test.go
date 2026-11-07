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

	"iworkflow/internal/core/i18n"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// hookSetCall is one skip the handler asked the hook port to set.
type hookSetCall struct {
	taskID  string
	hookID  string
	skipped bool
}

// hookStripFake answers the hooks of a task and remembers the skips it is asked to set. A set skip changes the hook it
// names, so the answer after a toggle shows the new state. setErr makes every set fail.
type hookStripFake struct {
	hooks            []outbound.StripHook
	stateErr         error
	setErr           error
	stateWorkspaceID string
	setCalls         []hookSetCall
}

func (f *hookStripFake) State(_ context.Context, workspaceID string, _ string) ([]outbound.StripHook, error) {
	f.stateWorkspaceID = workspaceID
	return f.hooks, f.stateErr
}

func (f *hookStripFake) SetSkipped(_ context.Context, taskID string, hookID string, skipped bool) error {
	f.setCalls = append(f.setCalls, hookSetCall{taskID: taskID, hookID: hookID, skipped: skipped})
	if f.setErr != nil {
		return f.setErr
	}
	for i := range f.hooks {
		if f.hooks[i].HookID == hookID {
			f.hooks[i].Skipped = skipped
		}
	}
	return nil
}

// hookStripHookForTest is an enabled script hook of the workspace of the task, which has not run for it.
func hookStripHookForTest(id string) outbound.StripHook {
	return outbound.StripHook{
		HookID: id, Kind: "script", Description: "make test", Phase: "pre", Event: "task_start",
		Enabled: true, LastStatus: outbound.StripStatusNone,
	}
}

// newHookStripHandlersForTest builds the detail handlers over fakes, with the hook port injected or not.
func newHookStripHandlersForTest(task models.Task, getErr error, hooks outbound.HookStrip) *DetailHandlers {
	return NewDetailHandlers(DetailUseCases{
		Get:             &tabsTaskFake{task: task, err: getErr},
		Events:          tabsHistoryFake{},
		ListAttachments: tabsAttachmentsFake{},
		HookStrip:       hooks,
	}, time.Now)
}

// postHookSkip sends the toggle of a hook the way the strip does: a form with the wanted state, in the scope of the page.
func postHookSkip(t *testing.T, h *DetailHandlers, taskID string, hookID string, form string) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	h.Mount(router)
	target := "/tasks/" + taskID + "/hooks/" + hookID + "/skip?scope=abc123"
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("HX-Request", "true")
	request = request.WithContext(inLanguage(i18n.En))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

// A toggle sets the skip of the hook for the task and answers the strip with the new state.
func TestSetHookSkippedSetsTheSkipAndAnswersTheStrip(t *testing.T) {
	task := newRunningTaskForTabs(t)
	hooks := &hookStripFake{hooks: []outbound.StripHook{hookStripHookForTest("h-1")}}
	h := newHookStripHandlersForTest(task, nil, hooks)

	recorder := postHookSkip(t, h, task.ID.String(), "h-1", "skipped=true")

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, []hookSetCall{{taskID: task.ID.String(), hookID: "h-1", skipped: true}}, hooks.setCalls)
	body := recorder.Body.String()
	assert.Contains(t, body, `id="task-hooks-abc123"`, "the answer replaces the strip of this page")
	assert.Contains(t, body, ">Include<", "the hook now offers to include it again")
}

// A refused toggle keeps the strip and shows the refusal in it, without the internal text of the error.
func TestSetHookSkippedShowsARefusalInTheStrip(t *testing.T) {
	task := newRunningTaskForTabs(t)
	hooks := &hookStripFake{hooks: []outbound.StripHook{hookStripHookForTest("h-1")}, setErr: errors.New("database is locked")}
	h := newHookStripHandlersForTest(task, nil, hooks)

	recorder := postHookSkip(t, h, task.ID.String(), "h-1", "skipped=true")

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "The hook could not be updated.")
	assert.NotContains(t, body, "database is locked", "the internal text of the error is logged, not shown")
	assert.Contains(t, body, `data-task-hook="h-1"`, "the rows stay on the strip")
}

// A value that is not a boolean is refused before the port is asked.
func TestSetHookSkippedRejectsAnInvalidValue(t *testing.T) {
	task := newRunningTaskForTabs(t)
	hooks := &hookStripFake{hooks: []outbound.StripHook{hookStripHookForTest("h-1")}}
	h := newHookStripHandlersForTest(task, nil, hooks)

	recorder := postHookSkip(t, h, task.ID.String(), "h-1", "skipped=maybe")

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, hooks.setCalls)
	assert.Contains(t, recorder.Body.String(), "The skip value is not valid.")
}

// A task that no longer exists says so, as the other actions of the page do, and nothing is changed.
func TestSetHookSkippedOfAGoneTaskSaysItIsGone(t *testing.T) {
	task := newRunningTaskForTabs(t)
	hooks := &hookStripFake{hooks: []outbound.StripHook{hookStripHookForTest("h-1")}}
	h := newHookStripHandlersForTest(task, werrors.NotFound(""), hooks)

	recorder := postHookSkip(t, h, task.ID.String(), "h-1", "skipped=true")

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, hooks.setCalls)
	assert.NotContains(t, recorder.Body.String(), "task-hooks-abc123")
}

// Without a hook port the toggle does not exist.
func TestSetHookSkippedWithoutHookPortIsNotFound(t *testing.T) {
	task := newRunningTaskForTabs(t)
	h := newHookStripHandlersForTest(task, nil, nil)

	recorder := postHookSkip(t, h, task.ID.String(), "h-1", "skipped=true")

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

// The actions tab shows the strip of the workspace hooks of the task.
func TestActionsTabShowsTheHookStrip(t *testing.T) {
	task := newRunningTaskForTabs(t)
	hooks := &hookStripFake{hooks: []outbound.StripHook{hookStripHookForTest("h-1")}}
	h := newHookStripHandlersForTest(task, nil, hooks)

	recorder := serveTabs(t, h, i18n.En, "/tasks/"+task.ID.String()+"/tabs/actions?scope=abc123")

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `id="task-hooks-abc123"`)
	assert.Equal(t, task.WorkspaceID, hooks.stateWorkspaceID, "the hooks are the ones of the workspace of the task")
}

// A workspace without hooks shows no strip at all.
func TestActionsTabHidesTheHookStripWithoutHooks(t *testing.T) {
	task := newRunningTaskForTabs(t)
	h := newHookStripHandlersForTest(task, nil, &hookStripFake{})

	recorder := serveTabs(t, h, i18n.En, "/tasks/"+task.ID.String()+"/tabs/actions?scope=abc123")

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "task-hooks-abc123")
}

// A failed read of the hooks is shown in the strip, and the rest of the actions tab still renders.
func TestActionsTabShowsAFailedHookRead(t *testing.T) {
	task := newRunningTaskForTabs(t)
	h := newHookStripHandlersForTest(task, nil, &hookStripFake{stateErr: errors.New("database is locked")})

	recorder := serveTabs(t, h, i18n.En, "/tasks/"+task.ID.String()+"/tabs/actions?scope=abc123")

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "The hooks of the task could not be read.")
	assert.NotContains(t, body, "database is locked")
}
