package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	hookapp "iworkflow/internal/modules/task_hooks/application"
	hookmodels "iworkflow/internal/modules/task_hooks/domain/models"
	tasksmodels "iworkflow/internal/modules/tasks/domain/models"
	tasksoutbound "iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// stripListerFake answers the hook states of a task, or the error it is set to.
type stripListerFake struct {
	states      []hookapp.TaskHookState
	err         error
	workspaceID string
	taskID      string
}

func (f *stripListerFake) Execute(_ context.Context, workspaceID string, taskID string) ([]hookapp.TaskHookState, error) {
	f.workspaceID = workspaceID
	f.taskID = taskID
	return f.states, f.err
}

// stripSkipperFake records the skip it is asked to set, or fails with the error it is set to.
type stripSkipperFake struct {
	err         error
	workspaceID string
	taskID      string
	hookID      string
	skipped     bool
	calls       int
}

func (f *stripSkipperFake) Execute(_ context.Context, workspaceID string, taskID string, hookID string, skipped bool) error {
	f.calls++
	f.workspaceID = workspaceID
	f.taskID = taskID
	f.hookID = hookID
	f.skipped = skipped
	return f.err
}

// stripScriptHook is an enabled script hook of the workspace, attached to the start of the task.
func stripScriptHook(t *testing.T, id string) hookmodels.Hook {
	t.Helper()
	hook, err := hookmodels.NewHook(hookmodels.HookParams{
		ID:          id,
		WorkspaceID: "ws-1",
		Phase:       hookmodels.PhasePre,
		Event:       hookmodels.EventTaskStart,
		Kind:        hookmodels.KindScript,
		Enabled:     true,
		OnFailure:   hookmodels.FailureBlock,
		Timeout:     time.Minute,
		Script:      hookmodels.ScriptSpec{Command: "make", Args: []string{"test"}},
	})
	require.NoError(t, err)
	return hook
}

func TestTaskHooksStripStateMapsHookWithoutRun(t *testing.T) {
	lister := &stripListerFake{states: []hookapp.TaskHookState{{Hook: stripScriptHook(t, "h-1")}}}
	strip := NewTaskHooksStrip()
	strip.Bind(lister, &stripSkipperFake{})

	hooks, err := strip.State(context.Background(), "ws-1", "task-1")

	require.NoError(t, err)
	assert.Equal(t, "ws-1", lister.workspaceID)
	assert.Equal(t, "task-1", lister.taskID)
	require.Len(t, hooks, 1)
	assert.Equal(t, tasksoutbound.StripHook{
		HookID:      "h-1",
		Kind:        "script",
		Description: "make",
		Phase:       "pre",
		Event:       "task_start",
		Enabled:     true,
		Skipped:     false,
		LastStatus:  tasksoutbound.StripStatusNone,
	}, hooks[0])
}

func TestTaskHooksStripStateMapsSkippedAgentHookAndEveryRunStatus(t *testing.T) {
	started := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	agent, err := hookmodels.NewHook(hookmodels.HookParams{
		ID:          "h-agent",
		WorkspaceID: "ws-1",
		Phase:       hookmodels.PhasePost,
		Event:       hookmodels.EventTaskFinish,
		Kind:        hookmodels.KindAgent,
		Enabled:     false,
		OnFailure:   hookmodels.FailureWarn,
		Timeout:     time.Minute,
		Agent:       hookmodels.AgentSpec{Prompt: "run the linter"},
	})
	require.NoError(t, err)
	runCases := map[hookmodels.RunStatus]string{
		hookmodels.RunOK:       tasksoutbound.StripStatusOK,
		hookmodels.RunFailed:   tasksoutbound.StripStatusFailed,
		hookmodels.RunTimedOut: tasksoutbound.StripStatusTimeout,
		hookmodels.RunErrored:  tasksoutbound.StripStatusError,
		hookmodels.RunRunning:  tasksoutbound.StripStatusRunning,
		hookmodels.RunSkipped:  tasksoutbound.StripStatusSkipped,
	}
	for runStatus, want := range runCases {
		run := hookmodels.HookRun{
			ID: "run-1", TaskID: "task-1", HookID: "h-agent", Status: runStatus,
			ExitCode: 3, Output: "boom <script>", StartedAt: started,
		}
		lister := &stripListerFake{states: []hookapp.TaskHookState{{Hook: agent, Skipped: true, LastRun: &run}}}
		strip := NewTaskHooksStrip()
		strip.Bind(lister, &stripSkipperFake{})

		hooks, err := strip.State(context.Background(), "ws-1", "task-1")

		require.NoError(t, err, string(runStatus))
		require.Len(t, hooks, 1)
		assert.Equal(t, "agent", hooks[0].Kind, string(runStatus))
		assert.Equal(t, "run the linter", hooks[0].Description, string(runStatus))
		assert.Equal(t, "post", hooks[0].Phase, string(runStatus))
		assert.Equal(t, "task_finish", hooks[0].Event, string(runStatus))
		assert.False(t, hooks[0].Enabled, string(runStatus))
		assert.True(t, hooks[0].Skipped, string(runStatus))
		assert.Equal(t, want, hooks[0].LastStatus, string(runStatus))
		assert.Equal(t, "boom <script>", hooks[0].LastOutput, string(runStatus))
		assert.Equal(t, 3, hooks[0].LastExitCode, string(runStatus))
		assert.True(t, started.Equal(hooks[0].LastStartedAt), string(runStatus))
	}
}

