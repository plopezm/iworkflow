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

func TestDeleteAgentsFileDeletesRowThenFile(t *testing.T) {
	file := storedFile(t, "a")
	repository := newFakeRepository(file)
	storage := newFakeStorage()
	uc := application.NewDeleteAgentsFile(repository, storage)

	err := uc.Execute(context.Background(), file.ID)

	require.NoError(t, err)
	require.Equal(t, []models.AgentsFileID{file.ID}, repository.deleted)
	require.Equal(t, []string{file.Path}, storage.removed)
}

func TestDeleteAgentsFileReturnsNotFoundWhenMissing(t *testing.T) {
	id := mustID(t, generatedID)
	repository := newFakeRepository()
	storage := newFakeStorage()
	uc := application.NewDeleteAgentsFile(repository, storage)

	err := uc.Execute(context.Background(), id)

	require.Equal(t, agentserrors.NotFound(id.String()), err)
	require.Empty(t, repository.deleted)
	require.Empty(t, storage.removed)
}

func TestDeleteAgentsFileKeepsTheFileWhenRowDeletionFails(t *testing.T) {
	file := storedFile(t, "a")
	repository := newFakeRepository(file)
	repository.deleteErr = agentserrors.Repository("boom")
	storage := newFakeStorage()
	uc := application.NewDeleteAgentsFile(repository, storage)

	err := uc.Execute(context.Background(), file.ID)

	require.Equal(t, agentserrors.Repository("boom"), err)
	require.Empty(t, storage.removed)
}

func TestDeleteAgentsFilePropagatesStorageErrorAfterTheRowIsGone(t *testing.T) {
	file := storedFile(t, "a")
	repository := newFakeRepository(file)
	storage := newFakeStorage()
	storage.removeErr = errors.New("permission denied")
	uc := application.NewDeleteAgentsFile(repository, storage)

	err := uc.Execute(context.Background(), file.ID)

	require.EqualError(t, err, "permission denied")
	require.Equal(t, []models.AgentsFileID{file.ID}, repository.deleted)
}
