package app

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	hooksapp "iworkflow/internal/modules/task_hooks/application"
	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	hooksmodels "iworkflow/internal/modules/task_hooks/domain/models"
	taskserrors "iworkflow/internal/modules/tasks/domain/errors"
	tasksmodels "iworkflow/internal/modules/tasks/domain/models"
)

// fakeHookRunner records the input of the run and answers with a fixed report and error.
type fakeHookRunner struct {
	input  hooksapp.RunHooksInput
	report hooksapp.RunHooksReport
	err    error
}

func (f *fakeHookRunner) Execute(_ context.Context, in hooksapp.RunHooksInput) (hooksapp.RunHooksReport, error) {
	f.input = in
	return f.report, f.err
}

func lifecycleTask(t *testing.T) tasksmodels.Task {
	t.Helper()
	return tasksmodels.Task{
		ID: tasksmodels.TaskIDFromUUID(uuid.MustParse("6f1c9d2e-3b4a-4c5d-8e9f-0a1b2c3d4e5f")), WorkspaceID: "w1",
		Title: "Arregla el login", Status: tasksmodels.StatusStarting, Branch: "iwf/arregla-login",
		WorktreePath: "/worktrees/arregla-login",
	}
}

// The mapping is the contract between the tasks lifecycle and the hooks: every event, phase and result of the
// task action reaches the hooks of the matching kind, and an unknown value is refused instead of running nothing.
func TestLifecycleHooksMapsTheTaskActionToTheHooksRun(t *testing.T) {
	t.Parallel()
	task := lifecycleTask(t)
	wantTask := hooksmodels.RunContext{
		TaskID: task.ID.String(), TaskTitle: task.Title, TaskStatus: string(tasksmodels.StatusStarting),
		WorkspacePath: "/workspaces/proyecto", WorktreePath: task.WorktreePath, Branch: task.Branch,
	}
	tests := []struct {
		name    string
		request tasksmodels.HookRequest
		want    hooksmodels.RunContext
		wantErr bool
	}{
		{
			name:    "pre start",
			request: tasksmodels.HookRequest{Event: tasksmodels.HookEventStart, Phase: tasksmodels.HookPhasePre},
			want:    withRun(wantTask, hooksmodels.PhasePre, hooksmodels.EventTaskStart, hooksmodels.TaskResultNone),
		},
		{
			name:    "post finish ok",
			request: tasksmodels.HookRequest{Event: tasksmodels.HookEventFinish, Phase: tasksmodels.HookPhasePost, Result: tasksmodels.HookResultOK},
			want:    withRun(wantTask, hooksmodels.PhasePost, hooksmodels.EventTaskFinish, hooksmodels.TaskResultOK),
		},
		{
			name:    "post start failed",
			request: tasksmodels.HookRequest{Event: tasksmodels.HookEventStart, Phase: tasksmodels.HookPhasePost, Result: tasksmodels.HookResultFailed},
			want:    withRun(wantTask, hooksmodels.PhasePost, hooksmodels.EventTaskStart, hooksmodels.TaskResultFailed),
		},
		{
			name:    "post finish aborted",
			request: tasksmodels.HookRequest{Event: tasksmodels.HookEventFinish, Phase: tasksmodels.HookPhasePost, Result: tasksmodels.HookResultAborted},
			want:    withRun(wantTask, hooksmodels.PhasePost, hooksmodels.EventTaskFinish, hooksmodels.TaskResultAborted),
		},
		{name: "unknown event", request: tasksmodels.HookRequest{Event: "discard", Phase: tasksmodels.HookPhasePre}, wantErr: true},
		{name: "unknown phase", request: tasksmodels.HookRequest{Event: tasksmodels.HookEventStart, Phase: "during"}, wantErr: true},
		{
			name:    "unknown result",
			request: tasksmodels.HookRequest{Event: tasksmodels.HookEventStart, Phase: tasksmodels.HookPhasePost, Result: "maybe"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runner := &fakeHookRunner{}
			request := tt.request
			request.Task = task
			request.WorkspacePath = "/workspaces/proyecto"

			_, err := newLifecycleHooks(runner).Run(context.Background(), request)

			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, runner.input.Run.TaskID, "an unknown value runs no hook")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, task.WorkspaceID, runner.input.WorkspaceID)
			assert.Equal(t, tt.want, runner.input.Run)
		})
	}
}

func withRun(run hooksmodels.RunContext, phase hooksmodels.Phase, event hooksmodels.Event, result hooksmodels.TaskResult) hooksmodels.RunContext {
	run.Phase, run.Event, run.Result = phase, event, result
	return run
}

// A blocking hook fails the tasks action with the tasks ErrHookBlocked, and the message of the hook stays.
func TestLifecycleHooksMapsABlockingFailureToTheTasksError(t *testing.T) {
	t.Parallel()
	runner := &fakeHookRunner{
		report: hooksapp.RunHooksReport{Warnings: []string{"hook h2 failed"}},
		err:    fmt.Errorf("%w: hook h1 failed", hookerrors.ErrHookBlocked),
	}
	request := tasksmodels.HookRequest{Event: tasksmodels.HookEventStart, Phase: tasksmodels.HookPhasePre, Task: lifecycleTask(t)}

	report, err := newLifecycleHooks(runner).Run(context.Background(), request)

	require.ErrorIs(t, err, taskserrors.ErrHookBlocked)
	assert.ErrorContains(t, err, "hook h1 failed")
	assert.Equal(t, []string{"hook h2 failed"}, report.Warnings, "the warnings come back even when the action is blocked")
}

// Any other failure of the hooks is not presented as a blocked action.
func TestLifecycleHooksKeepsOtherFailuresOutOfTheBlockedError(t *testing.T) {
	t.Parallel()
	runner := &fakeHookRunner{err: errors.New("the hook storage is down")}
	request := tasksmodels.HookRequest{Event: tasksmodels.HookEventFinish, Phase: tasksmodels.HookPhasePost, Result: tasksmodels.HookResultOK, Task: lifecycleTask(t)}

	_, err := newLifecycleHooks(runner).Run(context.Background(), request)

	require.ErrorContains(t, err, "the hook storage is down")
	assert.False(t, errors.Is(err, taskserrors.ErrHookBlocked))
}

func TestLifecycleHooksReturnsTheWarningsOfAPassingRun(t *testing.T) {
	t.Parallel()
	runner := &fakeHookRunner{report: hooksapp.RunHooksReport{Warnings: []string{"hook h2 failed"}}}
	request := tasksmodels.HookRequest{Event: tasksmodels.HookEventStart, Phase: tasksmodels.HookPhasePre, Task: lifecycleTask(t)}

	report, err := newLifecycleHooks(runner).Run(context.Background(), request)

	require.NoError(t, err)
	assert.Equal(t, tasksmodels.HookReport{Warnings: []string{"hook h2 failed"}}, report)
}
