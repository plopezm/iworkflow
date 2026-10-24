// Package application holds the terminal use cases. Each one is a struct with Execute.
package application

import (
	"context"

	"github.com/google/uuid"

	"iworkflow/internal/modules/terminal/domain/models"
	"iworkflow/internal/modules/terminal/domain/ports/outbound"
)

// OpenTerminal opens a session in the directory of a workspace.
type OpenTerminal struct {
	pty   outbound.PtyPort
	paths outbound.WorkspacePathPort
}

// NewOpenTerminal builds the use case.
func NewOpenTerminal(pty outbound.PtyPort, paths outbound.WorkspacePathPort) *OpenTerminal {
	return &OpenTerminal{pty: pty, paths: paths}
}

// Execute resolves the workspace and spawns the session with the workspace name as snapshot.
func (u *OpenTerminal) Execute(ctx context.Context, workspaceID uuid.UUID, size models.TerminalSize) (models.TerminalSession, error) {
	target, err := u.paths.Resolve(ctx, workspaceID)
	if err != nil {
		return models.TerminalSession{}, err //nolint:wrapcheck // port errors are domain errors; passed unchanged
	}
	session, err := u.pty.Spawn(ctx, workspaceID, target.Name, target.Path, size)
	return session, err //nolint:wrapcheck // port errors are domain errors; passed unchanged
}
