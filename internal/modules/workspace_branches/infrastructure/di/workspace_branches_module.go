// Package di is the composition root of the workspace_branches module: it builds the git adapter, the use cases
// and the handlers, and exposes the routes and the dock tab the parent registers.
package di

import (
	"context"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/modules/workspace_branches/application"
	"iworkflow/internal/modules/workspace_branches/domain/ports/outbound"
	"iworkflow/internal/modules/workspace_branches/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/workspace_branches/infrastructure/adapters/outbound/git"
	"iworkflow/internal/modules/workspace_branches/presentation/components"
)

// TabPattern is the dock address of the branch tab of a workspace.
const TabPattern = "/workspaces/{id}/branches"

// Deps are the ports the module needs from the composition root. Both are bound there, so the module never
// imports the workspaces or the tasks module.
type Deps struct {
	// Workspaces gives the path and the base branch of a workspace.
	Workspaces outbound.WorkspaceResolver
	// Tasks gives the branches of the live tasks of a workspace.
	Tasks outbound.ProtectedBranches
}

// Module exposes the use cases, the routes and the dock tab of the workspace_branches module.
type Module struct {
	List     *application.ListBranches
	Delete   *application.DeleteBranches
	handlers *inbound.Handlers
}

// NewModule wires the git adapter into the use cases and the use cases into the handlers.
func NewModule(deps Deps) *Module {
	gitAdapter := git.NewAdapter()
	list := application.NewListBranches(deps.Workspaces, gitAdapter, deps.Tasks)
	deleteBranches := application.NewDeleteBranches(deps.Workspaces, gitAdapter, deps.Tasks)
	return &Module{
		List:     list,
		Delete:   deleteBranches,
		handlers: inbound.NewHandlers(list, deleteBranches),
	}
}

// Routes mounts the htmx endpoints of the module.
func (m *Module) Routes(r chi.Router) {
	m.handlers.Mount(r)
}

// Tabs returns the dock route of the branch tab of one workspace.
func (m *Module) Tabs() []dock.Route {
	return []dock.Route{
		{
			Name:    "workspace_branches",
			Pattern: TabPattern,
			Icon:    "⎇",
			Title: func(ctx context.Context, _ dock.Params) string {
				return components.TabTitle(ctx)
			},
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return m.handlers.ListTab(ctx, params)
			},
		},
	}
}
