package task_hooks_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/modules/task_hooks/application"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/outbound/system"
	"iworkflow/internal/modules/task_hooks/infrastructure/di"
)

// seedWorkspaceAndTask inserts the rows the assignments and their run records point at.
func seedWorkspaceAndTask(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO workspaces (id, name, path, base_branch, created_at) VALUES ('w1', 'one', '/tmp/one', 'main', '2026-01-01T00:00:00Z')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO tasks (id, workspace_id, title, prompt, branch, base_branch, worktree_path, status, created_at, updated_at)
		VALUES ('t1', 'w1', 't', 'p', 'b', 'main', '/tmp/wt', 'backlog', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	require.NoError(t, err)
}

// assign attaches the hook to w1 for the event and phase, as the screen does.
func assign(t *testing.T, module *di.Module, hookID string, event models.Event, phase models.Phase, policy models.FailurePolicy) models.Assignment {
	t.Helper()
	assignment, err := module.SaveAssignment.Execute(context.Background(), application.SaveAssignmentInput{
		HookID: hookID, WorkspaceID: "w1", Event: event, Phase: phase, Enabled: true, OnFailure: policy,
	})
	require.NoError(t, err)
	return assignment
}

// The module wired without an agent session: a script hook runs for real, and an agent hook is recorded as an error
// with the reason, instead of running anything or stopping the chain.
func TestModuleRunsScriptsAndRefusesAgentsWithoutASession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := dbtest.New(t)
	seedWorkspaceAndTask(t, db)
	module := di.NewModule(di.Deps{DB: db, Clock: system.NewClock()})
	script, err := module.SaveHook.Execute(ctx, application.SaveHookInput{
		Name: "script", Kind: models.KindScript, Timeout: 30 * time.Second, Script: models.ScriptSpec{Command: "true"},
	})
	require.NoError(t, err)
	agent, err := module.SaveHook.Execute(ctx, application.SaveHookInput{
		Name: "agent", Kind: models.KindAgent, Timeout: 30 * time.Second, Agent: models.AgentSpec{Prompt: "check the branch"},
	})
	require.NoError(t, err)
	assign(t, module, script.ID(), models.EventTaskStart, models.PhasePre, models.FailureBlock)
	assign(t, module, agent.ID(), models.EventTaskStart, models.PhasePre, models.FailureWarn)

	report, err := module.RunHooks.Execute(ctx, application.RunHooksInput{
		WorkspaceID: "w1",
		Run:         models.RunContext{TaskID: "t1", WorktreePath: t.TempDir(), Phase: models.PhasePre, Event: models.EventTaskStart},
	})

	require.NoError(t, err)
	require.Len(t, report.Runs, 2)
	assert.Equal(t, models.RunOK, report.Runs[0].Status)
	assert.Equal(t, models.RunErrored, report.Runs[1].Status)
	assert.Contains(t, report.Runs[1].Output, "no agent session is configured")
	assert.Equal(t, []string{"hook agent error"}, report.Warnings)
	assert.Equal(t, "agent", report.Runs[1].HookName)
}

// The status strip shows each assignment of the workspace with its latest run and the skip, through the module.
func TestModuleSkipsAnAssignmentForOneTaskAndShowsItInTheStatusStrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := dbtest.New(t)
	seedWorkspaceAndTask(t, db)
	module := di.NewModule(di.Deps{DB: db, Clock: system.NewClock()})
	saved, err := module.SaveHook.Execute(ctx, application.SaveHookInput{
		Name: "script", Kind: models.KindScript, Timeout: 30 * time.Second, Script: models.ScriptSpec{Command: "true"},
	})
	require.NoError(t, err)
	assignment := assign(t, module, saved.ID(), models.EventTaskFinish, models.PhasePost, models.FailureIgnore)
	require.NoError(t, module.SetTaskHookSkipped.Execute(ctx, "w1", "t1", assignment.ID(), true))

	report, err := module.RunHooks.Execute(ctx, application.RunHooksInput{
		WorkspaceID: "w1",
		Run: models.RunContext{TaskID: "t1", WorktreePath: t.TempDir(), Phase: models.PhasePost,
			Event: models.EventTaskFinish, Result: models.TaskResultOK},
	})
	require.NoError(t, err)
	require.Len(t, report.Runs, 1)
	assert.Equal(t, models.RunSkipped, report.Runs[0].Status)

	states, err := module.ListTaskHookState.Execute(ctx, "w1", "t1")

	require.NoError(t, err)
	require.Len(t, states, 1)
	assert.True(t, states[0].Skipped)
	require.NotNil(t, states[0].LastRun)
	assert.Equal(t, models.RunSkipped, states[0].LastRun.Status)
}

