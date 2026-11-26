package outbound

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
)

// DiscardListener is told that a task was discarded, so other features can release what they tied to it (for example
// the Jira issue it was created from). It cannot fail the discard: the task is already discarded, so an implementation
// reports its own failures.
type DiscardListener interface {
	Discarded(ctx context.Context, id models.TaskID)
}
