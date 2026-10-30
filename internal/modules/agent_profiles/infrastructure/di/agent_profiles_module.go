// Package di is the composition root of the agent_profiles module. It builds the file adapters, the
// use cases and the handlers, and exposes the routes and the dock tabs the parent registers.
package di

import (
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/modules/agent_profiles/application"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
	"iworkflow/internal/modules/agent_profiles/infrastructure/adapters/inbound"
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

// Module exposes the use cases of the agent_profiles module, its routes and its dock tabs.
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

	handlers *inbound.Handlers
}

// Routes mounts the htmx fragment endpoints of the module. It satisfies web.Module.
func (m *Module) Routes(r chi.Router) {
	m.handlers.Routes(r)
}

// Tabs returns the dock routes of the profiles and models screens. The composition root adds them to
// the dock registry.
func (m *Module) Tabs() []dock.Route {
	return m.handlers.Tabs()
}

// NewModule wires the adapters into the use cases. Every use case that mutates profiles.json
// shares one write guard, so concurrent mutations cannot lose each other's changes.
func NewModule(deps Deps) *Module {
	store := profiles.NewProfileStoreFileAdapter(deps.ProfilesDir)
	globalRouting := routing.NewGlobalRoutingFileAdapter(deps.ProfilesDir)
	exchangeAdapter := exchange.NewProfileExchangeJSONAdapter()
	agentRoles := roles.NewAgentRoleDirAdapter(deps.AgentsDir)
	guard := application.NewWriteGuard()
	module := &Module{
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
	module.handlers = inbound.NewHandlers(inbound.UseCases{
		ListProfiles:      module.ListProfiles,
		CreateProfile:     module.CreateProfile,
		DuplicateProfile:  module.DuplicateProfile,
		RenameProfile:     module.RenameProfile,
		DeleteProfile:     module.DeleteProfile,
		SetProfileRouting: module.SetProfileRouting,
		ExportProfile:     module.ExportProfile,
		ImportProfile:     module.ImportProfile,
		ListAgentRoles:    module.ListAgentRoles,
		CreateAgentRole:   module.CreateAgentRole,
		GetGlobalRouting:  module.GetGlobalRouting,
		SetGlobalRouting:  module.SetGlobalRouting,
		ListModelOptions:  module.ListModelOptions,
	})
	return module
}
