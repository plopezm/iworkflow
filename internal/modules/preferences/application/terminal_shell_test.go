package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/preferences/application"
	domainerrors "iworkflow/internal/modules/preferences/domain/errors"
)

func TestSaveTerminalShellStoresTheTrimmedPath(t *testing.T) {
	repository := newFakeRepository()
	shell, err := application.NewSaveTerminalShell(repository).Execute(context.Background(), ` "C:\pwsh.exe" `)
	require.NoError(t, err)
	assert.Equal(t, `C:\pwsh.exe`, shell.Path())
	assert.Equal(t, `C:\pwsh.exe`, repository.terminalShell.Path())
}

func TestSaveTerminalShellBlankRestoresTheDefault(t *testing.T) {
	repository := newFakeRepository()
	save := application.NewSaveTerminalShell(repository)
	_, err := save.Execute(context.Background(), "/bin/zsh")
	require.NoError(t, err)
	_, err = save.Execute(context.Background(), "  ")
	require.NoError(t, err)
	assert.Empty(t, repository.terminalShell.Path())
}

func TestSaveTerminalShellRejectsInvalidInputWithoutWriting(t *testing.T) {
	repository := newFakeRepository()
	_, err := application.NewSaveTerminalShell(repository).Execute(context.Background(), "a\nb")
	var invalid *domainerrors.InvalidShellError
	require.ErrorAs(t, err, &invalid)
	assert.Zero(t, repository.writes)
}

func TestSaveTerminalShellReportsStorageFailures(t *testing.T) {
	repository := newFakeRepository()
	repository.writeErr = errStorage
	_, err := application.NewSaveTerminalShell(repository).Execute(context.Background(), "/bin/zsh")
	require.ErrorIs(t, err, errStorage)
}

func TestGetTerminalShellReturnsTheStoredChoice(t *testing.T) {
	repository := newFakeRepository()
	_, err := application.NewSaveTerminalShell(repository).Execute(context.Background(), "/bin/zsh")
	require.NoError(t, err)
	shell, err := application.NewGetTerminalShell(repository).Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "/bin/zsh", shell.Path())
}

func TestPickTerminalShellStoresTheChosenPath(t *testing.T) {
	repository := newFakeRepository()
	pick := application.NewPickTerminalShell(&fakePicker{path: "/bin/fish"}, application.NewSaveTerminalShell(repository))
	require.NoError(t, pick.Execute(context.Background()))
	assert.Equal(t, "/bin/fish", repository.terminalShell.Path())
}

func TestPickTerminalShellCancelledKeepsTheChoice(t *testing.T) {
	repository := newFakeRepository()
	pick := application.NewPickTerminalShell(&fakePicker{}, application.NewSaveTerminalShell(repository))
	require.NoError(t, pick.Execute(context.Background()))
	assert.Zero(t, repository.writes)
}

func TestPickTerminalShellReportsAnUnavailableDialog(t *testing.T) {
	pick := application.NewPickTerminalShell(&fakePicker{err: domainerrors.ErrPickerUnavailable}, application.NewSaveTerminalShell(newFakeRepository()))
	require.ErrorIs(t, pick.Execute(context.Background()), domainerrors.ErrPickerUnavailable)
}
