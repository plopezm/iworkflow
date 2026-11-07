package task_hooks_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/outbound/persistence"
)

func setup(t *testing.T) (*persistence.HookRepository, *sql.DB) {
	t.Helper()
	db := dbtest.New(t)
	_, err := db.Exec(`INSERT INTO workspaces (id, name, path, base_branch, created_at) VALUES ('w1', 'one', '/tmp/one', 'main', '2026-01-01T00:00:00Z'), ('w2', 'two', '/tmp/two', 'main', '2026-01-01T00:00:00Z')`)
	require.NoError(t, err)
	return persistence.NewHookRepository(db), db
}

func hook(t *testing.T, id string, workspace string, event models.Event, phase models.Phase) models.Hook {
	t.Helper()
	h, err := models.NewHook(models.HookParams{
		ID: id, WorkspaceID: workspace, Phase: phase, Event: event, Kind: models.KindScript, Enabled: true,
		OnFailure: models.FailureWarn, Timeout: 90 * time.Second,
		Script: models.ScriptSpec{Command: "echo", Args: []string{"{{.Branch}}"}, Env: []models.EnvVar{{Name: "A", Value: "1"}}, UseShell: true},
	})
	require.NoError(t, err)
	return h
}

func TestSaveRoundTripAndOrder(t *testing.T) {
	repo, _ := setup(t)
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, hook(t, "a", "w1", models.EventTaskStart, models.PhasePre)))
	require.NoError(t, repo.Save(ctx, hook(t, "b", "w1", models.EventTaskFinish, models.PhasePost)))
	require.NoError(t, repo.Save(ctx, hook(t, "c", "w2", models.EventTaskStart, models.PhasePre)))

	got, err := repo.Get(ctx, "a")
	require.NoError(t, err)
	assert.Equal(t, 90*time.Second, got.Timeout())
	assert.Equal(t, []string{"{{.Branch}}"}, got.Script().Args)
	assert.Equal(t, []models.EnvVar{{Name: "A", Value: "1"}}, got.Script().Env)
	assert.True(t, got.Script().UseShell)

	list, err := repo.List(ctx, "w1")
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, []string{"a", "b"}, []string{list[0].ID(), list[1].ID()})

	filtered, err := repo.ListFor(ctx, "w1", models.EventTaskFinish, models.PhasePost)
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	assert.Equal(t, "b", filtered[0].ID())
}

func TestSaveUpdateKeepsPositionAndReorder(t *testing.T) {
	repo, _ := setup(t)
	ctx := context.Background()
	for _, id := range []string{"a", "b", "c"} {
		require.NoError(t, repo.Save(ctx, hook(t, id, "w1", models.EventTaskStart, models.PhasePre)))
	}
	disabled, err := models.NewHook(models.HookParams{
		ID: "a", WorkspaceID: "w1", Phase: models.PhasePre, Event: models.EventTaskStart, Kind: models.KindAgent,
		OnFailure: models.FailureBlock, Timeout: time.Minute, Agent: models.AgentSpec{Prompt: "lint", Runtime: "claude"},
	})
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, disabled))

	list, err := repo.List(ctx, "w1")
	require.NoError(t, err)
	assert.Equal(t, "a", list[0].ID())
	assert.False(t, list[0].Enabled())
	assert.Equal(t, "claude", list[0].Agent().Runtime)

	require.NoError(t, repo.Reorder(ctx, "w1", []string{"c", "a", "b"}))
	list, err = repo.List(ctx, "w1")
	require.NoError(t, err)
	assert.Equal(t, []string{"c", "a", "b"}, []string{list[0].ID(), list[1].ID(), list[2].ID()})
}

func TestNotFoundAndDelete(t *testing.T) {
	repo, _ := setup(t)
	ctx := context.Background()
	_, err := repo.Get(ctx, "missing")
	assert.ErrorIs(t, err, hookerrors.ErrHookNotFound)
	assert.ErrorIs(t, repo.Delete(ctx, "missing"), hookerrors.ErrHookNotFound)

	require.NoError(t, repo.Save(ctx, hook(t, "a", "w1", models.EventTaskStart, models.PhasePre)))
	require.NoError(t, repo.Delete(ctx, "a"))
	_, err = repo.Get(ctx, "a")
	assert.ErrorIs(t, err, hookerrors.ErrHookNotFound)
}

