//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"log/slog"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// AddTaskDependency makes successor wait for predecessor. Both tasks must belong to the same workspace, the
// successor must still be in backlog and the predecessor must not be discarded (it would never be merged).
// When the predecessor already meets the trigger, the successor starts at once.
type AddTaskDependency struct {
	lifecycle *TaskLifecycle
	graph     outbound.TaskGraphRepository
	trigger   inbound.SuccessorTrigger
}

// NewAddTaskDependency builds the use case.
func NewAddTaskDependency(lifecycle *TaskLifecycle, graph outbound.TaskGraphRepository, trigger inbound.SuccessorTrigger) *AddTaskDependency {
	return &AddTaskDependency{lifecycle: lifecycle, graph: graph, trigger: trigger}
}

// Execute stores the edge and evaluates the graph. A failed evaluation is logged and does not fail the add.
func (u *AddTaskDependency) Execute(ctx context.Context, predecessor models.TaskID, successor models.TaskID) (models.TaskDependency, error) {
	if predecessor == successor {
		return models.TaskDependency{}, werrors.SelfDependency()
	}
	from, err := u.lifecycle.Load(ctx, predecessor)
	if err != nil {
		return models.TaskDependency{}, err
	}
	to, err := u.lifecycle.Load(ctx, successor)
	if err != nil {
		return models.TaskDependency{}, err
	}
	if from.WorkspaceID != to.WorkspaceID {
		return models.TaskDependency{}, werrors.CrossWorkspaceDependency()
	}
	if from.Status == models.StatusDiscarded {
		return models.TaskDependency{}, invalidState("depender de", from.Status)
	}
	if to.Status != models.StatusBacklog {
		return models.TaskDependency{}, invalidState("añadir una dependencia a", to.Status)
	}
	dependency := models.TaskDependency{
		Predecessor: predecessor,
		Successor:   successor,
		WorkspaceID: to.WorkspaceID,
		Trigger:     models.DependencyTriggerMerged,
	}
	if err := u.graph.InsertDependency(ctx, dependency); err != nil {
		return models.TaskDependency{}, err
	}
	if _, err := u.trigger.Trigger(ctx); err != nil {
		slog.Warn("could not evaluate the graph after adding the dependency", "successor_id", successor.String(), "error", err)
	}
	return dependency, nil
}
