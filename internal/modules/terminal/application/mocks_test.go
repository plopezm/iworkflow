package application_test

import (
	"context"

	"github.com/google/uuid"

	terminalerrors "iworkflow/internal/modules/terminal/domain/errors"
	"iworkflow/internal/modules/terminal/domain/models"
)

// spawnCall records one Spawn invocation.
type spawnCall struct {
	workspaceID   uuid.UUID
	workspaceName string
	cwd           string
	size          models.TerminalSize
}

// fakePty is an in-memory PtyPort that records the calls and returns configured results.
type fakePty struct {
	spawned     models.TerminalSession
	spawnErr    error
	spawnCalls  []spawnCall
	sessions    []models.TerminalSession
	listErr     error
	attachment  models.TerminalAttachment
	attachErr   error
	attachedID  models.TerminalSessionID
	writeErr    error
	written     []byte
	writtenID   models.TerminalSessionID
	resizeErr   error
	resizedID   models.TerminalSessionID
	resizedSize models.TerminalSize
	closeErr    error
	closedID    models.TerminalSessionID
	calls       int
}

func (f *fakePty) Spawn(_ context.Context, workspaceID uuid.UUID, workspaceName string, cwd string, size models.TerminalSize) (models.TerminalSession, error) {
	f.calls++
	f.spawnCalls = append(f.spawnCalls, spawnCall{workspaceID: workspaceID, workspaceName: workspaceName, cwd: cwd, size: size})
	return f.spawned, f.spawnErr
}

func (f *fakePty) List(_ context.Context) ([]models.TerminalSession, error) {
	f.calls++
	return f.sessions, f.listErr
}

func (f *fakePty) Attach(_ context.Context, id models.TerminalSessionID) (models.TerminalAttachment, error) {
	f.calls++
	f.attachedID = id
	return f.attachment, f.attachErr
}

func (f *fakePty) Write(_ context.Context, id models.TerminalSessionID, data []byte) error {
	f.calls++
	f.writtenID = id
	f.written = data
	return f.writeErr
}

func (f *fakePty) Resize(_ context.Context, id models.TerminalSessionID, size models.TerminalSize) error {
	f.calls++
	f.resizedID = id
	f.resizedSize = size
	return f.resizeErr
}

func (f *fakePty) Close(_ context.Context, id models.TerminalSessionID) error {
	f.calls++
	f.closedID = id
	return f.closeErr
}

// fakeWorkspacePaths resolves one configured target, or fails with err.
type fakeWorkspacePaths struct {
	target models.WorkspaceTarget
	err    error
}

func (f *fakeWorkspacePaths) Resolve(_ context.Context, _ uuid.UUID) (models.WorkspaceTarget, error) {
	if f.err != nil {
		return models.WorkspaceTarget{}, f.err
	}
	return f.target, nil
}

// fakeTaskPaths resolves one configured target, or reports the task as missing.
type fakeTaskPaths struct {
	target models.TaskTarget
	found  bool
}

func (f *fakeTaskPaths) Resolve(_ context.Context, _ uuid.UUID) (models.TaskTarget, error) {
	if !f.found {
		return models.TaskTarget{}, terminalerrors.ErrTaskNotFound
	}
	return f.target, nil
}
