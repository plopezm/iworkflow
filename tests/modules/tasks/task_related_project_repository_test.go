package tasks_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
)

// relatedFixture is a task in backlog plus the workspaces it can relate to.
type relatedFixture struct {
	repo    *Repo
	task    models.Task
	first   string
	second  string
	third   string
	unknown models.TaskID
	ownerWS string
}

func newRelatedFixture(t *testing.T) relatedFixture {
	t.Helper()
	repo := newRepo(t)
	owner := insertWorkspace(t, repo.DB, "/repos/owner")
	task := newTask(t, owner, "uno", models.StatusBacklog)
	insertTask(t, repo, task)
	unknown, err := models.NewTaskID()
	require.NoError(t, err)
	return relatedFixture{
		repo:    repo,
		task:    task,
		first:   insertWorkspace(t, repo.DB, "/repos/a"),
		second:  insertWorkspace(t, repo.DB, "/repos/b"),
		third:   insertWorkspace(t, repo.DB, "/repos/c"),
		unknown: unknown,
		ownerWS: owner,
	}
}

// Replace stores the list in the order given, and ListByTask returns it in that order.
func TestRelatedProjectsReplaceStoresInOrder(t *testing.T) {
	t.Parallel()
	f := newRelatedFixture(t)

	require.NoError(t, f.repo.RelatedProjects.Replace(bg(), f.task.ID, []string{f.third, f.first, f.second}))

	stored, err := f.repo.RelatedProjects.ListByTask(bg(), f.task.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{f.third, f.first, f.second}, stored)
}

// A second Replace swaps the whole list: nothing of the previous one remains.
func TestRelatedProjectsReplaceOverwritesPreviousList(t *testing.T) {
	t.Parallel()
	f := newRelatedFixture(t)
	require.NoError(t, f.repo.RelatedProjects.Replace(bg(), f.task.ID, []string{f.first, f.second}))

	require.NoError(t, f.repo.RelatedProjects.Replace(bg(), f.task.ID, []string{f.third}))

	stored, err := f.repo.RelatedProjects.ListByTask(bg(), f.task.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{f.third}, stored)
}

// An empty list clears the related projects of the task.
func TestRelatedProjectsEmptyListClears(t *testing.T) {
	t.Parallel()
	f := newRelatedFixture(t)
	require.NoError(t, f.repo.RelatedProjects.Replace(bg(), f.task.ID, []string{f.first}))

	require.NoError(t, f.repo.RelatedProjects.Replace(bg(), f.task.ID, nil))

	stored, err := f.repo.RelatedProjects.ListByTask(bg(), f.task.ID)
	require.NoError(t, err)
	assert.Empty(t, stored)
}

// A task that was never stored has no related projects, and listing it is not an error.
func TestRelatedProjectsListOfUnknownTaskIsEmpty(t *testing.T) {
	t.Parallel()
	f := newRelatedFixture(t)

	stored, err := f.repo.RelatedProjects.ListByTask(bg(), f.unknown)
	require.NoError(t, err)
	assert.Empty(t, stored)
}

// A workspace that does not exist fails the whole Replace, and the previous list stays as it was.
func TestRelatedProjectsReplaceWithMissingWorkspaceKeepsPreviousList(t *testing.T) {
	t.Parallel()
	f := newRelatedFixture(t)
	require.NoError(t, f.repo.RelatedProjects.Replace(bg(), f.task.ID, []string{f.first}))

	err := f.repo.RelatedProjects.Replace(bg(), f.task.ID, []string{f.second, uuid.NewString()})

	require.Error(t, err)
	stored, listErr := f.repo.RelatedProjects.ListByTask(bg(), f.task.ID)
	require.NoError(t, listErr)
	assert.Equal(t, []string{f.first}, stored, "a failed replace must not leave a partial list")
}

// A task that does not exist cannot receive related projects.
func TestRelatedProjectsReplaceForMissingTaskFails(t *testing.T) {
	t.Parallel()
	f := newRelatedFixture(t)

	err := f.repo.RelatedProjects.Replace(bg(), f.unknown, []string{f.first})

	require.Error(t, err)
	stored, listErr := f.repo.RelatedProjects.ListByTask(bg(), f.unknown)
	require.NoError(t, listErr)
	assert.Empty(t, stored)
}

// Deleting the task removes its related projects.
func TestRelatedProjectsCascadeWhenTaskIsDeleted(t *testing.T) {
	t.Parallel()
	f := newRelatedFixture(t)
	require.NoError(t, f.repo.RelatedProjects.Replace(bg(), f.task.ID, []string{f.first, f.second}))

	deleted, err := f.repo.Tasks.DeleteIfBacklog(bg(), f.task.ID)
	require.NoError(t, err)
	require.True(t, deleted)

	stored, err := f.repo.RelatedProjects.ListByTask(bg(), f.task.ID)
	require.NoError(t, err)
	assert.Empty(t, stored)
	assertRowCount(t, f.repo, "SELECT COUNT(*) FROM task_related_projects WHERE task_id = ?", f.task.ID.String(), 0)
}

// Deleting a workspace removes the references to it, and keeps the other related projects of the task.
func TestRelatedProjectsCascadeWhenWorkspaceIsDeleted(t *testing.T) {
	t.Parallel()
	f := newRelatedFixture(t)
	require.NoError(t, f.repo.RelatedProjects.Replace(bg(), f.task.ID, []string{f.first, f.second}))

	_, err := f.repo.DB.ExecContext(bg(), "DELETE FROM workspaces WHERE id = ?", f.first)
	require.NoError(t, err)

	stored, err := f.repo.RelatedProjects.ListByTask(bg(), f.task.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{f.second}, stored)
}

// assertRowCount checks the result of a COUNT query with one bound argument.
func assertRowCount(t *testing.T, repo *Repo, query string, arg string, want int) {
	t.Helper()
	var count int
	require.NoError(t, repo.DB.QueryRowContext(context.Background(), query, arg).Scan(&count))
	assert.Equal(t, want, count)
}
