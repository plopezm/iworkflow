package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/terminal/application"
	terminalerrors "iworkflow/internal/modules/terminal/domain/errors"
	"iworkflow/internal/modules/terminal/domain/models"
)

var testSize = models.TerminalSize{Cols: 80, Rows: 24}

func TestOpenTerminalSpawnsWithTheWorkspaceSnapshot(t *testing.T) {
	workspaceID := uuid.New()
	expected := models.TerminalSession{
		ID:            models.NewTerminalSessionID(),
		WorkspaceID:   workspaceID,
		WorkspaceName: "demo",
		CreatedAt:     time.Now().UTC(),
		Status:        models.TerminalStatusRunning,
	}
	pty := &fakePty{spawned: expected}
	paths := &fakeWorkspacePaths{target: models.WorkspaceTarget{Name: "demo", Path: "/tmp/ws"}}
	useCase := application.NewOpenTerminal(pty, paths)

	session, err := useCase.Execute(context.Background(), workspaceID, testSize)

	require.NoError(t, err)
	require.Equal(t, expected, session)
	require.Equal(t, []spawnCall{{workspaceID: workspaceID, workspaceName: "demo", cwd: "/tmp/ws", size: testSize}}, pty.spawnCalls)
}

func TestOpenTerminalFailsWhenTheWorkspaceIsNotFound(t *testing.T) {
	pty := &fakePty{}
	paths := &fakeWorkspacePaths{err: terminalerrors.ErrWorkspaceNotFound}
	useCase := application.NewOpenTerminal(pty, paths)

	_, err := useCase.Execute(context.Background(), uuid.New(), testSize)

	require.ErrorIs(t, err, terminalerrors.ErrWorkspaceNotFound)
	require.EqualError(t, err, "el workspace no existe")
	require.Zero(t, pty.calls)
}

func TestOpenTerminalPropagatesThePtyError(t *testing.T) {
	pty := &fakePty{spawnErr: terminalerrors.Pty(errBoom)}
	paths := &fakeWorkspacePaths{target: models.WorkspaceTarget{Name: "demo", Path: "/tmp/ws"}}
	useCase := application.NewOpenTerminal(pty, paths)

	_, err := useCase.Execute(context.Background(), uuid.New(), testSize)

	require.ErrorIs(t, err, terminalerrors.ErrPty)
	require.EqualError(t, err, "error del pseudo-terminal: boom")
}
