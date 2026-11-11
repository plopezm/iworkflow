// Package di wires the task_hooks module: the adapters of its outbound ports and the use cases built over them.
package di

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/modules/task_hooks/application"
	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/outbound/persistence"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/outbound/process"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/outbound/system"
	hookcomponents "iworkflow/internal/modules/task_hooks/presentation/components"
	"iworkflow/internal/modules/task_hooks/presentation/state"
)

// Deps are the inputs the composition root gives the module.
type Deps struct {
	DB *sql.DB
	// Clock gives the times of the run records and of the reconciliation.
	Clock outbound.Clock
	// AgentSession runs the agent hooks. Nil means no agent session is configured: every agent hook then fails
	// to start with hookerrors.ErrRunNotStarted, and the script hooks are not affected.
	AgentSession outbound.AgentSession
	// Workspaces gives the working directory of a workspace, where a test of a hook runs. Nil means no lookup is
	// configured: a test then fails with hookerrors.ErrRunNotStarted, and the rest of the module is not affected.
	Workspaces outbound.WorkspaceLookup
	// Projects lists the projects a hook can be assigned to. Nil means no project source is configured: the screen
	// then shows no project and reports why.
	Projects application.WorkspaceLister
}

// Module exposes the use cases of the task_hooks module and its web surface: the Hooks screen, a dock tab mounted by the
// composition root through Routes and Tabs. The use cases are shared with the handlers, so the screen and the composition
// run the same code. The lifecycle hooks and the task status strip are bound with their own adapters.
type Module struct {
	ListHooks                *application.ListHooks
	SaveHook                 *application.SaveHook
	DeleteHook               *application.DeleteHook
	SaveAssignment           *application.SaveAssignment
	DeleteAssignment         *application.DeleteAssignment
	ReorderAssignments       *application.ReorderAssignments
	ListWorkspaceAssignments *application.ListWorkspaceAssignments
	ListRuns                 *application.ListRuns
	ListProjects             *application.ListProjects
	TestHook                 *application.TestHook
	BrowsePath               *application.BrowsePath
	RunHooks                 *application.RunHooks
	ReconcileHookRuns        *application.ReconcileHookRuns
	SetTaskHookSkipped       *application.SetTaskHookSkipped
	ListTaskHookState        *application.ListTaskHookState
	handlers                 *inbound.Handlers
}

// NewModule builds the adapters and the use cases over the database and the deps.
func NewModule(deps Deps) *Module {
	hooks := persistence.NewHookRepository(deps.DB)
	assignments := persistence.NewAssignmentRepository(deps.DB)
	runs := persistence.NewRunRepository(deps.DB)
	agents := deps.AgentSession
	if agents == nil {
		agents = unavailableAgentSession{}
	}
	workspaces := deps.Workspaces
	if workspaces == nil {
		workspaces = unavailableWorkspaces{}
	}
	projects := deps.Projects
	if projects == nil {
		projects = unavailableProjects{}
	}
	ids := system.NewUUIDGenerator()
	scripts := process.NewScriptRunner()
	agentRunner := application.NewAgentHookRunner(agents)
	module := &Module{
		ListHooks:                application.NewListHooks(hooks, assignments, runs),
		SaveHook:                 application.NewSaveHook(hooks, ids),
		DeleteHook:               application.NewDeleteHook(hooks),
		SaveAssignment:           application.NewSaveAssignment(hooks, assignments, ids),
		DeleteAssignment:         application.NewDeleteAssignment(assignments),
		ReorderAssignments:       application.NewReorderAssignments(assignments),
		ListWorkspaceAssignments: application.NewListWorkspaceAssignments(assignments),
		ListRuns:                 application.NewListRuns(runs),
		ListProjects:             application.NewListProjects(projects),
		TestHook:                 application.NewTestHook(hooks, workspaces, scripts, agentRunner),
		BrowsePath:               application.NewBrowsePath(workspaces, system.NewPathBrowser()),
		RunHooks:                 application.NewRunHooks(assignments, hooks, runs, scripts, agentRunner, deps.Clock, ids),
		ReconcileHookRuns:        application.NewReconcileHookRuns(runs, deps.Clock),
		SetTaskHookSkipped:       application.NewSetTaskHookSkipped(assignments, hooks),
		ListTaskHookState:        application.NewListTaskHookState(assignments, hooks, runs),
	}
	module.handlers = inbound.NewHandlers(inbound.UseCases{
		List:             module.ListHooks,
		Projects:         module.ListProjects,
		Save:             module.SaveHook,
		Delete:           module.DeleteHook,
		SaveAssignment:   module.SaveAssignment,
		DeleteAssignment: module.DeleteAssignment,
		Reorder:          module.ReorderAssignments,
		Assignments:      module.ListWorkspaceAssignments,
		Runs:             module.ListRuns,
		Test:             module.TestHook,
		Browse:           module.BrowsePath,
	})
	return module
}

// Routes mounts the htmx endpoints of the Hooks screen.
func (m *Module) Routes(r chi.Router) {
	m.handlers.Mount(r)
}

// Tabs returns the dock route of the Hooks screen, a top-level section. The parent registers it in the dock registry.
func (m *Module) Tabs() []dock.Route {
	return []dock.Route{
		{
			Name:    "hooks",
			Pattern: state.ScreenPath,
			Icon:    "⚓",
			Title: func(ctx context.Context, _ dock.Params) string {
				return hookcomponents.TabTitle(ctx)
			},
			Render: func(ctx context.Context, _ dock.Params) templ.Component {
				return m.handlers.Page(ctx)
			},
		},
	}
}

// unavailableAgentSession stands in when the composition root configures no agent session. It refuses every
// session, so an agent hook records an error instead of running anything.
type unavailableAgentSession struct{}

var _ outbound.AgentSession = unavailableAgentSession{}

// Run refuses the session: there is no agent to run it with.
func (unavailableAgentSession) Run(context.Context, outbound.AgentSessionRequest) (outbound.AgentSessionResult, error) {
	return outbound.AgentSessionResult{}, fmt.Errorf("%w: no agent session is configured", hookerrors.ErrRunNotStarted)
}

// unavailableWorkspaces stands in when the composition root configures no workspace lookup. It refuses every lookup,
// so a test records no run and says why.
type unavailableWorkspaces struct{}

var _ outbound.WorkspaceLookup = unavailableWorkspaces{}

// Path refuses the lookup: there is no workspace source to find the directory with.
func (unavailableWorkspaces) Path(context.Context, string) (string, error) {
	return "", fmt.Errorf("%w: no workspace lookup is configured", hookerrors.ErrRunNotStarted)
}

// unavailableProjects stands in when the composition root configures no project source. The screen shows the failure.
type unavailableProjects struct{}

var _ application.WorkspaceLister = unavailableProjects{}

// Projects refuses the listing: there is no project source to list from.
func (unavailableProjects) Projects(context.Context) ([]application.ProjectOption, error) {
	return nil, fmt.Errorf("%w: no project source is configured", hookerrors.ErrRunNotStarted)
}
