// Package di wires the preferences module: it builds the adapters, the use cases and the handlers
// the composition root (internal/app) needs.
package di

import (
	"database/sql"
	"log/slog"

	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/preferences/application"
	"iworkflow/internal/modules/preferences/domain/models"
	"iworkflow/internal/modules/preferences/domain/ports/outbound"
	"iworkflow/internal/modules/preferences/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/preferences/infrastructure/adapters/outbound/persistence"
)

// Deps are the shared resources the module is built from. The process-level ports are supplied by
// the composition root; the module never reaches into the process itself.
type Deps struct {
	DB     *sql.DB
	Logger *slog.Logger
	// Guard is the access guard the composition root also mounts as middleware. The regenerate
	// action updates it so the new token takes effect at once.
	Guard *inbound.AccessGuard
	// Network reports the addresses of this machine.
	Network outbound.NetworkDetector
	// Runtime describes the running server (port, reachability, environment lock).
	Runtime outbound.ServerRuntime
	// Restarter asks the process to restart so saved settings apply.
	Restarter outbound.ServerRestarter
}

// Module exposes the preferences use cases and mounts the settings endpoints. The composition root
// passes the use cases the language middlewares need and mounts the middlewares itself.
type Module struct {
	GetLanguage            *application.GetLanguage
	SaveLanguage           *application.SaveLanguage
	GetNetworkAccess       *application.GetNetworkAccess
	SaveNetworkAccess      *application.SaveNetworkAccess
	GetListenPort          *application.GetListenPort
	SaveListenPort         *application.SaveListenPort
	GetRequireAccessToken  *application.GetRequireAccessToken
	SaveRequireAccessToken *application.SaveRequireAccessToken
	GetAccessToken         *application.GetAccessToken
	RegenerateAccessToken  *application.RegenerateAccessToken
	BuildNetworkAccess     *application.BuildNetworkAccess
	RequestServerRestart   *application.RequestServerRestart

	handlers *inbound.Handlers
}

var _ web.Module = (*Module)(nil)

// NewModule builds the repository once, hands it to every use case and creates the handlers.
func NewModule(deps Deps) *Module {
	repository := persistence.NewPreferencesRepository(deps.DB)
	tokens := application.NewGetAccessToken(repository, models.GenerateAccessToken)
	regenerate := application.NewRegenerateAccessToken(repository, models.GenerateAccessToken)
	saveLanguage := application.NewSaveLanguage(repository)
	saveNetwork := application.NewSaveNetworkAccess(repository)
	savePort := application.NewSaveListenPort(repository)
	saveRequireToken := application.NewSaveRequireAccessToken(repository)
	buildNetwork := application.NewBuildNetworkAccess(repository, tokens, deps.Network, deps.Runtime)
	restart := application.NewRequestServerRestart(deps.Restarter)

	return &Module{
		GetLanguage:            application.NewGetLanguage(repository),
		SaveLanguage:           saveLanguage,
		GetNetworkAccess:       application.NewGetNetworkAccess(repository),
		SaveNetworkAccess:      saveNetwork,
		GetListenPort:          application.NewGetListenPort(repository),
		SaveListenPort:         savePort,
		GetRequireAccessToken:  application.NewGetRequireAccessToken(repository),
		SaveRequireAccessToken: saveRequireToken,
		GetAccessToken:         tokens,
		RegenerateAccessToken:  regenerate,
		BuildNetworkAccess:     buildNetwork,
		RequestServerRestart:   restart,
		handlers: inbound.NewHandlers(deps.Logger, deps.Guard, inbound.UseCases{
			SaveLanguage:           saveLanguage,
			BuildNetworkAccess:     buildNetwork,
			SaveNetworkAccess:      saveNetwork,
			SaveListenPort:         savePort,
			SaveRequireAccessToken: saveRequireToken,
			RegenerateAccessToken:  regenerate,
			RequestServerRestart:   restart,
		}),
	}
}

// Routes mounts the settings endpoints. The module has no tab page of its own: its UI is the
// network access and language sections of the settings screen.
func (m *Module) Routes(r chi.Router) {
	m.handlers.Routes(r)
}
