package application_test

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// pngImage is a tiny valid image; its data is base64 of three bytes.
func pngImage(name string) models.ImageAttachment {
	return models.ImageAttachment{Name: name, MimeType: "image/png", Data: base64.StdEncoding.EncodeToString([]byte("abc"))}
}

func newInput(prompt string) models.NewTask {
	return models.NewTask{WorkspaceID: "ws-1", Title: "t", Prompt: prompt}
}

// createUseCase wires CreateTask over the fakes.
func createUseCase(repo *fakeRepository, starter *fakeStarter, attachments *fakeAttachments, storage *fakeStorage) (*application.CreateTask, *application.TaskLifecycle) {
	lifecycle, _ := newLifecycle(repo)
	return application.NewCreateTask(lifecycle, fakeLayout{}, starter, attachments, newFakeRelatedProjects(), storage, newUUID, clock), lifecycle
}

// Rust: create_task::creates_task_in_backlog_without_starting_it
func TestCreateTaskCreatesBacklogWithoutStarting(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	starter := &fakeStarter{}
	useCase, _ := createUseCase(repo, starter, &fakeAttachments{}, &fakeStorage{})

	created, err := useCase.Execute(context.Background(), newInput("haz algo"))
	require.NoError(t, err)

	assert.Equal(t, models.StatusBacklog, created.Task.Status)
	assert.Equal(t, "main", created.Task.BaseBranch)
	assert.Nil(t, created.StartError)
	assert.Empty(t, starter.started)
	assert.Contains(t, created.Task.Branch, "pi/t-")
	assert.Equal(t, []models.TaskStatus{models.StatusBacklog}, repo.statuses)
}

// Rust: create_task::stores_files_first_and_then_their_metadata
func TestCreateTaskStoresFilesFirstAndThenTheirMetadata(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	storage := &fakeStorage{}
	attachments := &fakeAttachments{}
	useCase, _ := createUseCase(repo, &fakeStarter{}, attachments, storage)
	input := newInput("con imagen")
	input.Images = []models.ImageAttachment{pngImage("a.png"), pngImage("b.png")}

	_, err := useCase.Execute(context.Background(), input)
	require.NoError(t, err)

	require.Len(t, storage.saved, 2)
	require.Len(t, attachments.inserted, 1)
	stored := attachments.inserted[0]
	assert.Equal(t, "a.png", stored[0].Name)
	assert.Equal(t, storage.saved[0], stored[0].Path)
	assert.Equal(t, uint64(3), stored[0].SizeBytes)
	assert.Empty(t, storage.deletedDirs, "nothing is cleaned up on success")
}

// Rust: create_task::deletes_files_and_task_and_propagates_when_inserting_rows_fails
func TestCreateTaskDeletesTaskAndFilesWhenInsertingRowsFails(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	storage := &fakeStorage{}
	attachments := &fakeAttachments{failInsert: true}
	useCase, _ := createUseCase(repo, &fakeStarter{}, attachments, storage)
	input := newInput("falla")
	input.Images = []models.ImageAttachment{pngImage("a.png")}

	_, err := useCase.Execute(context.Background(), input)

	assert.ErrorIs(t, err, werrors.Repository("insert failed"))
	assert.Len(t, storage.deletedDirs, 1, "the files written by this call are removed")
	assert.Empty(t, repo.tasks, "the task created for the images is removed")
}

// Rust: create_task::rejects_invalid_images_before_touching_the_repository
func TestCreateTaskRejectsInvalidImagesBeforeTouchingTheRepository(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	useCase, _ := createUseCase(repo, &fakeStarter{}, &fakeAttachments{}, &fakeStorage{})
	input := newInput("bmp")
	input.Images = []models.ImageAttachment{{Name: "a.bmp", MimeType: "image/bmp", Data: "aGk="}}

	_, err := useCase.Execute(context.Background(), input)

	assert.ErrorIs(t, err, werrors.InvalidAttachment(""))
	assert.Empty(t, repo.tasks)
}

