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
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/outbound/system"
	"iworkflow/internal/modules/task_hooks/infrastructure/di"
)

// seedWorkspaceAndTask inserts the rows the hooks and their run records point at.
func seedWorkspaceAndTask(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO workspaces (id, name, path, base_branch, created_at) VALUES ('w1', 'one', '/tmp/one', 'main', '2026-01-01T00:00:00Z')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO tasks (id, workspace_id, title, prompt, branch, base_branch, worktree_path, status, created_at, updated_at)
		VALUES ('t1', 'w1', 't', 'p', 'b', 'main', '/tmp/wt', 'backlog', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	require.NoError(t, err)
}

// The module wired without an agent session: a script hook runs for real, and an agent hook is recorded as an error
// with the reason, instead of running anything or stopping the chain.
func TestModuleRunsScriptsAndRefusesAgentsWithoutASession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := dbtest.New(t)
	seedWorkspaceAndTask(t, db)
	module := di.NewModule(di.Deps{DB: db, Clock: system.NewClock()})
	_, err := module.SaveHook.Execute(ctx, application.SaveHookInput{
		WorkspaceID: "w1", Phase: models.PhasePre, Event: models.EventTaskStart, Kind: models.KindScript,
		Enabled: true, OnFailure: models.FailureBlock, Timeout: 30 * time.Second,
		Script: models.ScriptSpec{Command: "true"},
	})
	require.NoError(t, err)
	agent, err := module.SaveHook.Execute(ctx, application.SaveHookInput{
		WorkspaceID: "w1", Phase: models.PhasePre, Event: models.EventTaskStart, Kind: models.KindAgent,
		Enabled: true, OnFailure: models.FailureWarn, Timeout: 30 * time.Second,
		Agent: models.AgentSpec{Prompt: "check the branch"},
	})
	require.NoError(t, err)

	report, err := module.RunHooks.Execute(ctx, application.RunHooksInput{
		WorkspaceID: "w1",
		Run: models.RunContext{TaskID: "t1", WorktreePath: t.TempDir(), Phase: models.PhasePre,
			Event: models.EventTaskStart},
	})

	require.NoError(t, err)
	require.Len(t, report.Runs, 2)
	assert.Equal(t, models.RunOK, report.Runs[0].Status)
	assert.Equal(t, models.RunErrored, report.Runs[1].Status)
	assert.Contains(t, report.Runs[1].Output, "no agent session is configured")
	assert.Equal(t, []string{"hook " + agent.ID() + " error"}, report.Warnings)
}

// The status strip shows each hook of the workspace with its latest run and the skip, through the module.
func TestModuleSkipsAHookForOneTaskAndShowsItInTheStatusStrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := dbtest.New(t)
	seedWorkspaceAndTask(t, db)
	module := di.NewModule(di.Deps{DB: db, Clock: system.NewClock()})
	saved, err := module.SaveHook.Execute(ctx, application.SaveHookInput{
		WorkspaceID: "w1", Phase: models.PhasePost, Event: models.EventTaskFinish, Kind: models.KindScript,
		Enabled: true, OnFailure: models.FailureIgnore, Timeout: 30 * time.Second,
		Script: models.ScriptSpec{Command: "true"},
	})
	require.NoError(t, err)
	require.NoError(t, module.SetTaskHookSkipped.Execute(ctx, "w1", "t1", saved.ID(), true))

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

// A run left running by a previous process is closed by the reconciliation at start-up, and the status strip then
// shows it as errored.
func TestModuleReconcilesRunsLeftRunningByAPreviousProcess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := dbtest.New(t)
	seedWorkspaceAndTask(t, db)
	module := di.NewModule(di.Deps{DB: db, Clock: system.NewClock()})
	saved, err := module.SaveHook.Execute(ctx, application.SaveHookInput{
		WorkspaceID: "w1", Phase: models.PhasePre, Event: models.EventTaskStart, Kind: models.KindScript,
		Enabled: true, OnFailure: models.FailureBlock, Timeout: 30 * time.Second,
		Script: models.ScriptSpec{Command: "true"},
	})
	require.NoError(t, err)
	require.NoError(t, insertRunningRun(ctx, db, saved.ID()))

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
func insertRunningRun(ctx context.Context, db *sql.DB, hookID string) error {
	_, err := db.ExecContext(ctx, `INSERT INTO task_hook_runs (id, task_id, hook_id, phase, event, status, exit_code, output, started_at, finished_at)
		VALUES ('orphan', 't1', ?, 'pre', 'task_start', 'running', 0, '', '2026-01-01T00:00:00.000000000Z', '')`, hookID)
	return err
}
