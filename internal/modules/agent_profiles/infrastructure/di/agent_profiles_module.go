// Package di is the composition root of the agent_profiles module. It builds the file adapters and
// the use cases. Handlers and routes come in a later phase.
package di

import (
	"iworkflow/internal/modules/agent_profiles/application"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
	"iworkflow/internal/modules/agent_profiles/infrastructure/adapters/outbound/exchange"
	"iworkflow/internal/modules/agent_profiles/infrastructure/adapters/outbound/profiles"
	"iworkflow/internal/modules/agent_profiles/infrastructure/adapters/outbound/roles"
	"iworkflow/internal/modules/agent_profiles/infrastructure/adapters/outbound/routing"
)

// Deps are the resources the module needs from the composition root.
type Deps struct {
	// ProfilesDir is the gentle-pi config home. It holds profiles.json and models.json.
	ProfilesDir string
	// AgentsDir is the pi agents folder, where every role is a `<role>.md` file.
	AgentsDir string
	// ModelOptions lists the models offered by the routing editors. It is provided by the composition
	// root, which adapts the model catalog of another module.
	ModelOptions outbound.ModelOptionsPort
}

// Module exposes the use cases of the agent_profiles module.
type Module struct {
	ListProfiles      *application.ListProfiles
	CreateProfile     *application.CreateProfile
	DuplicateProfile  *application.DuplicateProfile
	RenameProfile     *application.RenameProfile
	DeleteProfile     *application.DeleteProfile
	SetProfileRouting *application.SetProfileRouting
	ExportProfile     *application.ExportProfile
	ImportProfile     *application.ImportProfile
	ListAgentRoles    *application.ListAgentRoles
	CreateAgentRole   *application.CreateAgentRole
	GetGlobalRouting  *application.GetGlobalRouting
	SetGlobalRouting  *application.SetGlobalRouting
	ListModelOptions  *application.ListModelOptions
}

// NewModule wires the adapters into the use cases. Every use case that mutates profiles.json
// shares one write guard, so concurrent mutations cannot lose each other's changes.
func NewModule(deps Deps) *Module {
	store := profiles.NewProfileStoreFileAdapter(deps.ProfilesDir)
	globalRouting := routing.NewGlobalRoutingFileAdapter(deps.ProfilesDir)
	exchangeAdapter := exchange.NewProfileExchangeJSONAdapter()
	agentRoles := roles.NewAgentRoleDirAdapter(deps.AgentsDir)
	guard := application.NewWriteGuard()
	return &Module{
		ListProfiles:      application.NewListProfiles(store),
		CreateProfile:     application.NewCreateProfile(store, guard),
		DuplicateProfile:  application.NewDuplicateProfile(store, guard),
		RenameProfile:     application.NewRenameProfile(store, guard),
		DeleteProfile:     application.NewDeleteProfile(store, guard),
		SetProfileRouting: application.NewSetProfileRouting(store, guard),
		ExportProfile:     application.NewExportProfile(store, exchangeAdapter),
		ImportProfile:     application.NewImportProfile(store, exchangeAdapter, guard),
		ListAgentRoles:    application.NewListAgentRoles(agentRoles),
		CreateAgentRole:   application.NewCreateAgentRole(agentRoles),
		GetGlobalRouting:  application.NewGetGlobalRouting(globalRouting),
		SetGlobalRouting:  application.NewSetGlobalRouting(globalRouting),
		ListModelOptions:  application.NewListModelOptions(deps.ModelOptions),
	}
}
