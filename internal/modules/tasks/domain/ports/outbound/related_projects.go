package outbound

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
)

// TaskRelatedProjectRepository persists the projects (workspaces) a task relates to, in the order the user chose.
type TaskRelatedProjectRepository interface {
	// Replace stores exactly workspaceIDs as the related projects of the task, in one transaction. An empty list
	// clears them. Nothing changes when any row fails (a missing task or workspace is one such failure).
	Replace(ctx context.Context, taskID models.TaskID, workspaceIDs []string) error
	// ListByTask returns the workspace ids in the stored order.
	ListByTask(ctx context.Context, taskID models.TaskID) ([]string, error)
}