// Rust: create_task::uses_the_requested_base_branch_instead_of_the_workspace_one
func TestCreateTaskUsesTheRequestedBaseBranch(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	useCase, _ := createUseCase(repo, &fakeStarter{}, &fakeAttachments{}, &fakeStorage{})
	requested := "origin/develop"
	input := newInput("rama")
	input.BaseBranch = &requested

	created, err := useCase.Execute(context.Background(), input)

	require.NoError(t, err)
	assert.Equal(t, "origin/develop", created.Task.BaseBranch)
	assert.Equal(t, "origin/develop", repo.tasks[created.Task.ID].BaseBranch, "the stored task has the requested branch")
}

// Rust: create_task::falls_back_to_the_workspace_base_branch_without_a_request
func TestCreateTaskFallsBackToTheWorkspaceBaseBranchWithoutARequest(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	useCase, _ := createUseCase(repo, &fakeStarter{}, &fakeAttachments{}, &fakeStorage{})

	created, err := useCase.Execute(context.Background(), newInput("sin rama"))

	require.NoError(t, err)
	assert.Equal(t, "main", created.Task.BaseBranch)
	assert.Equal(t, "main", repo.tasks[created.Task.ID].BaseBranch, "the stored task has the workspace branch")
}

// Rust: create_task::start_now_delegates_to_start_task
func TestCreateTaskStartNowDelegatesToStart(t *testing.T) {
	t.Parallel()
	starter := &fakeStarter{}
	repo := newFakeRepository()
	useCase, _ := createUseCase(repo, starter, &fakeAttachments{}, &fakeStorage{})
	input := newInput("arranca")
	input.StartNow = true

	created, err := useCase.Execute(context.Background(), input)
	require.NoError(t, err)

	require.Len(t, starter.started, 1)
	assert.Nil(t, created.StartError)
	assert.Equal(t, models.StatusStarting, created.Task.Status, "the task returned is the one the start produced")
	assert.Equal(t, []models.TaskStatus{models.StatusBacklog}, repo.statuses, "create itself only writes backlog")
}

// Rust: create_task::keeps_created_task_when_start_fails
func TestCreateTaskKeepsCreatedTaskWhenStartFails(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	starter := &fakeStarter{err: werrors.Git("sin rama")}
	useCase, _ := createUseCase(repo, starter, &fakeAttachments{}, &fakeStorage{})
	input := newInput("falla al arrancar")
	input.StartNow = true

	created, err := useCase.Execute(context.Background(), input)

	require.NoError(t, err)
	assert.ErrorIs(t, created.StartError, werrors.Git(""))
	assert.Equal(t, models.StatusBacklog, created.Task.Status)
	assert.Len(t, repo.tasks, 1, "the task still exists so the start can be retried")
}

// Rust: create_task::fails_without_side_effects_when_workspace_is_missing
func TestCreateTaskFailsWithoutSideEffectsWhenWorkspaceIsMissing(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	starter := &fakeStarter{}
	attachments := &fakeAttachments{}
	storage := &fakeStorage{}
	useCase, _ := createUseCase(repo, starter, attachments, storage)
	input := newInput("sin workspace")
	input.WorkspaceID = "missing"
	input.Images = []models.ImageAttachment{pngImage("a.png")}
	input.StartNow = true

	_, err := useCase.Execute(context.Background(), input)

	assert.ErrorIs(t, err, werrors.WorkspaceNotFound(""))
	assert.Empty(t, repo.tasks)
	assert.Empty(t, repo.statuses)
	assert.Empty(t, storage.saved)
	assert.Empty(t, attachments.inserted)
	assert.Empty(t, starter.started)
}

// Rust: create_task::rejects_empty_prompt
func TestCreateTaskRejectsEmptyPrompt(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	useCase, _ := createUseCase(repo, &fakeStarter{}, &fakeAttachments{}, &fakeStorage{})

	_, err := useCase.Execute(context.Background(), newInput(" \n "))

	assert.ErrorIs(t, err, werrors.EmptyPrompt())
	assert.Empty(t, repo.tasks)
}

// updateUseCase wires UpdateTask over the fakes.
func updateUseCase(repo *fakeRepository, attachments *fakeAttachments, storage *fakeStorage) *application.UpdateTask {
	lifecycle, _ := newLifecycle(repo)
	return application.NewUpdateTask(lifecycle, attachments, storage, newUUID)
}

