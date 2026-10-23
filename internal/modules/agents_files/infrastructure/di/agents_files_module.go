// Package di is the composition root of the agents_files module: it builds the adapters, the use
// cases and the handlers, and exposes the routes and the dock tab the parent registers.
package di

import (
	"database/sql"
	"log/slog"
	"time"

	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/modules/agents_files/application"
	"iworkflow/internal/modules/agents_files/domain/models"
	"iworkflow/internal/modules/agents_files/domain/ports/outbound"
	"iworkflow/internal/modules/agents_files/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/agents_files/infrastructure/adapters/outbound/fs"
	"iworkflow/internal/modules/agents_files/infrastructure/adapters/outbound/git"
	"iworkflow/internal/modules/agents_files/infrastructure/adapters/outbound/persistence"
)

// Deps are the resources the module needs from the composition root.
type Deps struct {
	DB *sql.DB
	// StorageDir is the folder where the AGENTS.md contents are stored.
	StorageDir string
	// Projects is provided by the workspaces module.
	Projects outbound.ProjectLookup
	Logger   *slog.Logger
}

// Module exposes the use cases of the agents_files module, its routes and its dock tab.
type Module struct {
	Upload       *application.UploadAgentsFile
	List         *application.ListAgentsFiles
	Delete       *application.DeleteAgentsFile
	Install      *application.InstallAgentsFile
	Apply        *application.ApplyAgentsFileToProject
	ListProjects *application.ListProjects

	handlers *inbound.Handlers
}

// NewModule wires the adapters into the use cases and the use cases into the handlers.
func NewModule(deps Deps) *Module {
	repository := persistence.NewAgentsFileRepository(deps.DB)
	storage := fs.NewAgentsFileStorage(deps.StorageDir)
	installer := git.NewAgentsFileInstaller(deps.Logger)
	clock := application.Clock(func() time.Time { return time.Now().UTC() })
	install := application.NewInstallAgentsFile(repository, storage, installer)
	module := &Module{
		Upload:       application.NewUploadAgentsFile(repository, storage, clock, application.IDGenerator(models.NewAgentsFileID), deps.Logger),
		List:         application.NewListAgentsFiles(repository),
		Delete:       application.NewDeleteAgentsFile(repository, storage),
		Install:      install,
		Apply:        application.NewApplyAgentsFileToProject(deps.Projects, install),
		ListProjects: application.NewListProjects(deps.Projects),
	}
	module.handlers = inbound.NewHandlers(module.List, module.ListProjects, module.Upload, module.Delete, module.Apply)
	return module
}

// Routes mounts the fragment endpoints of the screen. It satisfies web.Module.
func (m *Module) Routes(r chi.Router) {
	m.handlers.Routes(r)
}

// Tabs returns the dock routes of the module: the AGENTS.md tab. The composition root adds them to
// the dock registry.
func (m *Module) Tabs() []dock.Route {
	return []dock.Route{m.handlers.Tab()}
}
