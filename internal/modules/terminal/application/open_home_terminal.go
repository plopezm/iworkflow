package application

import (
	"context"

	"github.com/google/uuid"

	"iworkflow/internal/modules/terminal/domain/models"
	"iworkflow/internal/modules/terminal/domain/ports/outbound"
)

// HomeTerminalName is the name under which sessions that belong to no workspace are registered.
const HomeTerminalName string = "Home"

// noWorkspaceColor is the color of a session that is not a workspace's: home and task sessions are never marked. It is
// named so the empty argument of Spawn, next to the name, cannot be read as a swapped one.
const noWorkspaceColor string = ""

// OpenHomeTerminal opens a session in the user's home directory, without a workspace.
type OpenHomeTerminal struct {
	pty     outbound.PtyPort
	homeDir string
}

// NewOpenHomeTerminal builds the use case.
func NewOpenHomeTerminal(pty outbound.PtyPort, homeDir string) *OpenHomeTerminal {
	return &OpenHomeTerminal{pty: pty, homeDir: homeDir}
}

// Execute spawns the session in the home directory with the nil workspace id.
func (u *OpenHomeTerminal) Execute(ctx context.Context, size models.TerminalSize) (models.TerminalSession, error) {
	session, err := u.pty.Spawn(ctx, uuid.Nil, HomeTerminalName, noWorkspaceColor, u.homeDir, size)
	return session, err //nolint:wrapcheck // port errors are domain errors; passed unchanged
}
