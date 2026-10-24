package tasks_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
)

// attachmentFixture is the metadata of an image in the repository tests; the id encodes the name and the path.
func attachmentFixture(name string, path string) models.StoredAttachment {
	return models.StoredAttachment{
		ID:        "id-" + name + "-" + path,
		Name:      name,
		MimeType:  "image/png",
		Path:      path,
		SizeBytes: 1234,
	}
}

// Rust: task_attachment_repository_adapter_test::lists_images_in_insertion_order
func TestAttachmentsListInInsertionOrder(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	task := newTask(t, insertWorkspace(t, repo.DB, "/repos/uno"), "uno", models.StatusBacklog)
	insertTask(t, repo, task)
	images := []models.StoredAttachment{
		attachmentFixture("c.png", "t/c.png"),
		attachmentFixture("a.png", "t/a.png"),
		attachmentFixture("b.png", "t/b.png"),
	}

	require.NoError(t, repo.Attachments.InsertMany(bg(), task.ID, images))

	stored, err := repo.Attachments.ListByTask(bg(), task.ID)
	require.NoError(t, err)
	assert.Equal(t, images, stored)
	assert.Equal(t, "t/c.png", stored[0].Path)
	assert.Equal(t, uint64(1234), stored[0].SizeBytes)
}

// Rust: task_attachment_repository_adapter_test::lists_only_the_images_of_the_requested_task
func TestAttachmentsListOnlyTheRequestedTasksImages(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	first := newTask(t, workspaceID, "uno", models.StatusBacklog)
	second := newTask(t, workspaceID, "dos", models.StatusBacklog)
	insertTask(t, repo, first)
	insertTask(t, repo, second)
	require.NoError(t, repo.Attachments.InsertMany(bg(), first.ID, []models.StoredAttachment{attachmentFixture("a.png", "t/a.png")}))
	require.NoError(t, repo.Attachments.InsertMany(bg(), second.ID, []models.StoredAttachment{attachmentFixture("b.png", "t/b.png")}))

	stored, err := repo.Attachments.ListByTask(bg(), second.ID)
	require.NoError(t, err)
	assert.Equal(t, []models.StoredAttachment{attachmentFixture("b.png", "t/b.png")}, stored)

	missing, err := repo.Attachments.ListByTask(bg(), newTaskID(t))
	require.NoError(t, err)
	assert.Empty(t, missing)
}

// Rust: task_attachment_repository_adapter_test::inserting_nothing_is_a_no_op
func TestAttachmentsInsertingNothingIsANoOp(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	task := newTask(t, insertWorkspace(t, repo.DB, "/repos/uno"), "uno", models.StatusBacklog)
	insertTask(t, repo, task)

	require.NoError(t, repo.Attachments.InsertMany(bg(), task.ID, nil))

	stored, err := repo.Attachments.ListByTask(bg(), task.ID)
	require.NoError(t, err)
	assert.Empty(t, stored)
}

// Rust: task_attachment_repository_adapter_test::insert_many_is_atomic_when_the_task_does_not_exist
func TestAttachmentsInsertManyIsAtomicWhenTheTaskDoesNotExist(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	missing := newTaskID(t)

	err := repo.Attachments.InsertMany(bg(), missing, []models.StoredAttachment{
		attachmentFixture("a.png", "t/a.png"),
		attachmentFixture("b.png", "t/b.png"),
	})

	assert.ErrorContains(t, err, "persistencia")
	stored, listErr := repo.Attachments.ListByTask(bg(), missing)
	require.NoError(t, listErr)
	assert.Empty(t, stored)
}

// Rust: task_attachment_repository_adapter_test::images_are_deleted_with_their_task
func TestAttachmentsAreDeletedWithTheirTask(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	task := newTask(t, insertWorkspace(t, repo.DB, "/repos/uno"), "uno", models.StatusBacklog)
	insertTask(t, repo, task)
	require.NoError(t, repo.Attachments.InsertMany(bg(), task.ID, []models.StoredAttachment{attachmentFixture("a.png", "t/a.png")}))

	deleted, err := repo.Tasks.DeleteIfBacklog(bg(), task.ID)

	require.NoError(t, err)
	assert.True(t, deleted)
	stored, err := repo.Attachments.ListByTask(bg(), task.ID)
	require.NoError(t, err)
	assert.Empty(t, stored)
}

