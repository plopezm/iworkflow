package application_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// ownWorkspace is the workspace the test tasks belong to ("ws-1" in the fakes).
const ownWorkspace = "ws-1"

// knownWorkspacesWith knows the task's own workspace and the given related ones.
func knownWorkspacesWith(ids ...string) knownWorkspaces {
	known := knownWorkspaces{ownWorkspace: {ID: ownWorkspace, Name: "demo", Path: "/repos/demo", BaseBranch: "main"}}
	for _, id := range ids {
		known[id] = models.WorkspaceRef{ID: id, Name: "otro", Path: "/repos/" + id, BaseBranch: "main"}
	}
	return known
}

// assertStored compares the stored ids; nil and empty both mean that the task has no related projects.
func assertStored(t *testing.T, want []string, got []string) {
	t.Helper()
	if len(want) == 0 {
		assert.Empty(t, got)
		return
	}
	assert.Equal(t, want, got)
}

// createWithRelated wires CreateTask over the fakes, with the given related projects store and workspaces.
func createWithRelated(repo *fakeRepository, related *fakeRelatedProjects, known knownWorkspaces) *application.CreateTask {
	lifecycle := application.NewTaskLifecycle(repo, &fakePublisher{}, known, clock)
	return application.NewCreateTask(lifecycle, fakeLayout{}, &fakeStarter{}, &fakeAttachments{}, related, &fakeStorage{}, newUUID, clock)
}

// updateWithRelated wires UpdateTask over the fakes, with the given related projects store and workspaces.
func updateWithRelated(repo *fakeRepository, related *fakeRelatedProjects, known knownWorkspaces) *application.UpdateTask {
	lifecycle := application.NewTaskLifecycle(repo, &fakePublisher{}, known, clock)
	return application.NewUpdateTask(lifecycle, &fakeAttachments{}, related, &fakeStorage{}, newUUID)
}

func TestCreateTaskStoresRelatedProjectsInOrder(t *testing.T) {
	t.Parallel()
	first, second := uuid.NewString(), uuid.NewString()
	tests := []struct {
		name       string
		input      []string
		wantStored []string
	}{
		{name: "keeps the chosen order", input: []string{second, first}, wantStored: []string{second, first}},
		{name: "drops duplicates and the own workspace", input: []string{first, ownWorkspace, first, second}, wantStored: []string{first, second}},
		{name: "stores nothing without related projects", input: nil, wantStored: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := newFakeRepository()
			related := newFakeRelatedProjects()
			input := newInput("relacionada")
			input.RelatedProjectIDs = tt.input

			created, err := createWithRelated(repo, related, knownWorkspacesWith(first, second)).Execute(context.Background(), input)

			require.NoError(t, err)
			assert.Equal(t, models.StatusBacklog, created.Task.Status)
			assertStored(t, tt.wantStored, related.stored[created.Task.ID.String()])
		})
	}
}

// A related workspace that does not exist, or a value that is not a UUID, rejects the input: the task is not kept.
func TestCreateTaskRejectedRelatedProjectsStoreNothing(t *testing.T) {
	t.Parallel()
	known := uuid.NewString()
	missing := uuid.NewString()
	tests := []struct {
		name    string
		input   []string
		wantErr error
	}{
		{name: "unknown workspace", input: []string{known, missing}, wantErr: werrors.WorkspaceNotFound(missing)},
		{name: "value that is not a UUID", input: []string{known, "no-es-uuid"}, wantErr: werrors.InvalidRelatedProject("no-es-uuid")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := newFakeRepository()
			related := newFakeRelatedProjects()
			input := newInput("rechazada")
			input.RelatedProjectIDs = tt.input

			_, err := createWithRelated(repo, related, knownWorkspacesWith(known)).Execute(context.Background(), input)

			require.ErrorIs(t, err, tt.wantErr)
			assert.Empty(t, repo.tasks, "the task inserted before the check is deleted again")
			assert.Empty(t, related.stored)
		})
	}
}

// A failed write of the related projects deletes the backlog task and returns the write error.
func TestCreateTaskDeletesTheTaskWhenRelatedProjectsFailToSave(t *testing.T) {
	t.Parallel()
	first := uuid.NewString()
	repo := newFakeRepository()
	related := newFakeRelatedProjects()
	related.failReplace = true
	input := newInput("falla al guardar")
	input.RelatedProjectIDs = []string{first}

	_, err := createWithRelated(repo, related, knownWorkspacesWith(first)).Execute(context.Background(), input)

	require.Error(t, err)
	assert.Empty(t, repo.tasks)
	assert.Equal(t, 1, related.replaceCalls)
}

func TestUpdateTaskReplacesRelatedProjects(t *testing.T) {
	t.Parallel()
	first, second, third := uuid.NewString(), uuid.NewString(), uuid.NewString()
	tests := []struct {
		name       string
		input      []string
		wantStored []string
	}{
		{name: "replaces the previous list in order", input: []string{third, first}, wantStored: []string{third, first}},
		{name: "an empty list clears them", input: nil, wantStored: []string{}},
		{name: "drops the own workspace", input: []string{ownWorkspace, second}, wantStored: []string{second}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			task := newTask(models.StatusBacklog)
			repo := newFakeRepository(task)
			related := newFakeRelatedProjects()
			related.stored[task.ID.String()] = []string{first, second}
			input := baseUpdate("editada", "prompt")
			input.RelatedProjectIDs = tt.input

			_, err := updateWithRelated(repo, related, knownWorkspacesWith(first, second, third)).Execute(context.Background(), task.ID, input)

			require.NoError(t, err)
			assertStored(t, tt.wantStored, related.stored[task.ID.String()])
			assert.Equal(t, "editada", repo.tasks[task.ID].Title)
		})
	}
}

