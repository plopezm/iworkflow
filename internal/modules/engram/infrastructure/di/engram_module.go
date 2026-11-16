// Package di is the composition root of the engram module: it builds the switchable HTTP adapter, the settings
// store, the server launcher and the use cases.
package di

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/modules/engram/application"
	"iworkflow/internal/modules/engram/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/engram/infrastructure/adapters/outbound/api"
	"iworkflow/internal/modules/engram/infrastructure/adapters/outbound/persistence"
	"iworkflow/internal/modules/engram/infrastructure/adapters/outbound/process"
	"iworkflow/internal/modules/engram/presentation/components"
)

// Executable is the engram program iworkflow starts when the server is off.
const Executable = "engram"

// Deps are what the module needs.
type Deps struct {
	DB *sql.DB
	// EnvURL is the raw value of IWF_ENGRAM_URL, possibly blank or invalid.
	EnvURL string
	// Executable is the engram program; empty means Executable.
	Executable string
	Logger     *slog.Logger
}

// Module exposes the read-only use cases and the settings card of the engram module. The Engram address can change at
// runtime: Apply points the client at the effective address.
type Module struct {
	ListWorkspacesMemory *application.ListWorkspacesMemory
	RecentMemories       *application.RecentMemories
	ListMemories         *application.ListMemories
	ReadMemory           *application.ReadMemory
	GetSettings          *application.GetServerSettings
	SaveSettings         *application.SaveServerSettings
	Apply                *application.ApplyServerSettings
	handlers             *inbound.Handlers
}

// NewModule wires the adapters into the use cases. The client has no address until Apply runs, so every read is
// unavailable before then.
func NewModule(deps Deps) *Module {
	executable := deps.Executable
	if executable == "" {
		executable = Executable
	}
	store := api.NewSwitchableStore()
	server := process.NewServer(executable)
	settingsStore := persistence.NewSettingsStore(deps.DB)

	get := application.NewGetServerSettings(settingsStore, deps.EnvURL)
	save := application.NewSaveServerSettings(settingsStore)
	apply := application.NewApplyServerSettings(get, store, application.NewEnsureRunning(server, sleep))
	check := application.NewCheckServer(server)

	return &Module{
		ListWorkspacesMemory: application.NewListWorkspacesMemory(store),
		RecentMemories:       application.NewRecentMemories(store),
		ListMemories:         application.NewListMemories(store),
		ReadMemory:           application.NewReadMemory(store),
		GetSettings:          get,
		SaveSettings:         save,
		Apply:                apply,
		handlers: inbound.NewHandlers(inbound.UseCases{
			Get: get, Save: save, Apply: apply, Check: check,
		}, deps.Logger),
	}
}

// Routes mounts the settings card endpoints.
func (m *Module) Routes(r chi.Router) {
	m.handlers.Mount(r)
}

// SettingsContent is the content of the Engram section of Configuración.
func (m *Module) SettingsContent() func(ctx context.Context) templ.Component {
	return func(context.Context) templ.Component { return components.ServerPlaceholder() }
}

// sleep waits d or until ctx ends.
func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err() //nolint:wrapcheck // the caller wraps the cancellation
	case <-timer.C:
		return nil
	}
}
