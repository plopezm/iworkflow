package application

import (
	"context"
	"fmt"

	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// testTaskName is the title, branch and id the synthetic task of a test carries. A test has no task of its own.
const testTaskName = "test"

// TestHookInput names the hook to try and the workspace it runs in, with the event and phase it pretends to be
// attached to.
type TestHookInput struct {
	WorkspaceID string
	HookID      string
	Event       models.Event
	Phase       models.Phase
}

// TestHook runs one hook on request, as a dry run before it is enabled. It runs in the workspace directory with a
// synthetic task, and it writes nothing: there is no run record and no task involved. The hook runs even when it
// is not assigned, and the outcome is returned as it is, failures included.
type TestHook struct {
	hooks      outbound.HookRepository
	workspaces outbound.WorkspaceLookup
	scripts    outbound.ScriptRunner
	agents     outbound.AgentRunner
}

// NewTestHook builds the use case over the same runners the real runs use.
func NewTestHook(hooks outbound.HookRepository, workspaces outbound.WorkspaceLookup, scripts outbound.ScriptRunner, agents outbound.AgentRunner) *TestHook {
	return &TestHook{hooks: hooks, workspaces: workspaces, scripts: scripts, agents: agents}
}

// Execute runs the hook and returns its outcome. An unknown hook is ErrHookNotFound, a missing event or phase
// is ErrInvalidHook, and a run that could not be attempted is an error that wraps the cause.
func (u *TestHook) Execute(ctx context.Context, in TestHookInput) (models.RunOutcome, error) {
	hook, err := u.hooks.Get(ctx, in.HookID)
	if err != nil {
		return models.RunOutcome{}, fmt.Errorf("load hook %s: %w", in.HookID, err)
	}
	if in.Phase != models.PhasePre && in.Phase != models.PhasePost || in.Event != models.EventTaskStart && in.Event != models.EventTaskFinish {
		return models.RunOutcome{}, fmt.Errorf("%w: the event and the phase of the test are required", hookerrors.ErrInvalidHook)
	}
	path, err := u.workspaces.Path(ctx, in.WorkspaceID)
	if err != nil {
		return models.RunOutcome{}, fmt.Errorf("find workspace %s: %w", in.WorkspaceID, err)
	}
	run := models.RunContext{
		TaskID: testTaskName, TaskTitle: testTaskName, Branch: testTaskName,
		WorkspacePath: path, WorktreePath: path,
		Phase: in.Phase, Event: in.Event, Result: testResult(in.Phase),
	}
	runner := u.scripts
	if hook.Kind() == models.KindAgent {
		runner = u.agents
	}
	outcome, err := runner.Run(ctx, hook, run)
	if err != nil {
		return models.RunOutcome{}, fmt.Errorf("test hook %s: %w", in.HookID, err)
	}
	return outcome, nil
}

// testResult is the outcome a post hook would see. A test has not finished any task, so it reports success.
func testResult(phase models.Phase) models.TaskResult {
	if phase == models.PhasePost {
		return models.TaskResultOK
	}
	return models.TaskResultNone
}
