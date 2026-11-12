//nolint:wrapcheck // Port errors are returned unwrapped: their text is user-visible.
package application

import (
	"context"
	"time"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// GetTaskRunTime tells how long the agent worked on a task, from its persisted history.
type GetTaskRunTime struct {
	repository outbound.TaskRepository
	clock      Clock
}

// NewGetTaskRunTime builds the use case.
func NewGetTaskRunTime(repository outbound.TaskRepository, clock Clock) *GetTaskRunTime {
	return &GetTaskRunTime{repository: repository, clock: clock}
}

// Execute reads the whole history of the task and sums the agent working time.
func (u *GetTaskRunTime) Execute(ctx context.Context, id models.TaskID) (time.Duration, error) {
	events, err := u.repository.Events(ctx, id, nil)
	if err != nil {
		return 0, err
	}
	return models.AgentRunTime(events, u.clock()), nil
}
