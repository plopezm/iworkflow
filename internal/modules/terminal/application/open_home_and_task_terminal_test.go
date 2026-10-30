package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/terminal/application"
	terminalerrors "iworkflow/internal/modules/terminal/domain/errors"
	"iworkflow/internal/modules/terminal/domain/models"
)

var errBoom = errors.New("boom")

func TestOpenHomeTerminalSpawnsInHomeWithoutWorkspace(t *testing.T) {
	expected := models.TerminalSession{
		ID:            models.NewTerminalSessionID(),
		WorkspaceName: application.HomeTerminalName,
		CreatedAt:     time.Now().UTC(),
		Status:        models.TerminalStatusRunning,
	}
	pty := &fakePty{spawned: expected}
	useCase := application.NewOpenHomeTerminal(pty, "/home/u")

	session, err := useCase.Execute(context.Background(), testSize)

	require.NoError(t, err)
	require.Equal(t, expected, session)
	require.Equal(t, []spawnCall{{workspaceID: uuid.Nil, workspaceName: "Home", cwd: "/home/u", size: testSize}}, pty.spawnCalls)
}

func TestOpenHomeTerminalPropagatesThePtyError(t *testing.T) {
	pty := &fakePty{spawnErr: terminalerrors.Pty(errBoom)}
	useCase := application.NewOpenHomeTerminal(pty, "/home/u")

	_, err := useCase.Execute(context.Background(), testSize)

	require.EqualError(t, err, "error del pseudo-terminal: boom")
}

func TestOpenTaskTerminalSpawnsInTheWorktreeUnderTheTaskWorkspace(t *testing.T) {
	taskID := uuid.New()
	workspaceID := uuid.New()
	expected := models.TerminalSession{ID: models.NewTerminalSessionID(), WorkspaceID: workspaceID, WorkspaceName: "fix bug", Status: models.TerminalStatusRunning}
	pty := &fakePty{spawned: expected}
	paths := &fakeTaskPaths{found: true, target: models.TaskTarget{WorkspaceID: workspaceID, Name: "fix bug", Path: "/tmp/wt"}}
	useCase := application.NewOpenTaskTerminal(pty, paths)

	session, err := useCase.Execute(context.Background(), taskID, testSize)

	require.NoError(t, err)
	require.Equal(t, expected, session)
	require.Equal(t, []spawnCall{{workspaceID: workspaceID, workspaceName: "fix bug", cwd: "/tmp/wt", size: testSize}}, pty.spawnCalls)
}

func TestOpenTaskTerminalFailsWhenTheTaskIsNotFound(t *testing.T) {
	pty := &fakePty{}
	useCase := application.NewOpenTaskTerminal(pty, &fakeTaskPaths{found: false})

	_, err := useCase.Execute(context.Background(), uuid.New(), testSize)

	require.ErrorIs(t, err, terminalerrors.ErrTaskNotFound)
	require.Zero(t, pty.calls)
}

func TestOpenTaskTerminalPropagatesThePtyError(t *testing.T) {
	pty := &fakePty{spawnErr: terminalerrors.Pty(errBoom)}
	paths := &fakeTaskPaths{found: true, target: models.TaskTarget{Name: "t", Path: "/tmp/wt"}}
	useCase := application.NewOpenTaskTerminal(pty, paths)

	_, err := useCase.Execute(context.Background(), uuid.New(), testSize)

	require.EqualError(t, err, "error del pseudo-terminal: boom")
}