func TestTaskHooksStripStateWrapsListError(t *testing.T) {
	failure := errors.New("database is locked")
	strip := NewTaskHooksStrip()
	strip.Bind(&stripListerFake{err: failure}, &stripSkipperFake{})

	hooks, err := strip.State(context.Background(), "ws-1", "task-1")

	assert.Nil(t, hooks)
	assert.ErrorIs(t, err, failure)
}

func TestTaskHooksStripSetSkippedPassesThrough(t *testing.T) {
	for _, skipped := range []bool{true, false} {
		skipper := &stripSkipperFake{}
		strip := NewTaskHooksStrip()
		strip.Bind(&stripListerFake{}, skipper)

		require.NoError(t, strip.SetSkipped(context.Background(), "ws-1", "task-1", "h-1", skipped))

		assert.Equal(t, 1, skipper.calls)
		assert.Equal(t, "ws-1", skipper.workspaceID, "the skip is checked against the workspace of the task")
		assert.Equal(t, "task-1", skipper.taskID)
		assert.Equal(t, "h-1", skipper.hookID)
		assert.Equal(t, skipped, skipper.skipped)
	}
}

func TestTaskHooksStripSetSkippedWrapsError(t *testing.T) {
	failure := errors.New("disk full")
	strip := NewTaskHooksStrip()
	strip.Bind(&stripListerFake{}, &stripSkipperFake{err: failure})

	err := strip.SetSkipped(context.Background(), "ws-1", "task-1", "h-1", true)

	assert.ErrorIs(t, err, failure)
}

func TestTaskHooksStripUnboundShowsNothingAndRefusesSkips(t *testing.T) {
	strip := NewTaskHooksStrip()

	hooks, err := strip.State(context.Background(), "ws-1", "task-1")
	assert.NoError(t, err)
	assert.Empty(t, hooks)

	assert.Error(t, strip.SetSkipped(context.Background(), "ws-1", "task-1", "h-1", true))
}

// The task page must carry the strip of the workspace hooks in its Actions tab. Reproduces the e2e symptom at the
// composed router: a pre-start hook of the workspace, a task of that workspace, and the page of the task.
func TestTaskPageRendersTheHookStripOfTheWorkspace(t *testing.T) {
	ctx := context.Background()
	application := composeWithTasks(t)
	workspace := registerRepository(t, application)
	_, err := application.taskHooks.SaveHook.Execute(ctx, hookapp.SaveHookInput{
		WorkspaceID: workspace.ID.String(), Phase: hookmodels.PhasePre, Event: hookmodels.EventTaskStart,
		Kind: hookmodels.KindScript, Enabled: true, OnFailure: hookmodels.FailureBlock,
		Timeout: time.Minute, Script: hookmodels.ScriptSpec{Command: "true"},
	})
	require.NoError(t, err)
	task := createTask(t, application, workspace)
	started, err := application.tasks.Start.Execute(ctx, task.ID)
	require.NoError(t, err)
	require.NotEqual(t, tasksmodels.StatusBacklog, started.Status, "the start must leave the backlog")
	states, err := application.taskHooks.ListTaskHookState.Execute(ctx, workspace.ID.String(), task.ID.String())
	require.NoError(t, err)
	require.Len(t, states, 1, "the workspace hook must be listed for the task")

	request := httptest.NewRequest(http.MethodGet, "/dock/pane?path=/tasks/"+task.ID.String(), nil)
	request.AddCookie(&http.Cookie{Name: i18n.LocaleCookie, Value: "es"})
	rec := httptest.NewRecorder()
	application.Router().ServeHTTP(rec, request)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "data-task-hooks", "the strip is rendered in the actions tab")
	assert.Contains(t, body, `data-task-hook="`+states[0].Hook.ID()+`"`, "the row of the workspace hook is rendered")
	assert.Contains(t, body, `/tasks/`+task.ID.String()+`/hooks/`+states[0].Hook.ID()+`/skip`, "the row carries its skip toggle")
}
