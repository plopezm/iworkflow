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

// TestHookInput names the hook to try. The test runs it in the workspace it belongs to.
type TestHookInput struct {
	WorkspaceID string
	HookID      string
}

// TestHook runs one hook on request, as a dry run before it is enabled. It runs in the workspace directory with a
// synthetic task, and it writes nothing: there is no run record and no task involved. The hook runs even when it
// is disabled, and the outcome is returned as it is, failures included.
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

// Execute runs the hook and returns its outcome. An unknown hook is ErrHookNotFound, a hook of another workspace
// is ErrInvalidHook, and a run that could not be attempted is an error that wraps the cause.
func (u *TestHook) Execute(ctx context.Context, in TestHookInput) (models.RunOutcome, error) {
	hook, err := u.hooks.Get(ctx, in.HookID)
	if err != nil {
		return models.RunOutcome{}, fmt.Errorf("load hook %s: %w", in.HookID, err)
	}
	if hook.WorkspaceID() != in.WorkspaceID {
		return models.RunOutcome{}, fmt.Errorf("%w: hook %s belongs to another workspace", hookerrors.ErrInvalidHook, in.HookID)
	}
	path, err := u.workspaces.Path(ctx, in.WorkspaceID)
	if err != nil {
		return models.RunOutcome{}, fmt.Errorf("find workspace %s: %w", in.WorkspaceID, err)
	}
	run := models.RunContext{
		TaskID: testTaskName, TaskTitle: testTaskName, Branch: testTaskName,
		WorkspacePath: path, WorktreePath: path,
		Phase: hook.Phase(), Event: hook.Event(), Result: testResult(hook.Phase()),
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
