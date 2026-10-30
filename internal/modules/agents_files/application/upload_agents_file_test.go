package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agents_files/application"
	agentserrors "iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/domain/limits"
	"iworkflow/internal/modules/agents_files/domain/models"
)

const generatedID = "6f1c2a3e-0000-4000-8000-000000000001"

func mustID(t *testing.T, value string) models.AgentsFileID {
	t.Helper()
	id, err := models.ParseAgentsFileID(value)
	require.NoError(t, err)
	return id
}

func fixedID(t *testing.T) application.IDGenerator {
	t.Helper()
	id := mustID(t, generatedID)
	return func() (models.AgentsFileID, error) { return id, nil }
}

func fixedClock() application.Clock {
	return func() time.Time { return fixedNow }
}

func TestUploadAgentsFileStoresContentAndSavesTheReference(t *testing.T) {
	repository := newFakeRepository()
	storage := newFakeStorage()
	uc := application.NewUploadAgentsFile(repository, storage, fixedClock(), fixedID(t), quietLogger())

	file, err := uc.Execute(context.Background(), models.NewAgentsFile{Description: "  reglas ", Content: []byte("# reglas")})

	require.NoError(t, err)
	require.Equal(t, "reglas", file.Description)
	require.Equal(t, generatedID, file.ID.String())
	require.Equal(t, "/data/"+generatedID+".md", file.Path)
	require.Equal(t, fixedNow, file.CreatedAt)
	require.Equal(t, []byte("# reglas"), storage.contents[file.Path])
	require.Equal(t, 1, repository.saves)
	require.Contains(t, repository.files, generatedID)
}

func TestUploadAgentsFileRejectsInvalidInputWithoutTouchingPorts(t *testing.T) {
	tests := []struct {
		name        string
		description string
		content     []byte
		wantErr     error
	}{
		{"blank description", "   ", []byte("x"), agentserrors.InvalidDescription()},
		{"description over the limit", strings.Repeat("a", limits.MaxDescriptionChars+1), []byte("x"), agentserrors.InvalidDescription()},
		{"empty content", "d", []byte{}, agentserrors.EmptyContent()},
		{"oversized content", "d", make([]byte, limits.MaxContentBytes+1), agentserrors.TooLarge()},
		{"non UTF-8 content", "d", []byte{0xff, 0xfe, 0x00}, agentserrors.NotUTF8()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repository := newFakeRepository()
			storage := newFakeStorage()
			uc := application.NewUploadAgentsFile(repository, storage, fixedClock(), fixedID(t), quietLogger())

			_, err := uc.Execute(context.Background(), models.NewAgentsFile{Description: tc.description, Content: tc.content})

			require.Equal(t, tc.wantErr, err)
			require.Zero(t, storage.stores)
			require.Zero(t, repository.saves)
		})
	}
}

func TestUploadAgentsFileRemovesStoredFileWhenSavingFails(t *testing.T) {
	repository := newFakeRepository()
	repository.saveErr = agentserrors.Repository("boom")
	storage := newFakeStorage()
	uc := application.NewUploadAgentsFile(repository, storage, fixedClock(), fixedID(t), quietLogger())

	_, err := uc.Execute(context.Background(), models.NewAgentsFile{Description: "d", Content: []byte("x")})

	require.Equal(t, agentserrors.Repository("boom"), err)
	require.Equal(t, []string{"/data/" + generatedID + ".md"}, storage.removed)
}

func TestUploadAgentsFileReturnsTheSaveErrorWhenCleanupAlsoFails(t *testing.T) {
	repository := newFakeRepository()
	repository.saveErr = agentserrors.Repository("boom")
	storage := newFakeStorage()
	storage.removeErr = errors.New("disk gone")
	uc := application.NewUploadAgentsFile(repository, storage, fixedClock(), fixedID(t), quietLogger())

	_, err := uc.Execute(context.Background(), models.NewAgentsFile{Description: "d", Content: []byte("x")})

	require.Equal(t, agentserrors.Repository("boom"), err)
}

func TestUploadAgentsFileDoesNotSaveWhenStoringFails(t *testing.T) {
	repository := newFakeRepository()
	storage := newFakeStorage()
	storage.storeErr = agentserrors.Storage("disco lleno")
	uc := application.NewUploadAgentsFile(repository, storage, fixedClock(), fixedID(t), quietLogger())

	_, err := uc.Execute(context.Background(), models.NewAgentsFile{Description: "d", Content: []byte("x")})

	require.Equal(t, agentserrors.Storage("disco lleno"), err)
	require.Zero(t, repository.saves)
	require.Empty(t, storage.removed)
}

func TestUploadAgentsFileFailsBeforeStoringWhenIDGenerationFails(t *testing.T) {
	repository := newFakeRepository()
	storage := newFakeStorage()
	failingID := func() (models.AgentsFileID, error) { return models.AgentsFileID{}, errors.New("entropy") }
	uc := application.NewUploadAgentsFile(repository, storage, fixedClock(), failingID, quietLogger())

	_, err := uc.Execute(context.Background(), models.NewAgentsFile{Description: "d", Content: []byte("x")})

	require.ErrorContains(t, err, "generate agents file id: entropy")
	require.Zero(t, storage.stores)
	require.Zero(t, repository.saves)
}