// One hook assigned to a project is deleted from the library: its assignments go and its failed runs stay in the history.
func TestModuleKeepsTheHistoryOfADeletedHook(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := dbtest.New(t)
	seedWorkspaceAndTask(t, db)
	module := di.NewModule(di.Deps{DB: db, Clock: system.NewClock()})
	saved, err := module.SaveHook.Execute(ctx, application.SaveHookInput{
		Name: "failing", Kind: models.KindScript, Timeout: 30 * time.Second, Script: models.ScriptSpec{Command: "false"},
	})
	require.NoError(t, err)
	assign(t, module, saved.ID(), models.EventTaskStart, models.PhasePre, models.FailureWarn)
	_, err = module.RunHooks.Execute(ctx, application.RunHooksInput{
		WorkspaceID: "w1",
		Run:         models.RunContext{TaskID: "t1", WorktreePath: t.TempDir(), Phase: models.PhasePre, Event: models.EventTaskStart},
	})
	require.NoError(t, err)

	require.NoError(t, module.DeleteHook.Execute(ctx, saved.ID()))

	assigned, err := module.ListWorkspaceAssignments.Execute(ctx, "w1")
	require.NoError(t, err)
	assert.Empty(t, assigned)
	runs, err := module.ListRuns.Execute(ctx, outbound.RunFilter{FailedOnly: true})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, "failing", runs[0].HookName)
	assert.Equal(t, models.RunFailed, runs[0].Status)
}

// A run left running by a previous process is closed by the reconciliation at start-up, and the status strip then
// shows it as errored.
func TestModuleReconcilesRunsLeftRunningByAPreviousProcess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := dbtest.New(t)
	seedWorkspaceAndTask(t, db)
	module := di.NewModule(di.Deps{DB: db, Clock: system.NewClock()})
	saved, err := module.SaveHook.Execute(ctx, application.SaveHookInput{
		Name: "script", Kind: models.KindScript, Timeout: 30 * time.Second, Script: models.ScriptSpec{Command: "true"},
	})
	require.NoError(t, err)
	assignment := assign(t, module, saved.ID(), models.EventTaskStart, models.PhasePre, models.FailureBlock)
	require.NoError(t, insertRunningRun(ctx, db, saved.ID(), assignment.ID()))

	closed, err := module.ReconcileHookRuns.Execute(ctx)

	require.NoError(t, err)
	assert.Equal(t, int64(1), closed)
	states, err := module.ListTaskHookState.Execute(ctx, "w1", "t1")
	require.NoError(t, err)
	require.Len(t, states, 1)
	require.NotNil(t, states[0].LastRun)
	assert.Equal(t, models.RunErrored, states[0].LastRun.Status)
}

// insertRunningRun inserts a running record the way a process that died would have left it.
func insertRunningRun(ctx context.Context, db *sql.DB, hookID string, assignmentID string) error {
	_, err := db.ExecContext(ctx, `INSERT INTO task_hook_runs (id, task_id, hook_id, assignment_id, hook_name, workspace_id, phase, event, status, exit_code, output, started_at, finished_at)
		VALUES ('orphan', 't1', ?, ?, 'script', 'w1', 'pre', 'task_start', 'running', 0, '', '2026-01-01T00:00:00.000000000Z', '')`, hookID, assignmentID)
	return err
}