func baseUpdate(title string, prompt string) application.UpdateInput {
	return application.UpdateInput{Title: title, Prompt: prompt, Priority: models.PriorityHigh}
}

// Rust: update_task::updates_title_and_prompt_when_backlog
func TestUpdateChangesTitleAndPromptWhenBacklog(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	repo := newFakeRepository(task)
	useCase := updateUseCase(repo, &fakeAttachments{}, &fakeStorage{})

	updated, err := useCase.Execute(context.Background(), task.ID, baseUpdate("nuevo", "nuevo prompt"))

	require.NoError(t, err)
	assert.Equal(t, "nuevo", updated.Title)
	assert.Equal(t, models.PriorityHigh, updated.Priority)
	assert.Equal(t, "nuevo", repo.tasks[task.ID].Title)
	assert.Equal(t, "nuevo prompt", repo.tasks[task.ID].Prompt)
}

// Rust: update_task::rejects_task_not_in_backlog
func TestUpdateRejectsTaskNotInBacklog(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle)
	repo := newFakeRepository(task)
	useCase := updateUseCase(repo, &fakeAttachments{}, &fakeStorage{})

	_, err := useCase.Execute(context.Background(), task.ID, baseUpdate("x", "y"))

	assert.ErrorIs(t, err, werrors.InvalidState("editar", ""))
	assert.Equal(t, task.Title, repo.tasks[task.ID].Title, "the stored task is unchanged")
	assert.Equal(t, task.Prompt, repo.tasks[task.ID].Prompt, "the stored task is unchanged")
}

// Rust: update_task::rejects_when_task_left_backlog_concurrently
func TestUpdateRejectsWhenTaskLeftBacklogConcurrently(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	repo := newFakeRepository(task)
	notUpdated := false
	repo.detailsResult = &notUpdated
	repo.tasks[task.ID] = withStatus(task, models.StatusStarting)
	useCase := updateUseCase(repo, &fakeAttachments{}, &fakeStorage{})

	_, err := useCase.Execute(context.Background(), task.ID, baseUpdate("x", "y"))

	assert.ErrorIs(t, err, werrors.InvalidState("editar", "starting"))
}

// Rust: update_task::rejects_images_beyond_the_per_task_limit
func TestUpdateRejectsImagesBeyondThePerTaskLimit(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	attachments := &fakeAttachments{}
	for range models.MaxImagesPerMessage {
		attachments.listed = append(attachments.listed, models.StoredAttachment{ID: newIDString(t)})
	}
	useCase := updateUseCase(newFakeRepository(task), attachments, &fakeStorage{})
	input := baseUpdate("x", "y")
	input.Images = []models.ImageAttachment{pngImage("extra.png")}

	_, err := useCase.Execute(context.Background(), task.ID, input)

	assert.ErrorIs(t, err, werrors.InvalidAttachment("máximo 5 imágenes por tarea"))
	assert.Empty(t, attachments.inserted, "no metadata is written when the limit is exceeded")
}

// Rust: update_task::removes_attachments_and_deletes_their_files
func TestUpdateRemovesAttachmentsAndDeletesTheirFiles(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	attachments := &fakeAttachments{listed: []models.StoredAttachment{
		{ID: "keep", Path: "keep.png"},
		{ID: "drop", Path: "drop.png"},
	}}
	storage := &fakeStorage{}
	useCase := updateUseCase(newFakeRepository(task), attachments, storage)
	input := baseUpdate("x", "y")
	input.RemoveIDs = []string{"drop"}

	_, err := useCase.Execute(context.Background(), task.ID, input)

	require.NoError(t, err)
	assert.Equal(t, []string{"drop.png"}, storage.deleted)
	require.Len(t, attachments.removed, 1)
	assert.Equal(t, "drop", attachments.removed[0].ID, "only the requested attachment is removed")
}

// Rust: update_task::file_deletion_failure_does_not_fail_the_update
func TestUpdateFileDeletionFailureDoesNotFailTheUpdate(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	attachments := &fakeAttachments{listed: []models.StoredAttachment{{ID: "drop", Path: "drop.png"}}}
	storage := &fakeStorage{failDelete: true}
	useCase := updateUseCase(newFakeRepository(task), attachments, storage)
	input := baseUpdate("x", "y")
	input.RemoveIDs = []string{"drop"}

	_, err := useCase.Execute(context.Background(), task.ID, input)

	assert.NoError(t, err)
}

