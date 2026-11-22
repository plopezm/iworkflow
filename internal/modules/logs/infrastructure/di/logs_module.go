// Package di wires the logs module: it builds the use case and the handlers from the port the
// composition root binds. The module imports no other feature.
package di

import (
	"context"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/logs/application"
	"iworkflow/internal/modules/logs/domain/ports/outbound"
	"iworkflow/internal/modules/logs/infrastructure/adapters/inbound"
)

// Deps are the ports the module is built from. The composition root supplies the adapter.
type Deps struct {
	// Reader reads the end of the application log.
	Reader outbound.LogReaderPort
}

// Module exposes the use case and mounts the card route.
type Module struct {
	GetLogs *application.GetLogs

	handlers *inbound.Handlers
}

var _ web.Module = (*Module)(nil)

// NewModule builds the use case and the handlers.
func NewModule(deps Deps) *Module {
	get := application.NewGetLogs(deps.Reader)
	return &Module{GetLogs: get, handlers: inbound.NewHandlers(get)}
}

// Routes mounts the card route. It implements web.Module.
func (m *Module) Routes(r chi.Router) {
	m.handlers.Mount(r)
}

// Card returns the log card for the request context. The settings section embeds it in Resources.
func (m *Module) Card(ctx context.Context) templ.Component {
	return m.handlers.Card(ctx)
}
