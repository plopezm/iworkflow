// Package di composes the terminal module: it builds the adapters and the use cases with constructor
// injection. The composition root (internal/app) wires the use cases and the WebSocket transport.
package di

import (
	"context"

	"iworkflow/internal/modules/terminal/application"
	"iworkflow/internal/modules/terminal/infrastructure/adapters/outbound/pty"
	"iworkflow/internal/modules/terminal/infrastructure/adapters/outbound/resolver"
)

// Dependencies are the external inputs needed to build the module.
type Dependencies struct {
	// WorkspaceResolver and TaskResolver are built by the composition root over the other modules.
	WorkspaceResolver resolver.WorkspaceResolver
	TaskResolver      resolver.TaskResolver
	// HomeDir is the working directory of the home terminal.
	HomeDir string
	// Shell is the fixed shell. Empty means the user's $SHELL.
	Shell string
}

// Module exposes the use cases of the terminal module.
type Module struct {
	OpenTerminal     *application.OpenTerminal
	OpenTaskTerminal *application.OpenTaskTerminal
	OpenHomeTerminal *application.OpenHomeTerminal
	WriteTerminal    *application.WriteTerminal
	ResizeTerminal   *application.ResizeTerminal
	CloseTerminal    *application.CloseTerminal
	AttachTerminal   *application.AttachTerminal
	ListTerminals    *application.ListTerminals

	sessions *pty.Adapter
}

// Shutdown kills every live session and waits for their shells to be reaped. The composition root calls
// it when the server stops, so that no shell outlives the process.
func (m *Module) Shutdown(ctx context.Context) error {
	return m.sessions.Shutdown(ctx) //nolint:wrapcheck // the adapter already reports domain errors
}

// NewModule builds the adapters and the use cases of the module.
func NewModule(deps Dependencies) *Module {
	sessions := pty.NewAdapter(deps.Shell)
	paths := resolver.NewWorkspacePathAdapter(deps.WorkspaceResolver)
	taskPaths := resolver.NewTaskPathAdapter(deps.TaskResolver)
	return &Module{
		OpenTerminal:     application.NewOpenTerminal(sessions, paths),
		OpenTaskTerminal: application.NewOpenTaskTerminal(sessions, taskPaths),
		OpenHomeTerminal: application.NewOpenHomeTerminal(sessions, deps.HomeDir),
		WriteTerminal:    application.NewWriteTerminal(sessions),
		ResizeTerminal:   application.NewResizeTerminal(sessions),
		CloseTerminal:    application.NewCloseTerminal(sessions),
		AttachTerminal:   application.NewAttachTerminal(sessions),
		ListTerminals:    application.NewListTerminals(sessions),
		sessions:         sessions,
	}
}
