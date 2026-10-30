// Package di is the composition root of the workspaces module: it builds the adapters, the use cases
// and the HTTP handlers, and exposes the routes and the dock tabs the parent registers.
package di

import (
	"context"
	"database/sql"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/modules/workspaces/application"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
	"iworkflow/internal/modules/workspaces/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/workspaces/infrastructure/adapters/outbound/fs"
	"iworkflow/internal/modules/workspaces/infrastructure/adapters/outbound/git"
	"iworkflow/internal/modules/workspaces/infrastructure/adapters/outbound/persistence"
	"iworkflow/internal/modules/workspaces/presentation/pages"
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

// Module exposes the use cases, the routes and the dock tabs of the workspaces module.
type Module struct {
	Register          *application.RegisterWorkspace
	List              *application.ListWorkspaces
	Get               *application.GetWorkspace
	Update            *application.UpdateWorkspace
	Delete            *application.DeleteWorkspace
	BrowseDirectories *application.BrowseDirectories
	CreateDirectory   *application.CreateDirectory
	ListAgentsFiles   *application.ListAgentsFileOptions
	handlers          *inbound.Handlers
}

// NewModule wires the adapters into the use cases and the use cases into the handlers.
func NewModule(deps Deps) *Module {
	repository := persistence.NewWorkspaceRepository(deps.DB)
	gitRepository := git.NewRepositoryAdapter()
	browser := fs.NewDirectoryBrowser(deps.HomeDir)
	clock := application.Clock(func() time.Time { return time.Now().UTC() })
	module := &Module{
		Register:          application.NewRegisterWorkspace(repository, gitRepository, deps.AgentsFiles, clock, application.IDGenerator(models.NewWorkspaceID)),
		List:              application.NewListWorkspaces(repository),
		Get:               application.NewGetWorkspace(repository),
		Update:            application.NewUpdateWorkspace(repository, gitRepository, deps.AgentsFiles),
		Delete:            application.NewDeleteWorkspace(repository, deps.Tasks),
		BrowseDirectories: application.NewBrowseDirectories(browser, deps.HomeDir),
		CreateDirectory:   application.NewCreateDirectory(browser),
		ListAgentsFiles:   application.NewListAgentsFileOptions(deps.AgentsFiles),
	}
	module.handlers = inbound.NewHandlers(inbound.UseCases{
		Register:        module.Register,
		List:            module.List,
		Get:             module.Get,
		Update:          module.Update,
		Delete:          module.Delete,
		BrowseDirectory: module.BrowseDirectories,
		CreateDirectory: module.CreateDirectory,
		ListAgentsFiles: module.ListAgentsFiles,
	})
	return module
}

// Routes mounts the htmx endpoints of the module.
func (m *Module) Routes(r chi.Router) {
	m.handlers.Mount(r)
}

// Tabs returns the dock routes of the module: the workspaces list and the edit page of one workspace.
// The parent registers them in the dock registry.
func (m *Module) Tabs() []dock.Route {
	return []dock.Route{
		{
			Name:    "workspaces",
			Pattern: "/workspaces",
			Icon:    "▤",
			Title: func(ctx context.Context, _ dock.Params) string {
				return pages.ListTitle(ctx)
			},
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return m.handlers.ListTab(ctx, params)
			},
		},
		{
			Name:    "workspace_edit",
			Pattern: "/workspaces/{id}/edit",
			Icon:    "✎",
			Title: func(ctx context.Context, _ dock.Params) string {
				return pages.EditTitle(ctx)
			},
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return m.handlers.EditTab(ctx, params)
			},
		},
	}
}
