package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/workspaces/application"
	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
)

func TestListWorkspacesSortsByNameCaseInsensitive(t *testing.T) {
	repository := newFakeRepository(
		workspaceAt(t, "beta", "/repos/beta", "main"),
		workspaceAt(t, "Alpha", "/repos/alpha", "main"),
		workspaceAt(t, "gamma", "/repos/gamma", "main"),
	)
	useCase := application.NewListWorkspaces(repository)

	workspaces, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	names := make([]string, 0, len(workspaces))
	for _, workspace := range workspaces {
		names = append(names, workspace.Name)
	}
	assert.Equal(t, []string{"Alpha", "beta", "gamma"}, names)
}

func TestListWorkspacesPropagatesRepositoryErrors(t *testing.T) {
	repository := newFakeRepository()
	repository.listErr = werrors.Repository("boom")

	_, err := application.NewListWorkspaces(repository).Execute(context.Background())

	require.ErrorIs(t, err, werrors.Repository(""))
	assert.EqualError(t, err, "error de persistencia: boom")
}

func TestGetWorkspaceReturnsNotFoundWhenMissing(t *testing.T) {
	id, err := models.NewWorkspaceID()
	require.NoError(t, err)
	useCase := application.NewGetWorkspace(newFakeRepository())

	_, err = useCase.Execute(context.Background(), id)

	require.ErrorIs(t, err, werrors.NotFound(id.String()))
	assert.EqualError(t, err, "workspace no encontrado: "+id.String())
}

func TestGetWorkspaceReturnsStoredWorkspace(t *testing.T) {
	stored := workspaceAt(t, "demo", "/repos/demo", "main")
	useCase := application.NewGetWorkspace(newFakeRepository(stored))

	found, err := useCase.Execute(context.Background(), stored.ID)

	require.NoError(t, err)
	assert.Equal(t, stored, found)
}

func TestDeleteWorkspaceDeletesTasksBeforeTheWorkspace(t *testing.T) {
	stored := workspaceAt(t, "demo", "/repos/demo", "main")
	repository := newFakeRepository(stored)
	tasks := &fakeWorkspaceTasks{}
	useCase := application.NewDeleteWorkspace(repository, tasks)

	require.NoError(t, useCase.Execute(context.Background(), stored.ID))

	assert.Equal(t, []models.WorkspaceID{stored.ID}, tasks.deleted)
	assert.Equal(t, []models.WorkspaceID{stored.ID}, repository.deleted)
}

func TestDeleteWorkspaceKeepsWorkspaceWhenTasksDeletionFails(t *testing.T) {
	stored := workspaceAt(t, "demo", "/repos/demo", "main")
	repository := newFakeRepository(stored)
	tasks := &fakeWorkspaceTasks{err: werrors.InUse()}
	useCase := application.NewDeleteWorkspace(repository, tasks)

	err := useCase.Execute(context.Background(), stored.ID)

	require.ErrorIs(t, err, werrors.InUse())
	assert.Empty(t, repository.deleted)
}

func TestDeleteWorkspacePropagatesRepositoryInUse(t *testing.T) {
	stored := workspaceAt(t, "demo", "/repos/demo", "main")
	repository := newFakeRepository(stored)
	repository.deleteErr = werrors.InUse()
	useCase := application.NewDeleteWorkspace(repository, &fakeWorkspaceTasks{})

	err := useCase.Execute(context.Background(), stored.ID)

	require.ErrorIs(t, err, werrors.InUse())
}

func TestListAgentsFileOptionsReturnsPortOptions(t *testing.T) {
	agentsFiles := &fakeAgentsFiles{options: []models.AgentsFileOption{{ID: "a", Description: "Rust"}}}

	options, err := application.NewListAgentsFileOptions(agentsFiles).Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []models.AgentsFileOption{{ID: "a", Description: "Rust"}}, options)
}

func TestListAgentsFileOptionsPropagatesPortErrors(t *testing.T) {
	agentsFiles := &fakeAgentsFiles{listErr: werrors.AgentsFile("boom")}

	_, err := application.NewListAgentsFileOptions(agentsFiles).Execute(context.Background())

	assert.EqualError(t, err, "error al instalar AGENTS.md: boom")
}
