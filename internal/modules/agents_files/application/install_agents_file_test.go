package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agents_files/application"
	agentserrors "iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/domain/models"
)

func TestInstallAgentsFileReadsContentAndInstallsIt(t *testing.T) {
	file := storedFile(t, "a")
	repository := newFakeRepository(file)
	storage := newFakeStorage()
	storage.contents[file.Path] = []byte("# reglas")
	installer := &fakeInstaller{outcome: models.InstallOutcome{Committed: true}}
	uc := application.NewInstallAgentsFile(repository, storage, installer)

	outcome, err := uc.Execute(context.Background(), file.ID, "/repos/demo", true)

	require.NoError(t, err)
	require.True(t, outcome.Committed)
	require.Equal(t, []installCall{{content: []byte("# reglas"), dir: "/repos/demo", overwrite: true}}, installer.calls)
}

func TestInstallAgentsFileReturnsNotFoundForUnknownFile(t *testing.T) {
	id := mustID(t, generatedID)
	installer := &fakeInstaller{}
	uc := application.NewInstallAgentsFile(newFakeRepository(), newFakeStorage(), installer)

	_, err := uc.Execute(context.Background(), id, "/repos/demo", false)

	require.Equal(t, agentserrors.NotFound(id.String()), err)
	require.Empty(t, installer.calls)
}

func TestInstallAgentsFilePropagatesTargetExists(t *testing.T) {
	file := storedFile(t, "a")
	repository := newFakeRepository(file)
	storage := newFakeStorage()
	storage.contents[file.Path] = []byte("x")
	installer := &fakeInstaller{err: agentserrors.TargetExists("/repos/demo/AGENTS.md")}
	uc := application.NewInstallAgentsFile(repository, storage, installer)

	_, err := uc.Execute(context.Background(), file.ID, "/repos/demo", false)

	require.Equal(t, agentserrors.TargetExists("/repos/demo/AGENTS.md"), err)
}

func TestInstallAgentsFileDoesNotInstallWhenTheStoredContentCannotBeRead(t *testing.T) {
	file := storedFile(t, "a")
	repository := newFakeRepository(file)
	storage := newFakeStorage()
	storage.readErr = agentserrors.Storage("unreadable")
	installer := &fakeInstaller{}
	uc := application.NewInstallAgentsFile(repository, storage, installer)

	_, err := uc.Execute(context.Background(), file.ID, "/repos/demo", false)

	require.Equal(t, agentserrors.Storage("unreadable"), err)
	require.Empty(t, installer.calls)
}
