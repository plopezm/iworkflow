package tasks_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// rustTaskRow is the input of insertTaskRow. Nil pointers are stored as NULL.
type rustTaskRow struct {
	id               string
	workspaceID      string
	title            string
	status           string
	priority         string
	createdAt        string
	updatedAt        string
	pendingDialogs   string
	subagentsActive  int64
	subagentsWaiting int64
	errorText        *string
	baseSHA          *string
	headSHA          *string
	memoryMB         *int64
	cpuPercent       *int64
	resumeAt         *string
	resumeAttempts   int64
}

// insertTaskRow stores a row with the exact column text the Rust (Diesel) version writes on SQLite:
// TIMESTAMP columns as "YYYY-MM-DD HH:MM:SS[.f]", nullable columns NULL when unset, counters as INTEGER.
// Values are bound parameters and the statement names every column, as Diesel does.
func insertTaskRow(t *testing.T, repo *Repo, row rustTaskRow) {
	t.Helper()
	_, err := repo.DB.ExecContext(bg(), `
		INSERT INTO tasks (id, workspace_id, title, prompt, branch, base_branch, worktree_path, status, error,
			created_at, updated_at, pending_dialogs, subagents_active, subagents_waiting, priority,
			base_sha, head_sha, memory_limit_mb, cpu_limit_percent, resume_at, resume_attempts)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.id, row.workspaceID, row.title, "haz algo", "pi/"+row.title, "main", "/wt/"+row.title,
		row.status, row.errorText, row.createdAt, row.updatedAt, row.pendingDialogs,
		row.subagentsActive, row.subagentsWaiting, row.priority, row.baseSHA, row.headSHA,
		row.memoryMB, row.cpuPercent, row.resumeAt, row.resumeAttempts)
	require.NoError(t, err)
}

// storedText reads one text value as the database holds it. The query is a literal with CAST, so the
// driver hands back the stored text and not a time it converted from it.
func storedText(t *testing.T, repo *Repo, query string, id models.TaskID) string {
	t.Helper()
	var text string
	require.NoError(t, repo.DB.QueryRowContext(bg(), query, id.String()).Scan(&text))
	return text
}

// Rust: no Rust test reads a row written by Diesel. This pins the column format that task_row.rs
// reads and writes: TIMESTAMP text with and without fraction, NULL nullable columns, and the legacy
// pending dialog array next to the current object form.
func TestReadsRowsWrittenWithTheRustColumnFormat(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/rust")
	nullsID := uuid.New()
	valuesID := uuid.New()
	insertTaskRow(t, repo, rustTaskRow{
		id: nullsID.String(), workspaceID: workspaceID, title: "nulls",
		status: "rate_limited", priority: "urgent",
		createdAt: "2026-10-24 10:00:00", updatedAt: "2026-10-24 10:00:01.250",
		pendingDialogs:  `[{"id":"d1","expires_at":"2026-10-24T11:00:00.250Z"}]`,
		subagentsActive: 1, subagentsWaiting: 0,
		cpuPercent: ptr(int64(150)), resumeAt: ptr("2026-10-24 12:00:00.5"), resumeAttempts: 2,
	})
	insertTaskRow(t, repo, rustTaskRow{
		id: valuesID.String(), workspaceID: workspaceID, title: "values",
		status: "merged", priority: "normal",
		createdAt: "2026-10-24 09:00:00", updatedAt: "2026-10-24 09:30:00",
		pendingDialogs: `["d0"]`,
		errorText:      ptr("texto"), baseSHA: ptr("aaa111"), headSHA: ptr("bbb222"), memoryMB: ptr(int64(512)),
	})
	nullsTask := models.TaskIDFromUUID(nullsID)
	valuesTask := models.TaskIDFromUUID(valuesID)

	nulls := found(t, repo, nullsTask)
	assert.Equal(t, models.StatusRateLimited, nulls.Status)
	assert.Equal(t, models.PriorityUrgent, nulls.Priority)
	assert.Nil(t, nulls.Error, "NULL error reads as no error")
	assert.Nil(t, nulls.DiffRange, "NULL base_sha and head_sha read as no recorded range")
	assert.Equal(t, models.ResourceLimits{CPUPercent: ptr(uint32(150))}, nulls.Limits, "NULL memory limit reads as unset")
	assert.True(t, time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC).Equal(nulls.CreatedAt), "created_at without fraction, got %s", nulls.CreatedAt)
	assert.True(t, time.Date(2026, 10, 24, 10, 0, 1, 250_000_000, time.UTC).Equal(nulls.UpdatedAt), "updated_at with fraction, got %s", nulls.UpdatedAt)
	require.Len(t, nulls.PendingDialogs, 1)
	assert.Equal(t, "d1", nulls.PendingDialogs[0].ID)
	require.NotNil(t, nulls.PendingDialogs[0].ExpiresAt)
	assert.True(t, time.Date(2026, 10, 24, 11, 0, 0, 250_000_000, time.UTC).Equal(*nulls.PendingDialogs[0].ExpiresAt))
	assert.Equal(t, models.NewSubagentActivity(1, 0), nulls.Subagents)
	assert.True(t, time.Date(2026, 10, 24, 12, 0, 0, 500_000_000, time.UTC).Equal(*nulls.UsageLimit.ResumeAt))
	assert.Equal(t, uint32(2), nulls.UsageLimit.Attempts)

	values := found(t, repo, valuesTask)
	assert.Equal(t, models.StatusMerged, values.Status)
	assert.Equal(t, models.PriorityNormal, values.Priority)
	require.NotNil(t, values.Error)
	assert.Equal(t, "texto", *values.Error)
	assert.Equal(t, &models.DiffRange{BaseSHA: "aaa111", HeadSHA: "bbb222"}, values.DiffRange)
	assert.Equal(t, models.ResourceLimits{MemoryMB: ptr(uint32(512))}, values.Limits, "NULL cpu limit reads as unset")
	assert.True(t, time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC).Equal(values.CreatedAt))
	assert.True(t, time.Date(2026, 10, 24, 9, 30, 0, 0, time.UTC).Equal(values.UpdatedAt))
	require.Len(t, values.PendingDialogs, 1)
	assert.Equal(t, "d0", values.PendingDialogs[0].ID, "the legacy array of ids is still read")
	assert.Nil(t, values.PendingDialogs[0].ExpiresAt)
	assert.Nil(t, values.UsageLimit.ResumeAt, "NULL resume_at reads as not limited")

	listed, err := repo.Tasks.List(bg(), &workspaceID)
	require.NoError(t, err)
	assert.Equal(t, []string{"nulls", "values"}, titles(listed), "newest first over the same rows")

	done, err := repo.Tasks.ListDonePage(bg(), &workspaceID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"values"}, titles(done), "only the closed row is done")
}

// Rust: none. The Go writer must produce the text the Rust reader parses: no fraction when the
// instant is a whole second, the shortest fraction otherwise, and the same for resume_at.
func TestWritesTimestampsInTheRustColumnFormat(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/rust")
	task := newTask(t, workspaceID, "uno", models.StatusBacklog)
	insertTask(t, repo, task)
	const createdQuery = "SELECT CAST(created_at AS TEXT) FROM tasks WHERE id = ?"
	const updatedQuery = "SELECT CAST(updated_at AS TEXT) FROM tasks WHERE id = ?"
	const resumeQuery = "SELECT CAST(resume_at AS TEXT) FROM tasks WHERE id = ?"

	assert.Equal(t, "2026-10-24 10:00:00", storedText(t, repo, createdQuery, task.ID))
	assert.Equal(t, "2026-10-24 10:00:00", storedText(t, repo, updatedQuery, task.ID))

	repo.advance(250 * time.Millisecond)
	require.NoError(t, repo.Tasks.UpdateStatus(bg(), task.ID, models.StatusRunning, nil))
	assert.Equal(t, "2026-10-24 10:00:00.25", storedText(t, repo, updatedQuery, task.ID))

	resumeAt := fixedClock.Add(time.Hour + 500*time.Millisecond)
	require.NoError(t, repo.Tasks.SetUsageLimit(bg(), task.ID, models.UsageLimitState{ResumeAt: &resumeAt, Attempts: 1}))
	assert.Equal(t, "2026-10-24 11:00:00.5", storedText(t, repo, resumeQuery, task.ID))
}

// Rust: none. A TIMESTAMP text the reader cannot parse is a repository error, and the row stays exactly
// as it was stored, so a bad row never gets rewritten by a read.
func TestRejectsARowWhoseTimestampCannotBeReadAndLeavesItUnchanged(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/rust")
	id := uuid.New()
	insertTaskRow(t, repo, rustTaskRow{
		id: id.String(), workspaceID: workspaceID, title: "roto",
		status: "running", priority: "normal",
		createdAt: "2026-13-45 99:00:00", updatedAt: "2026-10-24 10:00:00",
		pendingDialogs: "[]",
	})
	taskID := models.TaskIDFromUUID(id)
	const createdQuery = "SELECT CAST(created_at AS TEXT) FROM tasks WHERE id = ?"

	got, err := repo.Tasks.Find(bg(), taskID)

	assert.ErrorIs(t, err, werrors.Repository(""))
	assert.Nil(t, got)
	assert.Equal(t, "2026-13-45 99:00:00", storedText(t, repo, createdQuery, taskID))
}
