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
	"iworkflow/internal/modules/jira/presentation/pages"
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

	handlers      *inbound.ConnectionHandlers
	boardHandlers *inbound.BoardHandlers
	issueHandlers *inbound.IssueHandlers
}

// Routes mounts the fragment endpoints of the module. It satisfies web.Module.
func (m *Module) Routes(r chi.Router) {
	m.handlers.Mount(r)
	m.boardHandlers.Mount(r)
	m.issueHandlers.Mount(r)
}

// SettingsContent returns the settings card component for embedding into the settings screen.
func (m *Module) SettingsContent() func(context.Context) templ.Component {
	return m.handlers.SettingsContent()
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
		{
			Name:    "jira_convert",
			Pattern: "/jira/{key}/convert",
			Icon:    "◈",
			Title: func(ctx context.Context, params dock.Params) string {
				return pages.FormatConvertTitle(ctx, params["key"])
			},
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return m.issueHandlers.ConvertPage(ctx, params["key"])
			},
		},
		{
			Name:    "jira_issue",
			Pattern: "/jira/{key}",
			Icon:    "◈",
			Title: func(_ context.Context, params dock.Params) string {
				return params["key"]
			},
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return m.issueHandlers.IssuePage(ctx, params["key"])
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

	listIssues := application.NewListIssues(issues, lanes)
	createLane := application.NewCreateLane(lanes, sync)
	updateLane := application.NewUpdateLane(lanes, sync)
	deleteLane := application.NewDeleteLane(lanes)
	setLaneCollapsed := application.NewSetLaneCollapsed(lanes)
	setIssueIgnored := application.NewSetIssueIgnored(issues)

	getIssue := application.NewGetIssue(issues)
	listComments := application.NewListIssueComments(connections, client)
	listTransitions := application.NewListIssueTransitions(connections, client)
	addComment := application.NewAddIssueComment(connections, client)
	transitionIssue := application.NewTransitionIssue(connections, issues, client, clock)
	prepareConversion := application.NewPrepareConversion(issues, deps.Workspaces)
	listConversionBranches := application.NewListConversionBranches(deps.Workspaces)
	convertIssue := application.NewConvertIssue(issues, deps.Tasks)

	connectionHandlers := inbound.NewConnectionHandlers(getConnection, saveConnection, deleteConnection)
	boardHandlers := inbound.NewBoardHandlers(
		getConnection,
		sync,
		listIssues,
		createLane,
		updateLane,
		deleteLane,
		setLaneCollapsed,
		setIssueIgnored,
	)
	connectionHandlers.SetBoardHandlers(boardHandlers)
	issueHandlers := inbound.NewIssueHandlers(
		getIssue,
		listComments,
		listTransitions,
		addComment,
		transitionIssue,
		setIssueIgnored,
		prepareConversion,
		listConversionBranches,
		convertIssue,
	)

	return &Module{
		GetConnection:          getConnection,
		SaveConnection:         saveConnection,
		DeleteConnection:       deleteConnection,
		SyncIssues:             sync,
		ListIssues:             listIssues,
		CreateLane:             createLane,
		UpdateLane:             updateLane,
		DeleteLane:             deleteLane,
		SetLaneCollapsed:       setLaneCollapsed,
		GetIssue:               getIssue,
		ListComments:           listComments,
		ListTransitions:        listTransitions,
		AddComment:             addComment,
		TransitionIssue:        transitionIssue,
		SetIssueIgnored:        setIssueIgnored,
		PrepareConversion:      prepareConversion,
		ListConversionBranches: listConversionBranches,
		ConvertIssue:           convertIssue,
		handlers:               connectionHandlers,
		boardHandlers:          boardHandlers,
		issueHandlers:          issueHandlers,
	}
}
