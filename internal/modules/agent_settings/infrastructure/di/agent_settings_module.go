// Package di composes the agent settings module: it builds the adapters and the use cases with
// constructor injection. The composition root (internal/app) wires the use cases into the rest of
// the application.
package di

import (
	"context"
	"database/sql"

	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/modules/agent_settings/application"
	"iworkflow/internal/modules/agent_settings/domain/models"
	outbound "iworkflow/internal/modules/agent_settings/domain/ports/outbound"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/outbound/cli"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/outbound/git"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/outbound/persistence"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/outbound/profiles"
	"iworkflow/internal/modules/agent_settings/presentation/pages"
)

// Dependencies are the external inputs needed to build the module.
type Dependencies struct {
	// DB is the migrated SQLite database.
	DB *sql.DB
	// GentleShellBin, PiBin and CursorBin are the agent executables of each runtime.
	GentleShellBin string
	PiBin          string
	CursorBin      string
	// GentlePiConfigHome holds gentle-pi's profiles.json.
	GentlePiConfigHome string
}

// Module exposes the use cases of the agent settings module.
type Module struct {
	GetAgentSettings    *application.GetAgentSettings
	ListModels          *application.ListModels
	ListProfiles        *application.ListProfiles
	UpdateAgentSettings *application.UpdateAgentSettings
	// PrepareLaunch is consumed by other modules (the tasks launch) through the composition root.
	PrepareLaunch *application.PrepareLaunch
	ModelCatalog  outbound.ModelCatalogPort

	handlers *inbound.Handlers
}

// Routes mounts the htmx fragments and the JSON API of the module. It implements web.Module.
func (m *Module) Routes(r chi.Router) {
	m.handlers.Mount(r)
}

// Tabs returns the dock routes of the settings screen. The composition root passes the sections in
// display order; the resources section comes from ResourcesSection.
func (m *Module) Tabs(sections []pages.SettingsSection) []dock.Route {
	return m.handlers.SettingsTabs(sections)
}

// ResourcesSection returns the resource limits section for the settings screen, labelled by label.
func (m *Module) ResourcesSection(label func(ctx context.Context) string) pages.SettingsSection {
	return m.handlers.ResourcesSection(label)
}

// NewModule builds the adapters and the use cases of the module.
func NewModule(deps Dependencies) *Module {
	repository := persistence.NewAgentSettingsRepositoryAdapter(deps.DB)
	modelCatalog := cli.NewAgentCLIModelCatalogAdapter(map[models.AgentRuntime]string{
		models.RuntimeGentleShell: deps.GentleShellBin,
		models.RuntimePi:          deps.PiBin,
		models.RuntimeCursor:      deps.CursorBin,
	})
	profileCatalog := profiles.NewProfilesFileCatalogAdapter(deps.GentlePiConfigHome)
	pin := git.NewProfilePinAdapter()
	getSettings := application.NewGetAgentSettings(repository)
	listModels := application.NewListModels(repository, modelCatalog)
	listProfiles := application.NewListProfiles(profileCatalog)
	update := application.NewUpdateAgentSettings(repository, modelCatalog, profileCatalog)
	return &Module{
		GetAgentSettings:    getSettings,
		ListModels:          listModels,
		ListProfiles:        listProfiles,
		UpdateAgentSettings: update,
		PrepareLaunch:       application.NewPrepareLaunch(repository, pin),
		ModelCatalog:        modelCatalog,
		handlers:            inbound.NewHandlers(getSettings, update, listModels, listProfiles),
	}
}
