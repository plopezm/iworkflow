package application

import (
	"context"

	"github.com/google/uuid"

	"iworkflow/internal/modules/terminal/domain/models"
	"iworkflow/internal/modules/terminal/domain/ports/outbound"
)

// OpenTaskTerminal opens a session in the worktree of a task.
type OpenTaskTerminal struct {
	pty   outbound.PtyPort
	paths outbound.TaskPathPort
}

// NewOpenTaskTerminal builds the use case.
func NewOpenTaskTerminal(pty outbound.PtyPort, paths outbound.TaskPathPort) *OpenTaskTerminal {
	return &OpenTaskTerminal{pty: pty, paths: paths}
}

// Execute resolves the task and spawns the session in its worktree, under the task's workspace.
func (u *OpenTaskTerminal) Execute(ctx context.Context, taskID uuid.UUID, size models.TerminalSize) (models.TerminalSession, error) {
	target, err := u.paths.Resolve(ctx, taskID)
	if err != nil {
		return models.TerminalSession{}, err //nolint:wrapcheck // port errors are domain errors; passed unchanged
	}
	session, err := u.pty.Spawn(ctx, target.WorkspaceID, target.Name, target.Path, size)
	return session, err //nolint:wrapcheck // port errors are domain errors; passed unchanged
}
