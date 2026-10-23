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

func storedFile(t *testing.T, description string) models.AgentsFile {
	t.Helper()
	id, err := models.NewAgentsFileID()
	require.NoError(t, err)
	return models.AgentsFile{ID: id, Description: description, Path: "/data/" + description + ".md", CreatedAt: fixedNow}
}

func TestListAgentsFilesSortsByDescriptionCaseInsensitive(t *testing.T) {
	repository := newFakeRepository(
		storedFile(t, "beta"),
		storedFile(t, "Alpha"),
		storedFile(t, "gamma"),
	)
	uc := application.NewListAgentsFiles(repository)

	files, err := uc.Execute(context.Background())

	require.NoError(t, err)
	descriptions := make([]string, 0, len(files))
	for _, file := range files {
		descriptions = append(descriptions, file.Description)
	}
	require.Equal(t, []string{"Alpha", "beta", "gamma"}, descriptions)
}

func TestListAgentsFilesReturnsEmptyListWhenNothingIsStored(t *testing.T) {
	uc := application.NewListAgentsFiles(newFakeRepository())

	files, err := uc.Execute(context.Background())

	require.NoError(t, err)
	require.Empty(t, files)
}

func TestListAgentsFilesPropagatesRepositoryErrors(t *testing.T) {
	repository := newFakeRepository()
	repository.listErr = agentserrors.Repository("boom")
	uc := application.NewListAgentsFiles(repository)

	_, err := uc.Execute(context.Background())

	require.Equal(t, agentserrors.Repository("boom"), err)
	require.False(t, errors.Is(err, agentserrors.NotFound("")))
}
