// Package di composes the workspace_groups module with constructor injection.
package di

import (
	"context"
	"database/sql"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/workspace_groups/application"
	"iworkflow/internal/modules/workspace_groups/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/workspace_groups/infrastructure/adapters/outbound/persistence"
	"iworkflow/internal/modules/workspace_groups/presentation/pages"
	"iworkflow/internal/modules/workspace_groups/presentation/state"
)

// Module exposes the use cases and mounts the endpoints.
type Module struct {
	Create   *application.CreateGroup
	Update   *application.UpdateGroup
	Delete   *application.DeleteGroup
	Collapse *application.SetGroupCollapsed
	Move     *application.MoveWorkspace
	Pin      *application.SetPinned
	Arrange  *application.ArrangeWorkspaces
	Assigned *application.ListAssignments

	handlers *inbound.Handlers
}

var _ web.Module = (*Module)(nil)

type uuidGenerator struct{}

func (uuidGenerator) NewID() string { return uuid.NewString() }

// NewModule builds the repository, the use cases and the handlers. The catalog lists the workspaces the group screens show.
func NewModule(db *sql.DB, catalog inbound.WorkspaceCatalog) *Module {
	repository := persistence.NewGroupRepository(db)
	module := &Module{
		Create:   application.NewCreateGroup(repository, uuidGenerator{}),
		Update:   application.NewUpdateGroup(repository),
		Delete:   application.NewDeleteGroup(repository),
		Collapse: application.NewSetGroupCollapsed(repository),
		Move:     application.NewMoveWorkspace(repository),
		Pin:      application.NewSetPinned(repository),
		Arrange:  application.NewArrangeWorkspaces(repository),
		Assigned: application.NewListAssignments(repository),
	}
	module.handlers = inbound.NewHandlers(inbound.UseCases{
		Create: module.Create, Update: module.Update, Delete: module.Delete,
		Collapse: module.Collapse, Move: module.Move, Pin: module.Pin,
		Assignments: module.Assigned, Catalog: catalog,
	})
	return module
}

// Routes mounts the endpoints.
func (m *Module) Routes(r chi.Router) { m.handlers.Mount(r) }

// Tabs returns the dock routes of the module: the groups list and the page of one group, both children of Workspaces.
func (m *Module) Tabs() []dock.Route {
	return []dock.Route{
		{
			Name:    "workspace_groups",
			Pattern: state.ListPath,
			Icon:    "▦",
			Title:   func(ctx context.Context, _ dock.Params) string { return pages.ListTitle(ctx) },
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return m.handlers.ListTab(ctx, params)
			},
		},
		{
			Name:    "workspace_group",
			Pattern: state.ListPath + "/{id}",
			Icon:    "▦",
			Title:   m.handlers.DetailTitle,
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return m.handlers.DetailTab(ctx, params)
			},
		},
	}
}
