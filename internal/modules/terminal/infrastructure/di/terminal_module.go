// Package di composes the terminal module: it builds the adapters, the use cases and the HTTP handlers
// with constructor injection. The composition root (internal/app) mounts the routes and registers the
// dock tabs, and calls Shutdown when the server stops.
package di

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/modules/terminal/application"
	"iworkflow/internal/modules/terminal/infrastructure/adapters/inbound"
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
	// Logger receives the diagnostics of the WebSocket transport. Nil means the default logger.
	Logger *slog.Logger
}

// Module exposes the use cases of the terminal module, its routes and its dock tabs.
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
	handlers *inbound.Handlers
	socket   *inbound.TerminalWebSocket
}

// Routes mounts the htmx endpoints and the WebSocket of the module.
func (m *Module) Routes(r chi.Router) {
	m.handlers.Mount(r)
	r.Get(inbound.WebSocketPath, m.socket.ServeHTTP)
}

// Tabs returns the dock routes of the module: the sessions list, one session, and the new terminal of a
// workspace or of a task. The composition root registers them in the dock registry.
func (m *Module) Tabs() []dock.Route {
	return m.handlers.Tabs()
}

// Shutdown kills every live session and waits for their shells to be reaped. The composition root calls
// it when the server stops, so that no shell outlives the process.
func (m *Module) Shutdown(ctx context.Context) error {
	return m.sessions.Shutdown(ctx) //nolint:wrapcheck // the adapter already reports domain errors
}

// NewModule builds the adapters, the use cases and the HTTP handlers of the module.
func NewModule(deps Dependencies) *Module {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	sessions := pty.NewAdapter(deps.Shell)
	paths := resolver.NewWorkspacePathAdapter(deps.WorkspaceResolver)
	taskPaths := resolver.NewTaskPathAdapter(deps.TaskResolver)
	module := &Module{
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
	module.handlers = inbound.NewHandlers(inbound.UseCases{
		List:          module.ListTerminals,
		OpenWorkspace: module.OpenTerminal,
		OpenTask:      module.OpenTaskTerminal,
		OpenHome:      module.OpenHomeTerminal,
		Close:         module.CloseTerminal,
	})
	module.socket = inbound.NewTerminalWebSocket(module.AttachTerminal, module.WriteTerminal, module.ResizeTerminal, logger)
	return module
}