// A rejected edit changes neither the task nor its related projects.
func TestUpdateTaskRejectedRelatedProjectsChangeNothing(t *testing.T) {
	t.Parallel()
	first := uuid.NewString()
	missing := uuid.NewString()
	tests := []struct {
		name    string
		input   []string
		wantErr error
	}{
		{name: "unknown workspace", input: []string{missing}, wantErr: werrors.WorkspaceNotFound(missing)},
		{name: "value that is not a UUID", input: []string{"no-es-uuid"}, wantErr: werrors.InvalidRelatedProject("no-es-uuid")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			task := newTask(models.StatusBacklog)
			repo := newFakeRepository(task)
			related := newFakeRelatedProjects()
			related.stored[task.ID.String()] = []string{first}
			input := baseUpdate("no debe guardarse", "prompt")
			input.RelatedProjectIDs = tt.input

			_, err := updateWithRelated(repo, related, knownWorkspacesWith(first)).Execute(context.Background(), task.ID, input)

			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, task.Title, repo.tasks[task.ID].Title, "the details are not saved")
			assert.Equal(t, []string{first}, related.stored[task.ID.String()])
			assert.Equal(t, 0, related.replaceCalls)
		})
	}
}

// Only a backlog task can be edited, so its related projects stay as they were when it is running.
func TestUpdateTaskOutsideBacklogKeepsRelatedProjects(t *testing.T) {
	t.Parallel()
	first, second := uuid.NewString(), uuid.NewString()
	task := newTask(models.StatusRunning)
	repo := newFakeRepository(task)
	related := newFakeRelatedProjects()
	related.stored[task.ID.String()] = []string{first}
	input := baseUpdate("no", "prompt")
	input.RelatedProjectIDs = []string{second}

	_, err := updateWithRelated(repo, related, knownWorkspacesWith(first, second)).Execute(context.Background(), task.ID, input)

	require.Error(t, err)
	assert.Equal(t, []string{first}, related.stored[task.ID.String()])
	assert.Equal(t, 0, related.replaceCalls)
}

func TestUpdateTaskReturnsTheReplaceError(t *testing.T) {
	t.Parallel()
	first := uuid.NewString()
	task := newTask(models.StatusBacklog)
	repo := newFakeRepository(task)
	related := newFakeRelatedProjects()
	related.failReplace = true
	input := baseUpdate("editada", "prompt")
	input.RelatedProjectIDs = []string{first}

	_, err := updateWithRelated(repo, related, knownWorkspacesWith(first)).Execute(context.Background(), task.ID, input)

	require.Error(t, err)
	assert.Equal(t, 1, related.replaceCalls)
}

func TestListTaskRelatedProjectsReturnsStoredOrder(t *testing.T) {
	t.Parallel()
	first, second, third := uuid.NewString(), uuid.NewString(), uuid.NewString()
	task := newTask(models.StatusBacklog)
	related := newFakeRelatedProjects()
	related.stored[task.ID.String()] = []string{third, first, second}
	useCase := application.NewListTaskRelatedProjects(related, knownWorkspacesWith(first, second, third))

	refs, err := useCase.Execute(context.Background(), task.ID)

	require.NoError(t, err)
	require.Len(t, refs, 3)
	assert.Equal(t, third, refs[0].ID)
	assert.Equal(t, first, refs[1].ID)
	assert.Equal(t, second, refs[2].ID)
}

// A related workspace that no longer exists is skipped and the rest of the list is still returned.
func TestListTaskRelatedProjectsSkipsMissingWorkspaces(t *testing.T) {
	t.Parallel()
	first, gone, second := uuid.NewString(), uuid.NewString(), uuid.NewString()
	task := newTask(models.StatusBacklog)
	related := newFakeRelatedProjects()
	related.stored[task.ID.String()] = []string{first, gone, second}
	useCase := application.NewListTaskRelatedProjects(related, knownWorkspacesWith(first, second))

	refs, err := useCase.Execute(context.Background(), task.ID)

	require.NoError(t, err)
	require.Len(t, refs, 2)
	assert.Equal(t, first, refs[0].ID)
	assert.Equal(t, second, refs[1].ID)
}

func TestListTaskRelatedProjectsEdgeCases(t *testing.T) {
	t.Parallel()
	first := uuid.NewString()
	tests := []struct {
		name     string
		failList bool
		wantLen  int
		wantErr  bool
	}{
		{name: "task without related projects", wantLen: 0},
		{name: "repository failure is returned", failList: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			task := newTask(models.StatusBacklog)
			related := newFakeRelatedProjects()
			related.failList = tt.failList
			useCase := application.NewListTaskRelatedProjects(related, knownWorkspacesWith(first))

			refs, err := useCase.Execute(context.Background(), task.ID)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, refs, tt.wantLen)
		})
	}
}
