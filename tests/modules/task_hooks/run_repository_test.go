package task_hooks_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/outbound/persistence"
)

// seedRunFixtures inserts the tasks of two workspaces and the hooks the run records point at.
func seedRunFixtures(t *testing.T) (*persistence.RunRepository, *sql.DB) {
	t.Helper()
	repo, db := setup(t)
	seedTask(t, db, "t1", "w1")
	seedTask(t, db, "t2", "w2")
	require.NoError(t, repo.Save(context.Background(), libraryHook(t, "a", "Lint")))
	require.NoError(t, repo.Save(context.Background(), libraryHook(t, "b", "Test")))
	return persistence.NewRunRepository(db), db
}

func runAt(id string, started time.Time) models.HookRun {
	return models.HookRun{ID: id, TaskID: "t1", HookID: "a", AssignmentID: "x", HookName: "Lint", WorkspaceID: "w1",
		Phase: models.PhasePre, Event: models.EventTaskStart, Status: models.RunRunning, StartedAt: started}
}

// writeLegacyRow inserts a run the way the first version stored it: RFC 3339 text with a variable fraction.
func writeLegacyRow(t *testing.T, db *sql.DB, id string, started string, finished string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO task_hook_runs (id, task_id, hook_id, assignment_id, hook_name, workspace_id, phase, event, status, exit_code, output, started_at, finished_at)
		VALUES (?, 't1', 'a', 'x', 'Lint', 'w1', 'pre', 'task_start', 'ok', 0, '', ?, ?)`, id, started, finished)
	require.NoError(t, err)
}

func TestRunRecordsRoundTrip(t *testing.T) {
	runs, _ := seedRunFixtures(t)
	ctx := context.Background()
	start := time.Date(2026, 11, 7, 10, 0, 0, 123, time.UTC)
	run := runAt("r1", start)
	require.NoError(t, runs.Save(ctx, run))

	got, err := runs.ListByTask(ctx, "t1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, models.RunRunning, got[0].Status)
	assert.True(t, got[0].FinishedAt.IsZero())
	assert.Equal(t, "x", got[0].AssignmentID)
	assert.Equal(t, "Lint", got[0].HookName)
	assert.Equal(t, "w1", got[0].WorkspaceID)

	run.Status, run.ExitCode, run.Output, run.FinishedAt = models.RunFailed, 3, "boom", start.Add(time.Second)
	require.NoError(t, runs.Save(ctx, run))
	got, err = runs.ListByTask(ctx, "t1")
	require.NoError(t, err)
	require.Len(t, got, 1, "saving the same id updates the record")
	assert.Equal(t, models.RunFailed, got[0].Status)
	assert.Equal(t, 3, got[0].ExitCode)
	assert.Equal(t, "boom", got[0].Output)
	assert.True(t, start.Equal(got[0].StartedAt))
	assert.True(t, start.Add(time.Second).Equal(got[0].FinishedAt))

	none, err := runs.ListByTask(ctx, "other")
	require.NoError(t, err)
	assert.Empty(t, none)
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
	require.NoError(t, runs.Save(ctx, models.HookRun{ID: "r1", TaskID: "t1", HookID: "a", AssignmentID: "x", HookName: "Lint",
		WorkspaceID: "w1", Phase: models.PhasePre, Event: models.EventTaskStart, Status: models.RunOK,
		StartedAt: local, FinishedAt: local.Add(1500 * time.Millisecond)}))

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

// seedHistory writes six runs over two hooks and two workspaces, one per status, one minute apart:
//
//	r1 Lint  w1 ok        10:00
//	r2 Lint  w1 failed    10:01
//	r3 Test  w2 error     10:02
//	r4 Lint  w1 timeout   10:03
//	r5 Test  w2 running   10:04
//	r6 Test  w2 skipped   10:05
func seedHistory(t *testing.T, runs *persistence.RunRepository) {
	t.Helper()
	base := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	rows := []struct {
		id, task, hook, name, workspace string
		status                          models.RunStatus
		minute                          int
	}{
		{"r1", "t1", "a", "Lint", "w1", models.RunOK, 0},
		{"r2", "t1", "a", "Lint", "w1", models.RunFailed, 1},
		{"r3", "t2", "b", "Test", "w2", models.RunErrored, 2},
		{"r4", "t1", "a", "Lint", "w1", models.RunTimedOut, 3},
		{"r5", "t2", "b", "Test", "w2", models.RunRunning, 4},
		{"r6", "t2", "b", "Test", "w2", models.RunSkipped, 5},
	}
	for _, row := range rows {
		started := base.Add(time.Duration(row.minute) * time.Minute)
		require.NoError(t, runs.Save(context.Background(), models.HookRun{
			ID: row.id, TaskID: row.task, HookID: row.hook, AssignmentID: "x-" + row.hook, HookName: row.name,
			WorkspaceID: row.workspace, Phase: models.PhasePre, Event: models.EventTaskStart,
			Status: row.status, StartedAt: started,
		}))
	}
}

func TestRunListFilters(t *testing.T) {
	runs, _ := seedRunFixtures(t)
	seedHistory(t, runs)
	ctx := context.Background()

	cases := map[string]struct {
		filter outbound.RunFilter
		want   []string
	}{
		"no filter, newest first":         {outbound.RunFilter{}, []string{"r6", "r5", "r4", "r3", "r2", "r1"}},
		"one hook":                        {outbound.RunFilter{HookID: "a"}, []string{"r4", "r2", "r1"}},
		"one workspace":                   {outbound.RunFilter{WorkspaceID: "w2"}, []string{"r6", "r5", "r3"}},
		"hook and workspace":              {outbound.RunFilter{HookID: "b", WorkspaceID: "w1"}, []string{}},
		"failed only":                     {outbound.RunFilter{FailedOnly: true}, []string{"r4", "r3", "r2"}},
		"failed of one hook":              {outbound.RunFilter{HookID: "a", FailedOnly: true}, []string{"r4", "r2"}},
		"failed of one workspace":         {outbound.RunFilter{WorkspaceID: "w2", FailedOnly: true}, []string{"r3"}},
		"limit":                           {outbound.RunFilter{Limit: 2}, []string{"r6", "r5"}},
		"limit of failed":                 {outbound.RunFilter{FailedOnly: true, Limit: 2}, []string{"r4", "r3"}},
		"unknown hook":                    {outbound.RunFilter{HookID: "missing"}, []string{}},
		"negative limit uses the default": {outbound.RunFilter{Limit: -1}, []string{"r6", "r5", "r4", "r3", "r2", "r1"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := runs.List(ctx, tc.filter)

			require.NoError(t, err)
			ids := make([]string, 0, len(got))
			for _, run := range got {
				ids = append(ids, run.ID)
			}
			assert.Equal(t, tc.want, ids)
		})
	}
}

func TestRunListKeepsTheHookNameOfEachRecord(t *testing.T) {
	runs, _ := seedRunFixtures(t)
	seedHistory(t, runs)

	got, err := runs.List(context.Background(), outbound.RunFilter{HookID: "b", FailedOnly: true})

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Test", got[0].HookName)
	assert.Equal(t, "w2", got[0].WorkspaceID)
	assert.Equal(t, models.RunErrored, got[0].Status)
}

// Runs that start at the same instant are ordered by insertion, the later one first.
func TestRunListBreaksTiesByInsertionOrder(t *testing.T) {
	runs, _ := seedRunFixtures(t)
	ctx := context.Background()
	same := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	require.NoError(t, runs.Save(ctx, runAt("first", same)))
	require.NoError(t, runs.Save(ctx, runAt("second", same)))

	got, err := runs.List(ctx, outbound.RunFilter{})

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "second", got[0].ID)
	assert.Equal(t, "first", got[1].ID)
}

func TestRunLastFailuresIsTheLatestFailureOfEachHook(t *testing.T) {
	runs, _ := seedRunFixtures(t)
	seedHistory(t, runs)
	ctx := context.Background()
	require.NoError(t, runs.Save(ctx, models.HookRun{ID: "r7", TaskID: "t1", HookID: "c", AssignmentID: "x-c", HookName: "Audit",
		WorkspaceID: "w1", Phase: models.PhasePost, Event: models.EventTaskFinish, Status: models.RunOK,
		StartedAt: time.Date(2026, 11, 7, 10, 9, 0, 0, time.UTC)}))

	failures, err := runs.LastFailures(ctx)

	require.NoError(t, err)
	require.Len(t, failures, 2, "a hook with no failure has no entry")
	assert.Equal(t, "r4", failures["a"].ID, "the latest failure of Lint is the timeout, not the older failed run")
	assert.Equal(t, models.RunTimedOut, failures["a"].Status)
	assert.Equal(t, "r3", failures["b"].ID, "running and skipped runs are not failures")
	assert.NotContains(t, failures, "c")
}

// Review advisory (g): the runs a dead process left running are closed as errors; the finished ones are untouched.
func TestFailStaleRunningClosesOnlyTheRunningRecords(t *testing.T) {
	runs, _ := seedRunFixtures(t)
	ctx := context.Background()
	started := time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)
	require.NoError(t, runs.Save(ctx, runAt("stale", started)))
	require.NoError(t, runs.Save(ctx, models.HookRun{ID: "done", TaskID: "t1", HookID: "a", AssignmentID: "x", HookName: "Lint",
		WorkspaceID: "w1", Phase: models.PhasePre, Event: models.EventTaskStart, Status: models.RunOK,
		StartedAt: started, FinishedAt: started.Add(time.Second)}))
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
