package persistence_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/infrastructure/adapters/outbound/persistence"
)

const testWorkspaceID = "11111111-1111-4111-8111-111111111111"

// seedWorkspace stores one workspace and returns its id.
func seedWorkspace(t *testing.T, handle *sql.DB) models.WorkspaceID {
	t.Helper()
	id, err := models.ParseWorkspaceID(testWorkspaceID)
	require.NoError(t, err)
	repository := persistence.NewWorkspaceRepository(handle)
	require.NoError(t, repository.Save(context.Background(), models.Workspace{
		ID:         id,
		Name:       "demo",
		Path:       "/tmp/demo",
		BaseBranch: "main",
		CreatedAt:  time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
	}))
	return id
}

// seedTask inserts one task with the given status; the tests need the rows, not the task model.
func seedTask(t *testing.T, handle *sql.DB, taskID string, status string) {
	t.Helper()
	const insert = `INSERT INTO tasks (id, workspace_id, title, prompt, branch, base_branch, worktree_path,
		status, created_at, updated_at) VALUES (?, ?, 'title', 'prompt', 'branch', 'main', '/tmp/wt', ?,
		'2026-01-01 10:00:00', '2026-01-01 10:00:00')`
	_, err := handle.ExecContext(context.Background(), insert, taskID, testWorkspaceID, status)
	require.NoError(t, err)
}

func countTasks(t *testing.T, handle *sql.DB) int {
	t.Helper()
	var count int
	require.NoError(t, handle.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM tasks").Scan(&count))
	return count
}

func TestDeleteAllWithoutTasksSucceeds(t *testing.T) {
	handle := dbtest.New(t)
	id := seedWorkspace(t, handle)

	err := persistence.NewTemporaryWorkspaceTasks(handle).DeleteAll(context.Background(), id)

	require.NoError(t, err)
	require.Equal(t, 0, countTasks(t, handle))
}

func TestDeleteAllRemovesInactiveTasks(t *testing.T) {
	handle := dbtest.New(t)
	id := seedWorkspace(t, handle)
	seedTask(t, handle, "task-backlog", "backlog")
	seedTask(t, handle, "task-merged", "merged")
	seedTask(t, handle, "task-discarded", "discarded")

	err := persistence.NewTemporaryWorkspaceTasks(handle).DeleteAll(context.Background(), id)

	require.NoError(t, err)
	require.Equal(t, 0, countTasks(t, handle))
}

func TestDeleteAllRefusesWhenATaskIsActive(t *testing.T) {
	handle := dbtest.New(t)
	id := seedWorkspace(t, handle)
	seedTask(t, handle, "task-merged", "merged")
	seedTask(t, handle, "task-running", "running")

	err := persistence.NewTemporaryWorkspaceTasks(handle).DeleteAll(context.Background(), id)

	require.ErrorIs(t, err, werrors.InUse())
	// Nothing is deleted when any task blocks the workspace.
	require.Equal(t, 2, countTasks(t, handle))
}
