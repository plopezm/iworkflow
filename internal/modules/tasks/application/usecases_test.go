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
	return application.NewCreateTask(lifecycle, fakeLayout{}, starter, attachments, storage, newUUID, clock), lifecycle
}

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
}

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

func TestCreateTaskUsesTheRequestedBaseBranchOrTheWorkspaceDefault(t *testing.T) {
	t.Parallel()
	useCase, _ := createUseCase(newFakeRepository(), &fakeStarter{}, &fakeAttachments{}, &fakeStorage{})
	requested := "origin/develop"
	input := newInput("rama")
	input.BaseBranch = &requested

	created, err := useCase.Execute(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, "origin/develop", created.Task.BaseBranch)

	defaulted, err := useCase.Execute(context.Background(), newInput("sin rama"))
	require.NoError(t, err)
	assert.Equal(t, "main", defaulted.Task.BaseBranch)
}

func TestCreateTaskStartNowDelegatesToStart(t *testing.T) {
	t.Parallel()
	starter := &fakeStarter{}
	useCase, _ := createUseCase(newFakeRepository(), starter, &fakeAttachments{}, &fakeStorage{})
	input := newInput("arranca")
	input.StartNow = true

	created, err := useCase.Execute(context.Background(), input)
	require.NoError(t, err)

	require.Len(t, starter.started, 1)
	assert.Nil(t, created.StartError)
}

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

func TestCreateTaskFailsWithoutSideEffectsWhenWorkspaceIsMissing(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	useCase, _ := createUseCase(repo, &fakeStarter{}, &fakeAttachments{}, &fakeStorage{})
	input := newInput("sin workspace")
	input.WorkspaceID = "missing"

	_, err := useCase.Execute(context.Background(), input)

	assert.ErrorIs(t, err, werrors.WorkspaceNotFound(""))
	assert.Empty(t, repo.tasks)
}

func TestCreateTaskRejectsEmptyPrompt(t *testing.T) {
	t.Parallel()
	useCase, _ := createUseCase(newFakeRepository(), &fakeStarter{}, &fakeAttachments{}, &fakeStorage{})

	_, err := useCase.Execute(context.Background(), newInput(" \n "))

	assert.ErrorIs(t, err, werrors.EmptyPrompt())
}

// updateUseCase wires UpdateTask over the fakes.
func updateUseCase(repo *fakeRepository, attachments *fakeAttachments, storage *fakeStorage) *application.UpdateTask {
	lifecycle, _ := newLifecycle(repo)
	return application.NewUpdateTask(lifecycle, attachments, storage, newUUID)
}

func baseUpdate(title string, prompt string) application.UpdateInput {
	return application.UpdateInput{Title: title, Prompt: prompt, Priority: models.PriorityHigh}
}

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
}

func TestUpdateRejectsTaskNotInBacklog(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle)
	useCase := updateUseCase(newFakeRepository(task), &fakeAttachments{}, &fakeStorage{})

	_, err := useCase.Execute(context.Background(), task.ID, baseUpdate("x", "y"))

	assert.ErrorIs(t, err, werrors.InvalidState("editar", ""))
}

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
}

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
}

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

func TestGetTaskSubagentsOfMissingTaskIsNotFound(t *testing.T) {
	t.Parallel()
	lifecycle, _ := newLifecycle(newFakeRepository())
	runtime := &fakeRuntime{subagents: []models.SubagentTask{{ID: "s1"}}}
	id := newTaskIDValue(t)

	_, err := application.NewGetTaskSubagents(lifecycle, runtime).Execute(context.Background(), id)

	assert.ErrorIs(t, err, werrors.NotFound(""))
}

func TestGetTaskSubagentsReturnsTheRuntimeSubagents(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	lifecycle, _ := newLifecycle(newFakeRepository(task))
	runtime := &fakeRuntime{subagents: []models.SubagentTask{{ID: "s1", Status: models.SubagentRunning}}}

	subagents, err := application.NewGetTaskSubagents(lifecycle, runtime).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	require.Len(t, subagents, 1)
	assert.Equal(t, "s1", subagents[0].ID)
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

func TestDeleteWorkspaceTasksDeletesInactiveTasksAndNotifies(t *testing.T) {
	t.Parallel()
	backlog := newTask(models.StatusBacklog)
	merged := newTask(models.StatusMerged)
	repo := newFakeRepository(backlog, merged)
	storage := &fakeStorage{}
	lifecycle, publisher := newLifecycle(repo)

	require.NoError(t, application.NewDeleteWorkspaceTasks(lifecycle, &fakeRemover{}, storage).DeleteAll(context.Background(), "ws-1"))

	assert.Empty(t, repo.tasks)
	assert.Len(t, storage.deletedDirs, 2)
	assert.Len(t, publisher.published, 2)
}

func TestDeleteWorkspaceTasksRefusesWhenATaskIsActive(t *testing.T) {
	t.Parallel()
	backlog := newTask(models.StatusBacklog)
	running := newTask(models.StatusRunning)
	repo := newFakeRepository(backlog, running)
	lifecycle, _ := newLifecycle(repo)

	err := application.NewDeleteWorkspaceTasks(lifecycle, &fakeRemover{}, &fakeStorage{}).DeleteAll(context.Background(), "ws-1")

	assert.True(t, errors.Is(err, werrors.WorkspaceHasActiveTasks()))
	assert.Len(t, repo.tasks, 2, "nothing is deleted")
}

func withStatus(task models.Task, status models.TaskStatus) models.Task {
	task.Status = status
	return task
}
