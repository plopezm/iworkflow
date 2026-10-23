package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agents_files/application"
	agentserrors "iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/domain/models"
)

func newApply(file models.AgentsFile, projects *fakeProjects, installer *fakeInstaller) *application.ApplyAgentsFileToProject {
	storage := newFakeStorage()
	storage.contents[file.Path] = []byte("x")
	install := application.NewInstallAgentsFile(newFakeRepository(file), storage, installer)
	return application.NewApplyAgentsFileToProject(projects, install)
}

func TestApplyAgentsFileInstallsIntoTheProjectPath(t *testing.T) {
	file := storedFile(t, "a")
	projects := &fakeProjects{projects: []models.ProjectRef{{ID: "p1", Name: "demo", Path: "/repos/demo"}}}
	installer := &fakeInstaller{outcome: models.InstallOutcome{Committed: false}}
	uc := newApply(file, projects, installer)

	outcome, err := uc.Execute(context.Background(), file.ID, "p1", false)

	require.NoError(t, err)
	require.False(t, outcome.Committed)
	require.Equal(t, []installCall{{content: []byte("x"), dir: "/repos/demo", overwrite: false}}, installer.calls)
}

func TestApplyAgentsFileReturnsProjectNotFoundWithoutInstalling(t *testing.T) {
	file := storedFile(t, "a")
	installer := &fakeInstaller{}
	uc := newApply(file, &fakeProjects{}, installer)

	_, err := uc.Execute(context.Background(), file.ID, "missing", false)

	require.Equal(t, agentserrors.ProjectNotFound("missing"), err)
	require.Empty(t, installer.calls)
}

func TestApplyAgentsFilePropagatesLookupErrors(t *testing.T) {
	file := storedFile(t, "a")
	installer := &fakeInstaller{}
	uc := newApply(file, &fakeProjects{findErr: errors.New("lookup down")}, installer)

	_, err := uc.Execute(context.Background(), file.ID, "p1", false)

	require.EqualError(t, err, "lookup down")
	require.Empty(t, installer.calls)
}
