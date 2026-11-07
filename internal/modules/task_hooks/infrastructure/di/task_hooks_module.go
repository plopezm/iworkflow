// Package di wires the task_hooks module: the adapters of its outbound ports and the use cases built over them.
package di

import (
	"context"
	"database/sql"
	"fmt"

	"iworkflow/internal/modules/task_hooks/application"
	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/outbound/persistence"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/outbound/process"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/outbound/system"
)

// Deps are the inputs the composition root gives the module.
type Deps struct {
	DB *sql.DB
	// Clock gives the times of the run records and of the reconciliation.
	Clock outbound.Clock
	// AgentSession runs the agent hooks. Nil means no agent session is configured: every agent hook then fails
	// to start with hookerrors.ErrRunNotStarted, and the script hooks are not affected.
	AgentSession outbound.AgentSession
}

// Module exposes the use cases of the task_hooks module. The composition root calls them; the module mounts no
// routes, the editor and the task status strip are added with their handlers.
type Module struct {
	RunHooks           *application.RunHooks
	ListHooks          *application.ListHooks
	SaveHook           *application.SaveHook
	DeleteHook         *application.DeleteHook
	ReorderHooks       *application.ReorderHooks
	SetTaskHookSkipped *application.SetTaskHookSkipped
	ListTaskHookState  *application.ListTaskHookState
	ReconcileHookRuns  *application.ReconcileHookRuns
}

// NewModule builds the adapters and the use cases over the database and the deps.
func NewModule(deps Deps) *Module {
	hooks := persistence.NewHookRepository(deps.DB)
	runs := persistence.NewRunRepository(deps.DB)
	agents := deps.AgentSession
	if agents == nil {
		agents = unavailableAgentSession{}
	}
	ids := system.NewUUIDGenerator()
	return &Module{
		RunHooks: application.NewRunHooks(hooks, hooks, runs, process.NewScriptRunner(),
			application.NewAgentHookRunner(agents), deps.Clock, ids),
		ListHooks:          application.NewListHooks(hooks),
		SaveHook:           application.NewSaveHook(hooks, ids),
		DeleteHook:         application.NewDeleteHook(hooks),
		ReorderHooks:       application.NewReorderHooks(hooks),
		SetTaskHookSkipped: application.NewSetTaskHookSkipped(hooks),
		ListTaskHookState:  application.NewListTaskHookState(hooks, hooks, runs),
		ReconcileHookRuns:  application.NewReconcileHookRuns(runs, deps.Clock),
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
