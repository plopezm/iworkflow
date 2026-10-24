package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// Parity ports of the Rust use case tests that the first pass did not cover. Each test names the
// Rust test it mirrors.

// Rust: create_task::skips_attachment_storage_when_there_are_no_images
func TestCreateTaskSkipsAttachmentStorageWhenThereAreNoImages(t *testing.T) {
	t.Parallel()
	storage := &fakeStorage{}
	attachments := &fakeAttachments{}
	useCase, _ := createUseCase(newFakeRepository(), &fakeStarter{}, attachments, storage)

	_, err := useCase.Execute(context.Background(), newInput("sin imágenes"))

	require.NoError(t, err)
	assert.Empty(t, storage.saved)
	assert.Empty(t, attachments.inserted)
}

// Rust: create_task::cleans_up_and_returns_the_original_error_when_saving_a_file_fails
func TestCreateTaskReturnsTheOriginalErrorWhenSavingAFileFails(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	storage := &fakeStorage{failSaveFrom: 2, failDeleteDir: true}
	attachments := &fakeAttachments{}
	useCase, _ := createUseCase(repo, &fakeStarter{}, attachments, storage)
	input := newInput("dos imágenes")
	input.Images = []models.ImageAttachment{pngImage("a.png"), pngImage("b.png")}

	_, err := useCase.Execute(context.Background(), input)

	assert.ErrorIs(t, err, werrors.Storage("sin espacio"))
	assert.Empty(t, attachments.inserted, "no metadata is written when a file fails")
	assert.Len(t, storage.deletedDirs, 1, "the cleanup is attempted once; its failure does not replace the error")
}

// Rust: create_task::fails_when_insert_fails
func TestCreateTaskFailsWhenInsertFails(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	repo.insertErr = werrors.Repository("boom")
	starter := &fakeStarter{}
	useCase, _ := createUseCase(repo, starter, &fakeAttachments{}, &fakeStorage{})
	input := newInput("falla")
	input.StartNow = true

	_, err := useCase.Execute(context.Background(), input)

	assert.ErrorIs(t, err, werrors.Repository("boom"))
	assert.Empty(t, starter.started, "a task that was not stored is never started")
}

// Rust: create_task::keeps_requested_priority_on_the_created_task
func TestCreateTaskKeepsTheRequestedPriority(t *testing.T) {
	t.Parallel()
	useCase, _ := createUseCase(newFakeRepository(), &fakeStarter{}, &fakeAttachments{}, &fakeStorage{})
	input := newInput("urgente")
	input.Priority = models.PriorityUrgent

	created, err := useCase.Execute(context.Background(), input)

	require.NoError(t, err)
	assert.Equal(t, models.PriorityUrgent, created.Task.Priority)
}

// Rust: update_task::updates_priority_and_persists_it_through_the_port
func TestUpdatePersistsThePriorityThroughTheRepository(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	repo := newFakeRepository(task)
	useCase := updateUseCase(repo, &fakeAttachments{}, &fakeStorage{})
	input := baseUpdate("x", "y")
	input.Priority = models.PriorityUrgent

	updated, err := useCase.Execute(context.Background(), task.ID, input)

	require.NoError(t, err)
	assert.Equal(t, models.PriorityUrgent, updated.Priority)
	assert.Equal(t, models.PriorityUrgent, repo.tasks[task.ID].Priority)
}

// Rust: update_task::persists_the_chosen_base_branch (the request is trimmed)
func TestUpdatePersistsTheChosenBaseBranch(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	repo := newFakeRepository(task)
	useCase := updateUseCase(repo, &fakeAttachments{}, &fakeStorage{})
	branch := " origin/develop "
	input := baseUpdate("x", "y")
	input.BaseBranch = &branch

	updated, err := useCase.Execute(context.Background(), task.ID, input)

	require.NoError(t, err)
	assert.Equal(t, "origin/develop", updated.BaseBranch)
	assert.Equal(t, "origin/develop", repo.tasks[task.ID].BaseBranch)
}

