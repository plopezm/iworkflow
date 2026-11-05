// Package di wires the data_home module: it builds the use cases and the handlers from the ports the
// composition root binds. The module imports no other feature.
package di

import (
	"context"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/data_home/application"
	"iworkflow/internal/modules/data_home/domain/ports/outbound"
	"iworkflow/internal/modules/data_home/infrastructure/adapters/inbound"
)

// Deps are the ports the module is built from. The composition root supplies the adapters.
type Deps struct {
	// Mover reads and changes the data folder through the move engine.
	Mover outbound.HomeMoverPort
	// ActiveTasks tells whether a move must wait because a task is starting, running or finishing.
	ActiveTasks outbound.ActiveTasksPort
	// Restarter asks the process to restart so a recorded move runs at the next start.
	Restarter outbound.ServerRestarterPort
}

// Module exposes the use cases and mounts the card routes.
type Module struct {
	GetDataHome *application.GetDataHome
	RequestMove *application.RequestDataHomeMove
	CancelMove  *application.CancelDataHomeMove

	handlers *inbound.Handlers
}

var _ web.Module = (*Module)(nil)

// NewModule builds the use cases and the handlers.
func NewModule(deps Deps) *Module {
	getHome := application.NewGetDataHome(deps.Mover)
	request := application.NewRequestDataHomeMove(deps.Mover, deps.ActiveTasks, deps.Restarter)
	cancel := application.NewCancelDataHomeMove(deps.Mover)
	return &Module{
		GetDataHome: getHome,
		RequestMove: request,
		CancelMove:  cancel,
		handlers:    inbound.NewHandlers(getHome, request, cancel),
	}
}

// Routes mounts the card routes. It implements web.Module.
func (m *Module) Routes(r chi.Router) {
	m.handlers.Mount(r)
}

// Card returns the data folder card for the request context. The settings section embeds it above
// the resource limits card.
func (m *Module) Card(ctx context.Context) templ.Component {
	return m.handlers.Card(ctx)
}
