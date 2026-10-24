package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// fakeTaskFiles records the writes and reads of the worktree files and answers with configured errors.
type fakeTaskFiles struct {
	writes   []string
	writeErr error
	readErr  error
	content  models.TaskFileContent
}

func (f *fakeTaskFiles) Write(_ context.Context, root string, rel string, content string) error {
	f.writes = append(f.writes, root+"|"+rel+"|"+content)
	return f.writeErr
}

func (f *fakeTaskFiles) Read(_ context.Context, root string, rel string) (models.TaskFileContent, error) {
	if f.readErr != nil {
		return models.TaskFileContent{}, f.readErr
	}
	return f.content, nil
}

// fakeCommitter records the commits it is asked for and answers with a configured result.
type fakeCommitter struct {
	commits   []string
	committed bool
	err       error
}

func (c *fakeCommitter) CommitAll(_ context.Context, path string, message string) (bool, error) {
	c.commits = append(c.commits, path+"|"+message)
	return c.committed, c.err
}

// Rust: save_task_file::writes_then_commits_in_the_worktree
func TestSaveTaskFileWritesThenCommitsInTheWorktree(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle)
	lifecycle, _ := newLifecycle(newFakeRepository(task))
	files := &fakeTaskFiles{}
	committer := &fakeCommitter{committed: true}

	committed, err := application.NewSaveTaskFile(lifecycle, files, committer).Execute(context.Background(), task.ID, "src/a.rs", "fn b() {}")

	require.NoError(t, err)
	assert.True(t, committed)
	assert.Equal(t, []string{task.WorktreePath + "|src/a.rs|fn b() {}"}, files.writes)
	assert.Equal(t, []string{task.WorktreePath + "|edit src/a.rs from diff view"}, committer.commits)
}

// Rust: save_task_file::reports_no_commit_when_content_is_identical
func TestSaveTaskFileReportsNoCommitWhenNothingChanged(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusAwaitingReview)
	lifecycle, _ := newLifecycle(newFakeRepository(task))

	committed, err := application.NewSaveTaskFile(lifecycle, &fakeTaskFiles{}, &fakeCommitter{committed: false}).Execute(context.Background(), task.ID, "a.rs", "x")

	require.NoError(t, err)
	assert.False(t, committed)
}

// Rust: save_task_file::rejects_tasks_without_worktree
func TestSaveTaskFileRejectsTasksWithoutWorktree(t *testing.T) {
	t.Parallel()
	for _, status := range []models.TaskStatus{models.StatusBacklog, models.StatusMerged, models.StatusDiscarded} {
		task := newTask(status)
		lifecycle, _ := newLifecycle(newFakeRepository(task))
		files := &fakeTaskFiles{}
		committer := &fakeCommitter{}

		_, err := application.NewSaveTaskFile(lifecycle, files, committer).Execute(context.Background(), task.ID, "a.rs", "x")

		assert.ErrorIs(t, err, werrors.InvalidState("guardar ficheros", ""), string(status))
		assert.Empty(t, files.writes)
		assert.Empty(t, committer.commits)
	}
}

// Rust: save_task_file::write_error_stops_before_commit
func TestSaveTaskFileWriteErrorStopsBeforeCommit(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle)
	lifecycle, _ := newLifecycle(newFakeRepository(task))
	committer := &fakeCommitter{}

	_, err := application.NewSaveTaskFile(lifecycle, &fakeTaskFiles{writeErr: werrors.FileNotEditable("a")}, committer).Execute(context.Background(), task.ID, "a.rs", "x")

	assert.ErrorIs(t, err, werrors.FileNotEditable(""))
	assert.Empty(t, committer.commits)
}

// Rust: save_task_file::commit_error_is_propagated
func TestSaveTaskFileCommitErrorIsPropagated(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle)
	lifecycle, _ := newLifecycle(newFakeRepository(task))

	_, err := application.NewSaveTaskFile(lifecycle, &fakeTaskFiles{}, &fakeCommitter{err: werrors.Git("sin rama")}).Execute(context.Background(), task.ID, "a.rs", "x")

	assert.ErrorIs(t, err, werrors.Git(""))
}

// Rust: get_task_file::reads_from_the_task_worktree
func TestGetTaskFileReadsFromTheTaskWorktree(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	lifecycle, _ := newLifecycle(newFakeRepository(task))
	files := &fakeTaskFiles{content: models.TaskFileContent{Path: "src/a.rs", Content: "fn a() {}"}}

	content, err := application.NewGetTaskFile(lifecycle, files).Execute(context.Background(), task.ID, "src/a.rs")

	require.NoError(t, err)
	assert.Equal(t, files.content, content)
}

// Rust: get_task_file::rejects_tasks_without_worktree
func TestGetTaskFileRejectsTasksWithoutWorktree(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusDiscarded)
	lifecycle, _ := newLifecycle(newFakeRepository(task))

	_, err := application.NewGetTaskFile(lifecycle, &fakeTaskFiles{}).Execute(context.Background(), task.ID, "a.rs")

	assert.ErrorIs(t, err, werrors.InvalidState("leer ficheros", ""))
}

// Rust: get_task_file::propagates_port_errors
func TestGetTaskFilePropagatesPortErrors(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	lifecycle, _ := newLifecycle(newFakeRepository(task))

	_, err := application.NewGetTaskFile(lifecycle, &fakeTaskFiles{readErr: errors.New("boom")}).Execute(context.Background(), task.ID, "a.rs")

	assert.EqualError(t, err, "boom")
}