// Rust: update_task::falls_back_to_the_workspace_base_branch_without_a_choice
func TestUpdateFallsBackToTheWorkspaceBaseBranchWithoutAChoice(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	task.BaseBranch = "origin/develop"
	useCase := updateUseCase(newFakeRepository(task), &fakeAttachments{}, &fakeStorage{})

	updated, err := useCase.Execute(context.Background(), task.ID, baseUpdate("x", "y"))

	require.NoError(t, err)
	assert.Equal(t, "main", updated.BaseBranch)
}

// Rust: update_task::derives_title_from_prompt_when_blank
func TestUpdateDerivesTheTitleFromThePromptWhenBlank(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	useCase := updateUseCase(newFakeRepository(task), &fakeAttachments{}, &fakeStorage{})

	updated, err := useCase.Execute(context.Background(), task.ID, baseUpdate("  ", "Arregla el test\nmás detalles"))

	require.NoError(t, err)
	assert.Equal(t, "Arregla el test", updated.Title)
}

// Rust: update_task::rejects_empty_prompt
func TestUpdateRejectsEmptyPrompt(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	useCase := updateUseCase(newFakeRepository(task), &fakeAttachments{}, &fakeStorage{})

	_, err := useCase.Execute(context.Background(), task.ID, baseUpdate("x", " \n "))

	assert.ErrorIs(t, err, werrors.EmptyPrompt())
}

// Rust: update_task::stores_new_images_after_updating_details
func TestUpdateStoresNewImagesAfterUpdatingDetails(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	attachments := &fakeAttachments{listed: []models.StoredAttachment{{ID: "old", Path: "old.png"}}}
	storage := &fakeStorage{}
	useCase := updateUseCase(newFakeRepository(task), attachments, storage)
	input := baseUpdate("x", "y")
	input.Images = []models.ImageAttachment{pngImage("a.png"), pngImage("b.png")}

	_, err := useCase.Execute(context.Background(), task.ID, input)

	require.NoError(t, err)
	assert.Len(t, storage.saved, 2)
	require.Len(t, attachments.inserted, 1)
	assert.Len(t, attachments.inserted[0], 2)
}

// Rust: update_task::rejects_invalid_images (checked before the repository is touched)
func TestUpdateRejectsInvalidImagesBeforeTouchingTheTask(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	repo := newFakeRepository(task)
	useCase := updateUseCase(repo, &fakeAttachments{}, &fakeStorage{})
	input := baseUpdate("nuevo", "y")
	input.Images = []models.ImageAttachment{{Name: "a.bmp", MimeType: "image/bmp", Data: "aGk="}}

	_, err := useCase.Execute(context.Background(), task.ID, input)

	assert.ErrorIs(t, err, werrors.InvalidAttachment(""))
	assert.Equal(t, "Arregla el login", repo.tasks[task.ID].Title, "the title is not changed")
}

// Rust: update_task::deletes_only_new_files_when_registering_them_fails
func TestUpdateDeletesOnlyTheNewFilesWhenRegisteringFails(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	attachments := &fakeAttachments{
		listed:     []models.StoredAttachment{{ID: "old", Path: "old.png"}},
		failInsert: true,
	}
	storage := &fakeStorage{failDelete: true}
	useCase := updateUseCase(newFakeRepository(task), attachments, storage)
	input := baseUpdate("x", "y")
	input.Images = []models.ImageAttachment{pngImage("a.png"), pngImage("b.png")}

	_, err := useCase.Execute(context.Background(), task.ID, input)

	assert.ErrorIs(t, err, werrors.Repository("insert failed"))
	assert.Len(t, storage.deleted, 2, "the two files written by this call are removed")
	for _, path := range storage.deleted {
		assert.NotEqual(t, "old.png", path, "a stored attachment is never removed by a failed update")
	}
}

// Rust: update_task::limit_counts_attachments_after_removal
func TestUpdateLimitCountsAttachmentsAfterRemoval(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	attachments := &fakeAttachments{listed: []models.StoredAttachment{
		{ID: "1", Path: "1.png"}, {ID: "2", Path: "2.png"}, {ID: "3", Path: "3.png"}, {ID: "4", Path: "4.png"},
	}}
	useCase := updateUseCase(newFakeRepository(task), attachments, &fakeStorage{})
	input := baseUpdate("x", "y")
	input.Images = []models.ImageAttachment{pngImage("a.png"), pngImage("b.png")}
	input.RemoveIDs = []string{"1", "2"}

	_, err := useCase.Execute(context.Background(), task.ID, input)

	require.NoError(t, err, "4 stored - 2 removed + 2 new = 4 is within the limit of 5")
}