func TestDeleteTaskDeletesBacklogTask(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	repo := newFakeRepository(task)
	storage := &fakeStorage{}
	lifecycle, publisher := newLifecycle(repo)

	require.NoError(t, application.NewDeleteTask(lifecycle, storage).Execute(context.Background(), task.ID))

	assert.Empty(t, repo.tasks)
	assert.Len(t, storage.deletedDirs, 1)
	require.Len(t, publisher.published, 1)
}

func TestDeleteTaskStorageFailureDoesNotBlockDeletion(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	repo := newFakeRepository(task)
	lifecycle, _ := newLifecycle(repo)

	err := application.NewDeleteTask(lifecycle, &fakeStorage{failDeleteDir: true}).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Empty(t, repo.tasks)
}

func TestDeleteTaskRejectsTaskNotInBacklog(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	lifecycle, _ := newLifecycle(newFakeRepository(task))

	err := application.NewDeleteTask(lifecycle, &fakeStorage{}).Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.InvalidState("eliminar", ""))
}

func TestDeleteTaskRejectsWhenTaskLeftBacklogConcurrently(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	repo := newFakeRepository(task)
	notDeleted := false
	repo.deleteResult = &notDeleted
	repo.tasks[task.ID] = withStatus(task, models.StatusStarting)
	lifecycle, _ := newLifecycle(repo)

	err := application.NewDeleteTask(lifecycle, &fakeStorage{}).Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.InvalidState("eliminar", "starting"))
}

func TestAnswerDialogForwardsAnswerAndRecordsResolution(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle, "d1")
	repo := newFakeRepository(task)
	runtime := &fakeRuntime{running: true, capabilities: models.FullAgentCapabilities()}
	lifecycle, _ := newLifecycle(repo)

	err := application.NewAnswerDialog(lifecycle, runtime).Execute(context.Background(), task.ID, "d1", models.ConfirmedAnswer{Confirmed: true})

	require.NoError(t, err)
	assert.Equal(t, []models.AgentCommand{models.AnswerDialogCommand{DialogID: "d1", Answer: models.ConfirmedAnswer{Confirmed: true}}}, runtime.sent)
	assert.Equal(t, []string{"d1"}, repo.dialogRemoves)
}

// An answer the agent took is not reported as undelivered when its resolution cannot be recorded: the error says the
// agent has it, and the agent is sent it once.
func TestAnswerDialogReportsAnAnswerTheAgentTookButNotRecorded(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle, "d1")
	repo := newFakeRepository(task)
	repo.appendErr = werrors.Repository("db bloqueada")
	runtime := &fakeRuntime{running: true, capabilities: models.FullAgentCapabilities()}
	lifecycle, _ := newLifecycle(repo)

	err := application.NewAnswerDialog(lifecycle, runtime).Execute(context.Background(), task.ID, "d1", models.CancelledAnswer{})

	assert.ErrorIs(t, err, werrors.AnswerNotRecorded())
	assert.Len(t, runtime.sent, 1)
}

func TestAnswerDialogRejectsWhenRuntimeLacksDialogs(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle)
	runtime := &fakeRuntime{running: true, capabilities: models.MinimalAgentCapabilities()}
	lifecycle, _ := newLifecycle(newFakeRepository(task))

	err := application.NewAnswerDialog(lifecycle, runtime).Execute(context.Background(), task.ID, "d1", models.CancelledAnswer{})

	assert.ErrorIs(t, err, werrors.Unsupported(""))
	assert.Empty(t, runtime.sent)
}

func TestAnswerDialogStoppedTaskYieldsAgentNotRunningBeforeCheckingCapabilities(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle)
	runtime := &fakeRuntime{running: false, capabilities: models.MinimalAgentCapabilities()}
	lifecycle, _ := newLifecycle(newFakeRepository(task))

	err := application.NewAnswerDialog(lifecycle, runtime).Execute(context.Background(), task.ID, "d1", models.CancelledAnswer{})

	assert.ErrorIs(t, err, werrors.AgentNotRunning())
}

