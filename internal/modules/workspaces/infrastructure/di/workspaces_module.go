// Package di is the composition root of the workspaces module: it builds the adapters and the use
// cases. The routes are added in a later phase; until then the module only exposes use cases.
package di

import (
	"database/sql"
	"time"

	"iworkflow/internal/modules/workspaces/application"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
	"iworkflow/internal/modules/workspaces/infrastructure/adapters/outbound/fs"
	"iworkflow/internal/modules/workspaces/infrastructure/adapters/outbound/git"
	"iworkflow/internal/modules/workspaces/infrastructure/adapters/outbound/persistence"
)

// Deps are the resources the module needs from the composition root.
type Deps struct {
	DB *sql.DB
	// HomeDir is the folder the directory browser starts in, and the only folder it may create into.
	HomeDir string
	// Tasks deletes the tasks of a workspace; it is provided by the tasks module.
	Tasks outbound.WorkspaceTasks
	// AgentsFiles gives access to the stored AGENTS.md files; it is provided by the agents_files module.
	AgentsFiles outbound.AgentsFiles
}

// Module exposes the use cases of the workspaces module.
type Module struct {
	Register          *application.RegisterWorkspace
	List              *application.ListWorkspaces
	Get               *application.GetWorkspace
	Update            *application.UpdateWorkspace
	Delete            *application.DeleteWorkspace
	BrowseDirectories *application.BrowseDirectories
	CreateDirectory   *application.CreateDirectory
	ListAgentsFiles   *application.ListAgentsFileOptions
}

// NewModule wires the adapters into the use cases.
func NewModule(deps Deps) *Module {
	repository := persistence.NewWorkspaceRepository(deps.DB)
	gitRepository := git.NewRepositoryAdapter()
	browser := fs.NewDirectoryBrowser(deps.HomeDir)
	clock := application.Clock(func() time.Time { return time.Now().UTC() })
	return &Module{
		Register:          application.NewRegisterWorkspace(repository, gitRepository, deps.AgentsFiles, clock, application.IDGenerator(models.NewWorkspaceID)),
		List:              application.NewListWorkspaces(repository),
		Get:               application.NewGetWorkspace(repository),
		Update:            application.NewUpdateWorkspace(repository, gitRepository, deps.AgentsFiles),
		Delete:            application.NewDeleteWorkspace(repository, deps.Tasks),
		BrowseDirectories: application.NewBrowseDirectories(browser, deps.HomeDir),
		CreateDirectory:   application.NewCreateDirectory(browser),
		ListAgentsFiles:   application.NewListAgentsFileOptions(deps.AgentsFiles),
	}
}