// Rust: update_task::rejects_when_remaining_plus_new_exceed_the_limit
func TestUpdateRejectsWhenRemainingPlusNewExceedTheLimit(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	attachments := &fakeAttachments{listed: []models.StoredAttachment{
		{ID: "1", Path: "1.png"}, {ID: "2", Path: "2.png"}, {ID: "3", Path: "3.png"},
		{ID: "4", Path: "4.png"}, {ID: "5", Path: "5.png"},
	}}
	useCase := updateUseCase(newFakeRepository(task), attachments, &fakeStorage{})
	input := baseUpdate("x", "y")
	input.Images = []models.ImageAttachment{pngImage("a.png"), pngImage("b.png")}
	input.RemoveIDs = []string{"1"}

	_, err := useCase.Execute(context.Background(), task.ID, input)

	assert.ErrorIs(t, err, werrors.InvalidAttachment(""))
}

// Rust: update_task::rejects_removal_when_task_is_not_editable
func TestUpdateRejectsRemovalWhenTheTaskIsNotEditable(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle)
	attachments := &fakeAttachments{listed: []models.StoredAttachment{{ID: "1", Path: "1.png"}}}
	useCase := updateUseCase(newFakeRepository(task), attachments, &fakeStorage{})
	input := baseUpdate("x", "y")
	input.RemoveIDs = []string{"1"}

	_, err := useCase.Execute(context.Background(), task.ID, input)

	assert.ErrorIs(t, err, werrors.InvalidState("editar", ""))
	assert.Empty(t, attachments.removed)
}

// Rust: get_task::missing_task_is_not_found
func TestGetTaskMissingIsNotFound(t *testing.T) {
	t.Parallel()
	lifecycle, _ := newLifecycle(newFakeRepository())
	id := newTaskIDValue(t)

	_, err := application.NewGetTask(lifecycle).Execute(context.Background(), id)

	assert.ErrorIs(t, err, werrors.NotFound(""))
	assert.ErrorContains(t, err, id.String())
}

// Rust: get_task_subagents::existing_task_without_subagents_yields_empty
func TestGetTaskSubagentsOfTaskWithoutSubagentsIsEmpty(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	lifecycle, _ := newLifecycle(newFakeRepository(task))
	runtime := &fakeRuntime{}

	subagents, err := application.NewGetTaskSubagents(lifecycle, runtime).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Empty(t, subagents)
}

// Rust: list_done_tasks::passes_valid_values_through
func TestListDoneTasksPassesValidValuesThrough(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	workspace := "ws"

	_, err := application.NewListDoneTasks(repo).Execute(context.Background(), &workspace, 50, 10)

	require.NoError(t, err)
	assert.Equal(t, [2]int64{50, 10}, repo.donePage)
	require.NotNil(t, repo.doneWorkspace)
	assert.Equal(t, "ws", *repo.doneWorkspace)
}

// Rust: list_done_tasks::blank_workspace_means_all (the port reads a blank filter as no filter)
func TestListDoneTasksBlankWorkspaceMeansAll(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	blank := "  "

	_, err := application.NewListDoneTasks(repo).Execute(context.Background(), &blank, 20, 0)

	require.NoError(t, err)
	assert.Nil(t, repo.doneWorkspace)
	assert.Equal(t, [2]int64{20, 0}, repo.donePage)
}

// Rust: delete_workspace_tasks::attachment_cleanup_failure_does_not_block_deletion
func TestDeleteWorkspaceTasksAttachmentCleanupFailureDoesNotBlockDeletion(t *testing.T) {
	t.Parallel()
	backlog := newTask(models.StatusBacklog)
	repo := newFakeRepository(backlog)
	lifecycle, publisher := newLifecycle(repo)

	err := application.NewDeleteWorkspaceTasks(lifecycle, &fakeRemover{}, &fakeStorage{failDeleteDir: true}).DeleteAll(context.Background(), "ws-1")

	require.NoError(t, err)
	assert.Empty(t, repo.tasks)
	assert.Len(t, publisher.published, 1)
}