// Rust: get_task_subagents::missing_task_is_not_found_and_runtime_is_not_asked
func TestGetTaskSubagentsOfMissingTaskIsNotFound(t *testing.T) {
	t.Parallel()
	lifecycle, _ := newLifecycle(newFakeRepository())
	runtime := &fakeRuntime{subagents: []models.SubagentTask{{ID: "s1"}}}
	id := newTaskIDValue(t)

	_, err := application.NewGetTaskSubagents(lifecycle, runtime).Execute(context.Background(), id)

	assert.ErrorIs(t, err, werrors.NotFound(""))
	assert.Zero(t, runtime.subagentCalls, "the runtime is not asked for a missing task")
}

// Rust: get_task_subagents::returns_the_runtime_subagents_of_an_existing_task
func TestGetTaskSubagentsReturnsTheRuntimeSubagents(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	lifecycle, _ := newLifecycle(newFakeRepository(task))
	runtime := &fakeRuntime{subagents: []models.SubagentTask{{ID: "s1", Status: models.SubagentRunning}}}

	subagents, err := application.NewGetTaskSubagents(lifecycle, runtime).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, runtime.subagents, subagents)
}

func TestListTasksBlankFilterListsAllActive(t *testing.T) {
	t.Parallel()
	active := newTask(models.StatusRunning)
	done := newTask(models.StatusMerged)
	repo := newFakeRepository(active, done)
	blank := "   "

	tasks, err := application.NewListTasks(repo).Execute(context.Background(), &blank)

	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Nil(t, repo.activeFilter, "a blank filter means no filter")
}

// Rust: list_done_tasks::clamps_limit_and_offset
func TestListDoneTasksClampsLimitAndOffset(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	useCase := application.NewListDoneTasks(repo)

	_, err := useCase.Execute(context.Background(), nil, 500, -3)
	require.NoError(t, err)
	assert.Equal(t, [2]int64{100, 0}, repo.donePage)

	_, err = useCase.Execute(context.Background(), nil, 0, 7)
	require.NoError(t, err)
	assert.Equal(t, [2]int64{1, 7}, repo.donePage)
}

// Rust: delete_workspace_tasks::deletes_inactive_tasks_and_notifies_removal
func TestDeleteWorkspaceTasksDeletesInactiveTasksAndNotifies(t *testing.T) {
	t.Parallel()
	backlog := newTask(models.StatusBacklog)
	merged := newTask(models.StatusMerged)
	repo := newFakeRepository(backlog, merged)
	storage := &fakeStorage{}
	remover := &fakeRemover{}
	lifecycle, publisher := newLifecycle(repo)

	require.NoError(t, application.NewDeleteWorkspaceTasks(lifecycle, remover, storage).DeleteAll(context.Background(), "ws-1"))

	assert.Empty(t, repo.tasks)
	assert.Len(t, storage.deletedDirs, 2)
	assert.Len(t, publisher.published, 2)
	assert.Equal(t, []string{"/repos/demo|" + merged.WorktreePath + "|" + merged.Branch}, remover.removed, "only the closed task is cleaned up in git")
}

// Rust: delete_workspace_tasks::rejects_when_any_task_is_active_without_cleaning
func TestDeleteWorkspaceTasksRefusesWhenATaskIsActive(t *testing.T) {
	t.Parallel()
	backlog := newTask(models.StatusBacklog)
	running := newTask(models.StatusRunning)
	repo := newFakeRepository(backlog, running)
	storage := &fakeStorage{}
	lifecycle, publisher := newLifecycle(repo)

	err := application.NewDeleteWorkspaceTasks(lifecycle, &fakeRemover{}, storage).DeleteAll(context.Background(), "ws-1")

	assert.True(t, errors.Is(err, werrors.WorkspaceHasActiveTasks()))
	assert.Len(t, repo.tasks, 2, "nothing is deleted")
	assert.Empty(t, storage.deletedDirs, "no attachment folder is removed")
	assert.Empty(t, publisher.published, "no removal is announced")
}

func withStatus(task models.Task, status models.TaskStatus) models.Task {
	task.Status = status
	return task
}
