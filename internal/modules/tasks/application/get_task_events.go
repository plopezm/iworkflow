//nolint:wrapcheck // Port errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// GetTaskEvents returns the persisted history of a task after a sequence number (exclusive). A nil
// cursor means the whole history.
type GetTaskEvents struct {
	repository outbound.TaskRepository
}

// NewGetTaskEvents builds the use case.
func NewGetTaskEvents(repository outbound.TaskRepository) *GetTaskEvents {
	return &GetTaskEvents{repository: repository}
}

// Execute reads the events after the cursor, in sequence order.
func (u *GetTaskEvents) Execute(ctx context.Context, id models.TaskID, afterSeq *int64) ([]models.TaskEvent, error) {
	return u.repository.Events(ctx, id, afterSeq)
}