func TestWorkspaceDeleteCascades(t *testing.T) {
	repo, db := setup(t)
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, hook(t, "a", "w1", models.EventTaskStart, models.PhasePre)))
	_, err := db.Exec(`DELETE FROM workspaces WHERE id = 'w1'`)
	require.NoError(t, err)
	_, err = repo.Get(ctx, "a")
	assert.ErrorIs(t, err, hookerrors.ErrHookNotFound)
}

func TestSkipsPerTask(t *testing.T) {
	repo, db := setup(t)
	ctx := context.Background()
	_, err := db.Exec(`INSERT INTO tasks (id, workspace_id, title, prompt, branch, base_branch, worktree_path, status, created_at, updated_at)
		VALUES ('t1', 'w1', 't', 'p', 'b', 'main', '/tmp/wt', 'backlog', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, hook(t, "a", "w1", models.EventTaskStart, models.PhasePre)))

	require.NoError(t, repo.SetSkipped(ctx, "t1", "a", true))
	require.NoError(t, repo.SetSkipped(ctx, "t1", "a", true))
	ids, err := repo.SkippedHookIDs(ctx, "t1")
	require.NoError(t, err)
	assert.Equal(t, []string{"a"}, ids)

	require.NoError(t, repo.SetSkipped(ctx, "t1", "a", false))
	ids, err = repo.SkippedHookIDs(ctx, "t1")
	require.NoError(t, err)
	assert.Empty(t, ids)

	require.NoError(t, repo.SetSkipped(ctx, "t1", "a", true))
	require.NoError(t, repo.Delete(ctx, "a"))
	ids, err = repo.SkippedHookIDs(ctx, "t1")
	require.NoError(t, err)
	assert.Empty(t, ids)
}

// The upsert of an id that belongs to another workspace writes nothing, and the storage says so instead of succeeding
// silently: the caller gets ErrInvalidHook and the original hook is left as it was.
func TestSaveDoesNotTouchHookOfAnotherWorkspace(t *testing.T) {
	repo, _ := setup(t)
	ctx := context.Background()
	original := hook(t, "a", "w1", models.EventTaskStart, models.PhasePre)
	require.NoError(t, repo.Save(ctx, original))

	intruder := hook(t, "a", "w2", models.EventTaskFinish, models.PhasePost)
	err := repo.Save(ctx, intruder)

	require.ErrorIs(t, err, hookerrors.ErrInvalidHook)
	got, err := repo.Get(ctx, "a")
	require.NoError(t, err)
	assert.Equal(t, "w1", got.WorkspaceID())
	assert.Equal(t, models.EventTaskStart, got.Event())
	assert.Equal(t, models.PhasePre, got.Phase())
	assert.Equal(t, models.FailureWarn, got.OnFailure())
}

func TestRunRecordsRoundTrip(t *testing.T) {
	repo, db := setup(t)
	ctx := context.Background()
	_, err := db.Exec(`INSERT INTO tasks (id, workspace_id, title, prompt, branch, base_branch, worktree_path, status, created_at, updated_at)
		VALUES ('t1', 'w1', 't', 'p', 'b', 'main', '/tmp/wt', 'backlog', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, hook(t, "a", "w1", models.EventTaskStart, models.PhasePre)))
	runs := persistence.NewRunRepository(db)

	start := time.Date(2026, 11, 7, 10, 0, 0, 123, time.UTC)
	run := models.HookRun{ID: "r1", TaskID: "t1", HookID: "a", Phase: models.PhasePre, Event: models.EventTaskStart,
		Status: models.RunRunning, StartedAt: start}
	require.NoError(t, runs.Save(ctx, run))
	got, err := runs.ListByTask(ctx, "t1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, models.RunRunning, got[0].Status)
	assert.True(t, got[0].FinishedAt.IsZero())

	run.Status, run.ExitCode, run.Output, run.FinishedAt = models.RunFailed, 3, "boom", start.Add(time.Second)
	require.NoError(t, runs.Save(ctx, run))
	got, err = runs.ListByTask(ctx, "t1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, models.RunFailed, got[0].Status)
	assert.Equal(t, 3, got[0].ExitCode)
	assert.Equal(t, "boom", got[0].Output)
	assert.True(t, start.Equal(got[0].StartedAt))
	assert.True(t, start.Add(time.Second).Equal(got[0].FinishedAt))

	none, err := runs.ListByTask(ctx, "other")
	require.NoError(t, err)
	assert.Empty(t, none)
}
