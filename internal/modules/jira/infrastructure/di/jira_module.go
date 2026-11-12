// Package di is the composition root of the jira module: it builds the adapters and the use cases.
// Handlers and routes come in phase 2.
package di

import (
	"context"
	"database/sql"
	"errors"
	"io"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/i18n"
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
	// TaskInfo reads the task a worklog is proposed from. The composition root adapts the tasks module. Nil leaves the
	// worklog proposal unavailable: the panel shows the error instead of failing.
	TaskInfo outbound.TaskInfo
}

// Module exposes the use cases of the jira module. Every field is ready to be called.
type Module struct {
	GetConnection          *application.GetConnection
	SaveConnection         *application.SaveConnection
	DeleteConnection       *application.DeleteConnection
	SyncIssues             *application.SyncIssues
	ListIssues             *application.ListIssues
	ListEpics              *application.ListEpics
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
	GetTaskIssue           *application.GetTaskIssue
	LinkTaskToIssue        *application.LinkTaskToIssue
	RefreshIssue           *application.RefreshIssue
	GetWorklogProposal     *application.GetWorklogProposal
	RegisterWorklog        *application.RegisterWorklog
	ListTaskWorklogs       *application.ListTaskWorklogs

	handlers      *inbound.ConnectionHandlers
	boardHandlers *inbound.BoardHandlers
	issueHandlers *inbound.IssueHandlers
	taskPanels    *inbound.TaskPanelHandlers
}

// Routes mounts the fragment endpoints of the module. It satisfies web.Module.
func (m *Module) Routes(r chi.Router) {
	m.handlers.Mount(r)
	m.boardHandlers.Mount(r)
	m.issueHandlers.Mount(r)
	m.taskPanels.Mount(r)
}

// msgPanelFailed is the panel of a task whose Jira configuration could not be read. The text is generic: the error
// itself is internal and is not shown.
var msgPanelFailed = i18n.Msg{
	En: "The Jira connection could not be read.",
	Es: "No se pudo leer la conexión con Jira.",
	Zh: "无法读取 Jira 连接。",
	De: "Die Jira-Verbindung konnte nicht gelesen werden.",
	Fr: "La connexion à Jira n'a pas pu être lue.",
}

// TaskPanel gives the Jira panel of a task. The panel is offered when a Jira connection is configured, because any task
// can be linked to an issue by its key. The component reads its data when it renders.
func (m *Module) TaskPanel(ctx context.Context, taskID string) (bool, templ.Component) {
	connection, err := m.GetConnection.Execute(ctx)
	if err != nil {
		return true, templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			return shared.ErrorText(msgPanelFailed.T(ctx)).Render(ctx, w)
		})
	}
	if connection == nil {
		return false, nil
	}
	return true, m.taskPanels.Body(taskID)
}

// unwiredTaskInfo answers a worklog proposal that has no task to read: the composition root did not provide one.
type unwiredTaskInfo struct{}

func (unwiredTaskInfo) Info(context.Context, string) (outbound.TaskSnapshot, error) {
	return outbound.TaskSnapshot{}, errors.New("la información de la tarea no está disponible")
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
	worklogs := persistence.NewWorklogRepository(deps.DB)
	client := api.NewJiraClient(clock)
	taskInfo := deps.TaskInfo
	if taskInfo == nil {
		taskInfo = unwiredTaskInfo{}
	}

	sync := application.NewSyncIssues(connections, issues, lanes, client, clock)
	getConnection := application.NewGetConnection(connections)
	saveConnection := application.NewSaveConnection(connections, issues, client)
	deleteConnection := application.NewDeleteConnection(connections, issues)

	listIssues := application.NewListIssues(issues, lanes)
	listEpics := application.NewListEpics(issues)
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

	getTaskIssue := application.NewGetTaskIssue(issues)
	linkTask := application.NewLinkTaskToIssue(issues, connections, client, clock)
	refreshIssue := application.NewRefreshIssue(connections, issues, client, clock)
	registerWorklog := application.NewRegisterWorklog(issues, connections, client, worklogs, clock)
	listTaskWorklogs := application.NewListTaskWorklogs(worklogs)
	getWorklogProposal := application.NewGetWorklogProposal(taskInfo, listTaskWorklogs)

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
		listEpics,
		listTransitions,
		transitionIssue,
	)
	connectionHandlers.SetBoardHandlers(boardHandlers)
	taskPanels := inbound.NewTaskPanelHandlers(inbound.TaskPanelUseCases{
		GetTaskIssue:    getTaskIssue,
		LinkTask:        linkTask,
		RefreshIssue:    refreshIssue,
		ListComments:    listComments,
		ListTransitions: listTransitions,
		TransitionIssue: transitionIssue,
		Proposal:        getWorklogProposal,
		ListWorklogs:    listTaskWorklogs,
		RegisterWorklog: registerWorklog,
	})
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
		ListEpics:              listEpics,
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
		GetTaskIssue:           getTaskIssue,
		LinkTaskToIssue:        linkTask,
		RefreshIssue:           refreshIssue,
		GetWorklogProposal:     getWorklogProposal,
		RegisterWorklog:        registerWorklog,
		ListTaskWorklogs:       listTaskWorklogs,
		handlers:               connectionHandlers,
		boardHandlers:          boardHandlers,
		issueHandlers:          issueHandlers,
		taskPanels:             taskPanels,
	}
}
