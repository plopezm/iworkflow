// Package di is the composition root of the jira module: it builds the adapters and the use cases.
// Handlers and routes come in phase 2.
package di

import (
	"context"
	"database/sql"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/modules/jira/application"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/jira/infrastructure/adapters/outbound/api"
	"iworkflow/internal/modules/jira/infrastructure/adapters/outbound/persistence"
)

// Deps are the resources the module needs from the composition root.
type Deps struct {
	DB *sql.DB
	// Tasks creates the iworkflow task of a conversion. The tasks module provides it; the composition
	// root adapts it. Required.
	Tasks outbound.TaskCreation
	// Workspaces lists the workspaces and branches a conversion can target. Required.
	Workspaces outbound.WorkspaceCatalog
	// Clock returns the current instant. Nil uses the system clock.
	Clock application.Clock
	// NewID creates the identifier of a new custom lane. Nil uses a random UUID.
	NewID func() string
}

// Module exposes the use cases of the jira module. Every field is ready to be called.
type Module struct {
	GetConnection          *application.GetConnection
	SaveConnection         *application.SaveConnection
	DeleteConnection       *application.DeleteConnection
	SyncIssues             *application.SyncIssues
	ListIssues             *application.ListIssues
	CreateLane             *application.CreateLane
	UpdateLane             *application.UpdateLane
	DeleteLane             *application.DeleteLane
	SetLaneCollapsed       *application.SetLaneCollapsed
	GetIssue               *application.GetIssue
	ListComments           *application.ListIssueComments
	ListTransitions        *application.ListIssueTransitions
	AddComment             *application.AddIssueComment
	TransitionIssue        *application.TransitionIssue
	SetIssueIgnored        *application.SetIssueIgnored
	PrepareConversion      *application.PrepareConversion
	ListConversionBranches *application.ListConversionBranches
	ConvertIssue           *application.ConvertIssue

	handlers *inbound.ConnectionHandlers
}

// Routes mounts the fragment endpoints of the module. It satisfies web.Module.
func (m *Module) Routes(r chi.Router) {
	m.handlers.Mount(r)
}

// Tabs returns the dock tabs of the module.
func (m *Module) Tabs() []dock.Route {
	return []dock.Route{
		{
			Name:    "jira",
			Pattern: "/jira",
			Icon:    "◈",
			Title:   func(_ context.Context, _ dock.Params) string { return "Jira" },
			Render: func(ctx context.Context, _ dock.Params) templ.Component {
				return m.handlers.Page(ctx)
			},
		},
	}
}

// NewModule wires the adapters into the use cases. It does not open the database: the caller owns it.
func NewModule(deps Deps) *Module {
	clock := deps.Clock
	if clock == nil {
		clock = application.SystemClock
	}
	newID := deps.NewID
	if newID == nil {
		newID = uuid.NewString
	}
	connections := persistence.NewConnectionRepository(deps.DB, clock)
	issues := persistence.NewIssueRepository(deps.DB)
	lanes := persistence.NewLaneRepository(deps.DB, clock, newID)
	client := api.NewJiraClient(clock)

	sync := application.NewSyncIssues(connections, issues, lanes, client, clock)
	getConnection := application.NewGetConnection(connections)
	saveConnection := application.NewSaveConnection(connections, issues, client)
	deleteConnection := application.NewDeleteConnection(connections, issues)

	return &Module{
		GetConnection:          getConnection,
		SaveConnection:         saveConnection,
		DeleteConnection:       deleteConnection,
		SyncIssues:             sync,
		ListIssues:             application.NewListIssues(issues, lanes),
		CreateLane:             application.NewCreateLane(lanes, sync),
		UpdateLane:             application.NewUpdateLane(lanes, sync),
		DeleteLane:             application.NewDeleteLane(lanes),
		SetLaneCollapsed:       application.NewSetLaneCollapsed(lanes),
		GetIssue:               application.NewGetIssue(issues),
		ListComments:           application.NewListIssueComments(connections, client),
		ListTransitions:        application.NewListIssueTransitions(connections, client),
		AddComment:             application.NewAddIssueComment(connections, client),
		TransitionIssue:        application.NewTransitionIssue(connections, issues, client, clock),
		SetIssueIgnored:        application.NewSetIssueIgnored(issues),
		PrepareConversion:      application.NewPrepareConversion(issues, deps.Workspaces),
		ListConversionBranches: application.NewListConversionBranches(deps.Workspaces),
		ConvertIssue:           application.NewConvertIssue(issues, deps.Tasks),
		handlers:               inbound.NewConnectionHandlers(getConnection, saveConnection, deleteConnection),
	}
}
