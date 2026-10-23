// Package di composes the agent settings module: it builds the adapters and the use cases with
// constructor injection. The composition root (internal/app) wires the use cases into the rest of
// the application.
package di

import (
	"database/sql"

	"iworkflow/internal/modules/agent_settings/application"
	"iworkflow/internal/modules/agent_settings/domain/models"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/outbound/cli"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/outbound/git"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/outbound/persistence"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/outbound/profiles"
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
	return &Module{
		GetAgentSettings:    application.NewGetAgentSettings(repository),
		ListModels:          application.NewListModels(repository, modelCatalog),
		ListProfiles:        application.NewListProfiles(profileCatalog),
		UpdateAgentSettings: application.NewUpdateAgentSettings(repository, modelCatalog, profileCatalog),
		PrepareLaunch:       application.NewPrepareLaunch(repository, pin),
	}
}
