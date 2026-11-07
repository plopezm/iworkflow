package task_hooks_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/outbound/persistence"
)

// seedRunFixtures inserts the task and the hook the run records point at.
func seedRunFixtures(t *testing.T) (*persistence.RunRepository, *sql.DB) {
	t.Helper()
	repo, db := setup(t)
	_, err := db.Exec(`INSERT INTO tasks (id, workspace_id, title, prompt, branch, base_branch, worktree_path, status, created_at, updated_at)
		VALUES ('t1', 'w1', 't', 'p', 'b', 'main', '/tmp/wt', 'backlog', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), hook(t, "a", "w1", models.EventTaskStart, models.PhasePre)))
	return persistence.NewRunRepository(db), db
}

func runAt(id string, started time.Time) models.HookRun {
	return models.HookRun{ID: id, TaskID: "t1", HookID: "a", Phase: models.PhasePre, Event: models.EventTaskStart,
		Status: models.RunRunning, StartedAt: started}
}

// writeLegacyRow inserts a run the way the first version stored it: RFC 3339 text with a variable fraction.
func writeLegacyRow(t *testing.T, db *sql.DB, id string, started string, finished string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO task_hook_runs (id, task_id, hook_id, phase, event, status, exit_code, output, started_at, finished_at)
		VALUES (?, 't1', 'a', 'pre', 'task_start', 'ok', 0, '', ?, ?)`, id, started, finished)
	require.NoError(t, err)
}

// Review advisory (c): the start order is a text sort, so the stored times must have a fixed width. With the
// variable RFC 3339 text, "10:00:00.5Z" sorted before "10:00:00Z" and the order was wrong.
func TestRunRecordsAreListedInChronologicalOrderWithFixedWidthTimes(t *testing.T) {
	runs, _ := seedRunFixtures(t)
	ctx := context.Background()
	require.NoError(t, runs.Save(ctx, runAt("late", time.Date(2026, 11, 7, 10, 0, 0, 500_000_000, time.UTC))))
	require.NoError(t, runs.Save(ctx, runAt("early", time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC))))

	got, err := runs.ListByTask(ctx, "t1")

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "early", got[0].ID)
	assert.Equal(t, "late", got[1].ID)
}

func TestRunTimesAreStoredAsFixedWidthUTCText(t *testing.T) {
	runs, db := seedRunFixtures(t)
	ctx := context.Background()
	local := time.Date(2026, 11, 7, 12, 0, 0, 0, time.FixedZone("UTC+2", 2*60*60))
	require.NoError(t, runs.Save(ctx, models.HookRun{ID: "r1", TaskID: "t1", HookID: "a", Phase: models.PhasePre,
		Event: models.EventTaskStart, Status: models.RunOK, StartedAt: local, FinishedAt: local.Add(1500 * time.Millisecond)}))

	var started, finished string
	require.NoError(t, db.QueryRow(`SELECT started_at, finished_at FROM task_hook_runs WHERE id = 'r1'`).Scan(&started, &finished))

	assert.Equal(t, "2026-11-07T10:00:00.000000000Z", started)
	assert.Equal(t, "2026-11-07T10:00:01.500000000Z", finished)
}

// Rows written before the fixed width still read back.
func TestRunRecordsWrittenWithRFC3339StillRead(t *testing.T) {
	runs, db := seedRunFixtures(t)
	writeLegacyRow(t, db, "legacy", "2026-11-07T10:00:00Z", "2026-11-07T10:00:01.5Z")

	got, err := runs.ListByTask(context.Background(), "t1")

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.True(t, time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC).Equal(got[0].StartedAt))
	assert.True(t, time.Date(2026, 11, 7, 10, 0, 1, 500_000_000, time.UTC).Equal(got[0].FinishedAt))
}

// Review advisory (g): the runs a dead process left running are closed as errors; the finished ones are untouched.
func TestFailStaleRunningClosesOnlyTheRunningRecords(t *testing.T) {
	runs, _ := seedRunFixtures(t)
	ctx := context.Background()
	started := time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)
	require.NoError(t, runs.Save(ctx, runAt("stale", started)))
	require.NoError(t, runs.Save(ctx, models.HookRun{ID: "done", TaskID: "t1", HookID: "a", Phase: models.PhasePre,
		Event: models.EventTaskStart, Status: models.RunOK, StartedAt: started, FinishedAt: started.Add(time.Second)}))
	closedAt := time.Date(2026, 11, 7, 11, 0, 0, 0, time.UTC)

	closed, err := runs.FailStaleRunning(ctx, closedAt)

	require.NoError(t, err)
	assert.Equal(t, int64(1), closed)
	got, err := runs.ListByTask(ctx, "t1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, models.RunErrored, got[0].Status)
	assert.Equal(t, models.StaleRunOutput, got[0].Output)
	assert.True(t, closedAt.Equal(got[0].FinishedAt))
	assert.Equal(t, models.RunOK, got[1].Status)
	assert.Empty(t, got[1].Output)
}

func TestFailStaleRunningWithNothingRunningClosesNothing(t *testing.T) {
	runs, _ := seedRunFixtures(t)

	closed, err := runs.FailStaleRunning(context.Background(), time.Now())

	require.NoError(t, err)
	assert.Equal(t, int64(0), closed)
}
