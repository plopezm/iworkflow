//nolint:wrapcheck // Port errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"log/slog"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// RemoveTaskDependency removes an edge. When it was the edge that blocked the successor, the successor can
// start now. Removing an edge that does not exist is not an error.
type RemoveTaskDependency struct {
	graph   outbound.TaskGraphRepository
	trigger inbound.SuccessorTrigger
}

// NewRemoveTaskDependency builds the use case.
func NewRemoveTaskDependency(graph outbound.TaskGraphRepository, trigger inbound.SuccessorTrigger) *RemoveTaskDependency {
	return &RemoveTaskDependency{graph: graph, trigger: trigger}
}

// Execute removes the edge and, only when one was removed, evaluates the graph again.
func (u *RemoveTaskDependency) Execute(ctx context.Context, predecessor models.TaskID, successor models.TaskID) error {
	removed, err := u.graph.RemoveDependency(ctx, predecessor, successor)
	if err != nil {
		return err
	}
	if !removed {
		return nil
	}
	if _, err := u.trigger.Trigger(ctx); err != nil {
		slog.Warn("could not evaluate the graph after removing the dependency", "successor_id", successor.String(), "error", err)
	}
	return nil
}
