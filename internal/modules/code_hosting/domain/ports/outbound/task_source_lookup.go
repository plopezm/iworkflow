package outbound

import (
	"context"

	"iworkflow/internal/modules/code_hosting/domain/models"
)

// TaskSourceLookup resolves a task into what the code hosting module needs. The tasks module
// implements it; the composition root adapts it.
type TaskSourceLookup interface {
	// Find returns nil and no error when the task does not exist or has no branch yet.
	Find(ctx context.Context, taskID string) (*models.TaskSource, error)
}
