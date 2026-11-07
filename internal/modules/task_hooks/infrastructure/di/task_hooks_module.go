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
}

// Module exposes the use cases of the task_hooks module and its web surface: the Hooks tab of a workspace, mounted by
// the composition root through Routes and Tabs. The task status strip is added with its own handlers.
type Module struct {
	RunHooks           *application.RunHooks
	ListHooks          *application.ListHooks
	SaveHook           *application.SaveHook
	DeleteHook         *application.DeleteHook
	ReorderHooks       *application.ReorderHooks
	SetHookEnabled     *application.SetHookEnabled
	TestHook           *application.TestHook
	SetTaskHookSkipped *application.SetTaskHookSkipped
	ListTaskHookState  *application.ListTaskHookState
	ReconcileHookRuns  *application.ReconcileHookRuns
	handlers           *inbound.Handlers
}

// NewModule builds the adapters and the use cases over the database and the deps.
func NewModule(deps Deps) *Module {
	hooks := persistence.NewHookRepository(deps.DB)
	runs := persistence.NewRunRepository(deps.DB)
	agents := deps.AgentSession
	if agents == nil {
		agents = unavailableAgentSession{}
	}
	workspaces := deps.Workspaces
	if workspaces == nil {
		workspaces = unavailableWorkspaces{}
	}
	ids := system.NewUUIDGenerator()
	scripts := process.NewScriptRunner()
	agentRunner := application.NewAgentHookRunner(agents)
	module := &Module{
		RunHooks:           application.NewRunHooks(hooks, hooks, runs, scripts, agentRunner, deps.Clock, ids),
		ListHooks:          application.NewListHooks(hooks),
		SaveHook:           application.NewSaveHook(hooks, ids),
		DeleteHook:         application.NewDeleteHook(hooks),
		ReorderHooks:       application.NewReorderHooks(hooks),
		SetHookEnabled:     application.NewSetHookEnabled(hooks),
		TestHook:           application.NewTestHook(hooks, workspaces, scripts, agentRunner),
		SetTaskHookSkipped: application.NewSetTaskHookSkipped(hooks),
		ListTaskHookState:  application.NewListTaskHookState(hooks, hooks, runs),
		ReconcileHookRuns:  application.NewReconcileHookRuns(runs, deps.Clock),
	}
	module.handlers = inbound.NewHandlers(inbound.UseCases{
		List:       module.ListHooks,
		Save:       module.SaveHook,
		Delete:     module.DeleteHook,
		Reorder:    module.ReorderHooks,
		SetEnabled: module.SetHookEnabled,
		Test:       module.TestHook,
	})
	return module
}

// Routes mounts the htmx endpoints of the Hooks tab.
func (m *Module) Routes(r chi.Router) {
	m.handlers.Mount(r)
}

// Tabs returns the dock route of the Hooks tab of a workspace. The parent registers it in the dock registry.
func (m *Module) Tabs() []dock.Route {
	return []dock.Route{
		{
			Name:    "workspace_hooks",
			Pattern: "/workspaces/{id}/hooks",
			Icon:    "⚓",
			Title: func(ctx context.Context, _ dock.Params) string {
				return hookcomponents.TabTitle(ctx)
			},
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return m.handlers.Tab(ctx, params)
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
