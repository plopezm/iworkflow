package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/terminal/application"
	terminalerrors "iworkflow/internal/modules/terminal/domain/errors"
	"iworkflow/internal/modules/terminal/domain/models"
)

func TestAttachTerminalReturnsScrollbackAndLiveOutput(t *testing.T) {
	id := models.NewTerminalSessionID()
	live := make(chan []byte, 1)
	live <- []byte("live")
	pty := &fakePty{attachment: models.NewTerminalAttachment([]byte("old"), live, nil)}
	useCase := application.NewAttachTerminal(pty)

	attachment, err := useCase.Execute(context.Background(), id)

	require.NoError(t, err)
	require.Equal(t, id, pty.attachedID)
	require.Equal(t, []byte("old"), attachment.Scrollback)
	require.Equal(t, []byte("live"), <-attachment.Output)
}

func TestAttachTerminalPropagatesSessionNotFound(t *testing.T) {
	pty := &fakePty{attachErr: terminalerrors.ErrSessionNotFound}
	useCase := application.NewAttachTerminal(pty)

	_, err := useCase.Execute(context.Background(), models.NewTerminalSessionID())

	require.ErrorIs(t, err, terminalerrors.ErrSessionNotFound)
}

func TestCloseTerminalClosesTheSession(t *testing.T) {
	id := models.NewTerminalSessionID()
	pty := &fakePty{}
	useCase := application.NewCloseTerminal(pty)

	require.NoError(t, useCase.Execute(context.Background(), id))
	require.Equal(t, id, pty.closedID)
}

func TestCloseTerminalPropagatesSessionNotFound(t *testing.T) {
	pty := &fakePty{closeErr: terminalerrors.ErrSessionNotFound}
	useCase := application.NewCloseTerminal(pty)

	err := useCase.Execute(context.Background(), models.NewTerminalSessionID())

	require.ErrorIs(t, err, terminalerrors.ErrSessionNotFound)
}

func TestCloseTerminalPropagatesThePtyError(t *testing.T) {
	pty := &fakePty{closeErr: terminalerrors.Pty(errBoom)}
	useCase := application.NewCloseTerminal(pty)

	err := useCase.Execute(context.Background(), models.NewTerminalSessionID())

	require.EqualError(t, err, "error del pseudo-terminal: boom")
}

func TestListTerminalsReturnsTheNewestFirst(t *testing.T) {
	now := time.Now().UTC()
	older := models.TerminalSession{ID: models.NewTerminalSessionID(), WorkspaceName: "old", CreatedAt: now.Add(-5 * time.Minute)}
	newer := models.TerminalSession{ID: models.NewTerminalSessionID(), WorkspaceName: "new", CreatedAt: now}
	pty := &fakePty{sessions: []models.TerminalSession{older, newer}}
	useCase := application.NewListTerminals(pty)

	sessions, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	require.Equal(t, []models.TerminalSession{newer, older}, sessions)
}

func TestListTerminalsReturnsAnEmptyList(t *testing.T) {
	useCase := application.NewListTerminals(&fakePty{})

	sessions, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	require.Empty(t, sessions)
}

func TestListTerminalsPropagatesThePtyError(t *testing.T) {
	useCase := application.NewListTerminals(&fakePty{listErr: terminalerrors.Pty(errBoom)})

	_, err := useCase.Execute(context.Background())

	require.EqualError(t, err, "error del pseudo-terminal: boom")
}

func TestResizeTerminalResizesTheSession(t *testing.T) {
	id := models.NewTerminalSessionID()
	pty := &fakePty{}
	useCase := application.NewResizeTerminal(pty)

	require.NoError(t, useCase.Execute(context.Background(), id, 100, 30))
	require.Equal(t, id, pty.resizedID)
	require.Equal(t, models.TerminalSize{Cols: 100, Rows: 30}, pty.resizedSize)
}

func TestResizeTerminalRejectsZeroSizeWithoutCallingThePty(t *testing.T) {
	pty := &fakePty{}
	useCase := application.NewResizeTerminal(pty)

	err := useCase.Execute(context.Background(), models.NewTerminalSessionID(), 80, 0)

	require.ErrorIs(t, err, terminalerrors.ErrInvalidSize)
	require.Zero(t, pty.calls)
}

func TestResizeTerminalPropagatesSessionNotFound(t *testing.T) {
	useCase := application.NewResizeTerminal(&fakePty{resizeErr: terminalerrors.ErrSessionNotFound})

	err := useCase.Execute(context.Background(), models.NewTerminalSessionID(), 80, 24)

	require.ErrorIs(t, err, terminalerrors.ErrSessionNotFound)
}

func TestResizeTerminalPropagatesThePtyError(t *testing.T) {
	useCase := application.NewResizeTerminal(&fakePty{resizeErr: terminalerrors.Pty(errBoom)})

	err := useCase.Execute(context.Background(), models.NewTerminalSessionID(), 80, 24)

	require.EqualError(t, err, "error del pseudo-terminal: boom")
}

func TestWriteTerminalWritesTheBytes(t *testing.T) {
	id := models.NewTerminalSessionID()
	pty := &fakePty{}
	useCase := application.NewWriteTerminal(pty)

	require.NoError(t, useCase.Execute(context.Background(), id, []byte("ls\n")))
	require.Equal(t, id, pty.writtenID)
	require.Equal(t, []byte("ls\n"), pty.written)
}

func TestWriteTerminalPropagatesSessionNotFound(t *testing.T) {
	useCase := application.NewWriteTerminal(&fakePty{writeErr: terminalerrors.ErrSessionNotFound})

	err := useCase.Execute(context.Background(), models.NewTerminalSessionID(), []byte{1})

	require.ErrorIs(t, err, terminalerrors.ErrSessionNotFound)
}

func TestWriteTerminalPropagatesThePtyError(t *testing.T) {
	useCase := application.NewWriteTerminal(&fakePty{writeErr: terminalerrors.Pty(errBoom)})

	err := useCase.Execute(context.Background(), models.NewTerminalSessionID(), []byte{1})

	require.EqualError(t, err, "error del pseudo-terminal: boom")
}
