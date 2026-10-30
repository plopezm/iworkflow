//nolint:wrapcheck // Port errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"log/slog"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// DetachTaskFromGraph returns a task to the loose tasks: it deletes every edge of the task (in both
// directions) and its position. Successors left without predecessors become normal tasks and never start by
// themselves; the ones that keep only merged predecessors may start through the trigger, whose failure is only
// logged. Detaching a task that is not in the graph is not an error.
type DetachTaskFromGraph struct {
	graph   outbound.TaskGraphRepository
	trigger inbound.SuccessorTrigger
}

// NewDetachTaskFromGraph builds the use case.
func NewDetachTaskFromGraph(graph outbound.TaskGraphRepository, trigger inbound.SuccessorTrigger) *DetachTaskFromGraph {
	return &DetachTaskFromGraph{graph: graph, trigger: trigger}
}

// Execute detaches the task and evaluates the graph. A repository failure is returned without evaluating it.
func (u *DetachTaskFromGraph) Execute(ctx context.Context, taskID models.TaskID) error {
	if err := u.graph.DetachTask(ctx, taskID); err != nil {
		return err
	}
	if _, err := u.trigger.Trigger(ctx); err != nil {
		slog.Warn("could not evaluate the graph after detaching the task", "task_id", taskID.String(), "error", err)
	}
	return nil
}
