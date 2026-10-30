package outbound

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
)

// TaskGraphRepository persists the dependency edges and the node positions of the board. Declared for slice C.
type TaskGraphRepository interface {
	// InsertDependency stores the edge after validating it against the workspace edges (no self edge,
	// no duplicate, no cycle), in one transaction, so two concurrent inserts cannot close a cycle.
	InsertDependency(ctx context.Context, dependency models.TaskDependency) error
	// RemoveDependency deletes the edge; it returns false when the edge did not exist.
	RemoveDependency(ctx context.Context, predecessor models.TaskID, successor models.TaskID) (bool, error)
	// ListDependencies returns the edges of a workspace, or of every workspace when nil.
	ListDependencies(ctx context.Context, workspaceID *string) ([]models.TaskDependency, error)
	// SetPosition stores (or replaces) the position of a task node. It does not fail when the task is gone.
	SetPosition(ctx context.Context, taskID models.TaskID, position models.NodePosition) error
	// DetachTask deletes every edge of the task (as predecessor and as successor) and its position in one
	// transaction. It does not fail when there was nothing to delete.
	DetachTask(ctx context.Context, taskID models.TaskID) error
	// ListPositions returns the saved positions of the tasks of a workspace.
	ListPositions(ctx context.Context, workspaceID string) ([]models.TaskPosition, error)
}