// Rust: task_attachment_repository_adapter_test::images_are_deleted_with_their_workspace_tasks
func TestAttachmentsAreDeletedWithTheirWorkspaceTasks(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/uno")
	task := newTask(t, workspaceID, "uno", models.StatusBacklog)
	insertTask(t, repo, task)
	require.NoError(t, repo.Tasks.UpdateStatus(bg(), task.ID, models.StatusMerged, nil))
	require.NoError(t, repo.Attachments.InsertMany(bg(), task.ID, []models.StoredAttachment{attachmentFixture("a.png", "t/a.png")}))

	ids, deleted, err := repo.Tasks.DeleteInactiveByWorkspace(bg(), workspaceID)

	require.NoError(t, err)
	assert.True(t, deleted)
	assert.Equal(t, []models.TaskID{task.ID}, ids)
	stored, err := repo.Attachments.ListByTask(bg(), task.ID)
	require.NoError(t, err)
	assert.Empty(t, stored)
}

// Rust: task_attachment_repository_adapter_test::inserting_again_appends_after_the_existing_images
func TestAttachmentsInsertingAgainAppendsAfterTheExistingImages(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	task := newTask(t, insertWorkspace(t, repo.DB, "/repos/uno"), "append", models.StatusBacklog)
	insertTask(t, repo, task)
	first := []models.StoredAttachment{attachmentFixture("a.png", "t/a.png"), attachmentFixture("b.png", "t/b.png")}
	second := []models.StoredAttachment{attachmentFixture("c.png", "t/c.png"), attachmentFixture("d.png", "t/d.png")}
	require.NoError(t, repo.Attachments.InsertMany(bg(), task.ID, first))
	require.NoError(t, repo.Attachments.InsertMany(bg(), task.ID, second))

	stored, err := repo.Attachments.ListByTask(bg(), task.ID)
	require.NoError(t, err)
	assert.Equal(t, append(first, second...), stored)
}

// Rust: task_attachment_repository_adapter_test::appending_to_a_task_does_not_depend_on_other_tasks_positions
func TestAttachmentsAppendingDoesNotDependOnOtherTasksPositions(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	first := newTask(t, workspaceID, "uno", models.StatusBacklog)
	second := newTask(t, workspaceID, "dos", models.StatusBacklog)
	insertTask(t, repo, first)
	insertTask(t, repo, second)
	require.NoError(t, repo.Attachments.InsertMany(bg(), first.ID, []models.StoredAttachment{attachmentFixture("a.png", "t/a.png"), attachmentFixture("b.png", "t/b.png")}))

	require.NoError(t, repo.Attachments.InsertMany(bg(), second.ID, []models.StoredAttachment{attachmentFixture("c.png", "t/c.png")}))

	stored, err := repo.Attachments.ListByTask(bg(), second.ID)
	require.NoError(t, err)
	assert.Equal(t, []models.StoredAttachment{attachmentFixture("c.png", "t/c.png")}, stored)
}

// Rust: task_attachment_repository_adapter_test::remove_deletes_only_the_requested_rows_of_that_task
func TestAttachmentsRemoveDeletesOnlyTheRequestedRowsOfThatTask(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	first := newTask(t, workspaceID, "uno", models.StatusBacklog)
	second := newTask(t, workspaceID, "dos", models.StatusBacklog)
	insertTask(t, repo, first)
	insertTask(t, repo, second)
	images := []models.StoredAttachment{attachmentFixture("a.png", "t/a.png"), attachmentFixture("b.png", "t/b.png"), attachmentFixture("c.png", "t/c.png")}
	require.NoError(t, repo.Attachments.InsertMany(bg(), first.ID, images))
	foreign := attachmentFixture("x.png", "t/x.png")
	require.NoError(t, repo.Attachments.InsertMany(bg(), second.ID, []models.StoredAttachment{foreign}))

	removed, err := repo.Attachments.Remove(bg(), first.ID, []string{images[1].ID, foreign.ID, "unknown"})

	require.NoError(t, err)
	assert.Equal(t, []models.StoredAttachment{images[1]}, removed)
	remaining, err := repo.Attachments.ListByTask(bg(), first.ID)
	require.NoError(t, err)
	assert.Equal(t, []models.StoredAttachment{images[0], images[2]}, remaining)
	other, err := repo.Attachments.ListByTask(bg(), second.ID)
	require.NoError(t, err)
	assert.Equal(t, []models.StoredAttachment{foreign}, other)
}

