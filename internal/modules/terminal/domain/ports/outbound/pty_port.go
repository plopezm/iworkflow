// Package outbound holds the ports the terminal module needs from infrastructure and from other
// modules. The use cases depend on these interfaces, never on their implementations.
package outbound

import (
	"context"

	"github.com/google/uuid"

	"iworkflow/internal/modules/terminal/domain/models"
)

// PtyPort manages the pseudo-terminal sessions.
type PtyPort interface {
	// Spawn starts a persistent session in cwd. The registry keeps workspaceID and workspaceName
	// as metadata and assigns the id, the creation time and the status.
	Spawn(ctx context.Context, workspaceID uuid.UUID, workspaceName string, cwd string, size models.TerminalSize) (models.TerminalSession, error)
	// List returns every session in the registry, including exited ones.
	List(ctx context.Context) ([]models.TerminalSession, error)
	// Attach returns the scrollback and the live output. It may be called several times per session.
	Attach(ctx context.Context, id models.TerminalSessionID) (models.TerminalAttachment, error)
	Write(ctx context.Context, id models.TerminalSessionID, data []byte) error
	Resize(ctx context.Context, id models.TerminalSessionID, size models.TerminalSize) error
	// Close kills the session's processes and removes it from the registry.
	Close(ctx context.Context, id models.TerminalSessionID) error
}
