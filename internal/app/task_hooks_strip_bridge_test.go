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

// stripListerFake answers the assignment states of a task, or the error it is set to.
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
	err          error
	workspaceID  string
	taskID       string
	assignmentID string
	skipped      bool
	calls        int
}

func (f *stripSkipperFake) Execute(_ context.Context, workspaceID string, taskID string, assignmentID string, skipped bool) error {
	f.calls++
	f.workspaceID = workspaceID
	f.taskID = taskID
	f.assignmentID = assignmentID
	f.skipped = skipped
	return f.err
}

// stripScriptHook is a script hook of the library.
func stripScriptHook(t *testing.T, id string) hookmodels.Hook {
	t.Helper()
	hook, err := hookmodels.NewHook(hookmodels.HookParams{
		ID: id, Name: "make test", Kind: hookmodels.KindScript, Timeout: time.Minute,
		Script: hookmodels.ScriptSpec{Command: "make", Args: []string{"test"}},
	})
	require.NoError(t, err)
	return hook
}

// stripAssignment attaches a hook to the start of a task of the workspace, before the action.
func stripAssignment(t *testing.T, id string, hookID string, enabled bool, phase hookmodels.Phase, event hookmodels.Event) hookmodels.Assignment {
	t.Helper()
	assignment, err := hookmodels.NewAssignment(hookmodels.AssignmentParams{
		ID: id, HookID: hookID, WorkspaceID: "ws-1", Event: event, Phase: phase,
		Enabled: enabled, OnFailure: hookmodels.FailureBlock, Position: 1,
	})
	require.NoError(t, err)
	return assignment
}

func TestTaskHooksStripStateMapsAssignmentWithoutRun(t *testing.T) {
	assignment := stripAssignment(t, "a-1", "h-1", true, hookmodels.PhasePre, hookmodels.EventTaskStart)
	lister := &stripListerFake{states: []hookapp.TaskHookState{{Hook: stripScriptHook(t, "h-1"), Assignment: assignment}}}
	strip := NewTaskHooksStrip()
	strip.Bind(lister, &stripSkipperFake{})

	hooks, err := strip.State(context.Background(), "ws-1", "task-1")

	require.NoError(t, err)
	assert.Equal(t, "ws-1", lister.workspaceID)
	assert.Equal(t, "task-1", lister.taskID)
	require.Len(t, hooks, 1)
	assert.Equal(t, tasksoutbound.StripHook{
		AssignmentID: "a-1",
		HookID:       "h-1",
		Kind:         "script",
		Description:  "make",
		Phase:        "pre",
		Event:        "task_start",
		Enabled:      true,
		Skipped:      false,
		LastStatus:   tasksoutbound.StripStatusNone,
	}, hooks[0])
}

func TestTaskHooksStripStateMapsSkippedAgentAssignmentAndEveryRunStatus(t *testing.T) {
	started := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	agent, err := hookmodels.NewHook(hookmodels.HookParams{
		ID: "h-agent", Name: "lint", Kind: hookmodels.KindAgent, Timeout: time.Minute,
		Agent: hookmodels.AgentSpec{Prompt: "run the linter"},
	})
	require.NoError(t, err)
	assignment := stripAssignment(t, "a-agent", "h-agent", false, hookmodels.PhasePost, hookmodels.EventTaskFinish)
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
			ID: "run-1", TaskID: "task-1", HookID: "h-agent", AssignmentID: "a-agent", Status: runStatus,
			ExitCode: 3, Output: "boom <script>", StartedAt: started,
		}
		lister := &stripListerFake{states: []hookapp.TaskHookState{{Hook: agent, Assignment: assignment, Skipped: true, LastRun: &run}}}
		strip := NewTaskHooksStrip()
		strip.Bind(lister, &stripSkipperFake{})

		hooks, err := strip.State(context.Background(), "ws-1", "task-1")

		require.NoError(t, err, string(runStatus))
		require.Len(t, hooks, 1)
		assert.Equal(t, "a-agent", hooks[0].AssignmentID, string(runStatus))
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

func TestTaskHooksStripSetSkippedPassesTheAssignmentThrough(t *testing.T) {
	for _, skipped := range []bool{true, false} {
		skipper := &stripSkipperFake{}
		strip := NewTaskHooksStrip()
		strip.Bind(&stripListerFake{}, skipper)

		require.NoError(t, strip.SetSkipped(context.Background(), "ws-1", "task-1", "a-1", skipped))

		assert.Equal(t, 1, skipper.calls)
		assert.Equal(t, "ws-1", skipper.workspaceID, "the skip is checked against the workspace of the task")
		assert.Equal(t, "task-1", skipper.taskID)
		assert.Equal(t, "a-1", skipper.assignmentID)
		assert.Equal(t, skipped, skipper.skipped)
	}
}

func TestTaskHooksStripSetSkippedWrapsError(t *testing.T) {
	failure := errors.New("disk full")
	strip := NewTaskHooksStrip()
	strip.Bind(&stripListerFake{}, &stripSkipperFake{err: failure})

	err := strip.SetSkipped(context.Background(), "ws-1", "task-1", "a-1", true)

	assert.ErrorIs(t, err, failure)
}

func TestTaskHooksStripUnboundShowsNothingAndRefusesSkips(t *testing.T) {
	strip := NewTaskHooksStrip()

	hooks, err := strip.State(context.Background(), "ws-1", "task-1")
	assert.NoError(t, err)
	assert.Empty(t, hooks)

	assert.Error(t, strip.SetSkipped(context.Background(), "ws-1", "task-1", "a-1", true))
}

// The task page must carry the strip of the workspace assignments in its Actions tab. Reproduces the e2e symptom at the
// composed router: a pre-start assignment of the workspace, a task of that workspace, and the page of the task.
func TestTaskPageRendersTheHookStripOfTheWorkspace(t *testing.T) {
	ctx := context.Background()
	application := composeWithTasks(t)
	workspace := registerRepository(t, application)
	assignHook(t, application, workspace.ID.String(), hookmodels.EventTaskStart, hookmodels.PhasePre, hookmodels.FailureBlock,
		hookapp.SaveHookInput{Name: "true", Kind: hookmodels.KindScript, Timeout: time.Minute, Script: hookmodels.ScriptSpec{Command: "true"}})
	task := createTask(t, application, workspace)
	started, err := application.tasks.Start.Execute(ctx, task.ID)
	require.NoError(t, err)
	require.NotEqual(t, tasksmodels.StatusBacklog, started.Status, "the start must leave the backlog")
	states, err := application.taskHooks.ListTaskHookState.Execute(ctx, workspace.ID.String(), task.ID.String())
	require.NoError(t, err)
	require.Len(t, states, 1, "the workspace assignment must be listed for the task")

	request := httptest.NewRequest(http.MethodGet, "/dock/pane?path=/tasks/"+task.ID.String(), nil)
	request.AddCookie(&http.Cookie{Name: i18n.LocaleCookie, Value: "es"})
	rec := httptest.NewRecorder()
	application.Router().ServeHTTP(rec, request)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "data-task-hooks", "the strip is rendered in the actions tab")
	assert.Contains(t, body, `data-task-hook="`+states[0].Assignment.ID()+`"`, "the row of the assignment is rendered")
	assert.Contains(t, body, `/tasks/`+task.ID.String()+`/hooks/`+states[0].Assignment.ID()+`/skip`, "the row carries its skip toggle")
}