// Rust: task_attachment_repository_adapter_test::remove_with_no_ids_is_a_no_op_and_insert_appends_after_gaps
func TestAttachmentsRemoveWithNoIDsIsANoOpAndInsertAppendsAfterGaps(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	task := newTask(t, insertWorkspace(t, repo.DB, "/repos/uno"), "uno", models.StatusBacklog)
	insertTask(t, repo, task)
	images := []models.StoredAttachment{attachmentFixture("a.png", "t/a.png"), attachmentFixture("b.png", "t/b.png")}
	require.NoError(t, repo.Attachments.InsertMany(bg(), task.ID, images))

	removed, err := repo.Attachments.Remove(bg(), task.ID, nil)
	require.NoError(t, err)
	assert.Empty(t, removed)
	_, err = repo.Attachments.Remove(bg(), task.ID, []string{images[0].ID})
	require.NoError(t, err)
	appended := attachmentFixture("c.png", "t/c.png")
	require.NoError(t, repo.Attachments.InsertMany(bg(), task.ID, []models.StoredAttachment{appended}))

	stored, err := repo.Attachments.ListByTask(bg(), task.ID)
	require.NoError(t, err)
	assert.Equal(t, []models.StoredAttachment{images[1], appended}, stored)
}

// Rust: task_attachment_repository_adapter_test::delete_by_task_removes_only_that_tasks_rows
func TestAttachmentsDeleteByTaskRemovesOnlyThatTasksRows(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	first := newTask(t, workspaceID, "uno", models.StatusBacklog)
	second := newTask(t, workspaceID, "dos", models.StatusBacklog)
	insertTask(t, repo, first)
	insertTask(t, repo, second)
	require.NoError(t, repo.Attachments.InsertMany(bg(), first.ID, []models.StoredAttachment{attachmentFixture("a.png", "t/a.png")}))
	require.NoError(t, repo.Attachments.InsertMany(bg(), second.ID, []models.StoredAttachment{attachmentFixture("b.png", "t/b.png")}))

	require.NoError(t, repo.Attachments.DeleteByTask(bg(), first.ID))
	require.NoError(t, repo.Attachments.DeleteByTask(bg(), newTaskID(t)), "a missing task is fine")

	firstLeft, err := repo.Attachments.ListByTask(bg(), first.ID)
	require.NoError(t, err)
	assert.Empty(t, firstLeft)
	secondLeft, err := repo.Attachments.ListByTask(bg(), second.ID)
	require.NoError(t, err)
	assert.Len(t, secondLeft, 1)
}

// Go-only: appends from concurrent goroutines must get distinct positions. Without the write lock around
// the maximum-position read and the insert, two appends could pick the same position and one would fail
// on the unique index.
func TestAttachmentsConcurrentAppendsGetDistinctPositions(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	task := newTask(t, insertWorkspace(t, repo.DB, "/repos/race"), "race", models.StatusBacklog)
	insertTask(t, repo, task)
	const appends = 12

	var wg sync.WaitGroup
	failures := make(chan error, appends)
	for index := range appends {
		wg.Add(1)
		go func() {
			defer wg.Done()
			attachment := attachmentFixture(fmt.Sprintf("img%02d.png", index), fmt.Sprintf("t/%02d.png", index))
			if err := repo.Attachments.InsertMany(bg(), task.ID, []models.StoredAttachment{attachment}); err != nil {
				failures <- err
			}
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		require.NoError(t, err)
	}

	stored, err := repo.Attachments.ListByTask(bg(), task.ID)
	require.NoError(t, err)
	assert.Len(t, stored, appends)
	paths := make(map[string]bool, appends)
	for _, attachment := range stored {
		paths[attachment.Path] = true
	}
	assert.Len(t, paths, appends, "every append must be stored exactly once")
}
