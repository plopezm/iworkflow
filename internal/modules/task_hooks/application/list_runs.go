package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// ListRuns returns the run history of the hooks, newest first, for the error view.
type ListRuns struct {
	runs outbound.RunRepository
}

// NewListRuns builds the use case.
func NewListRuns(runs outbound.RunRepository) *ListRuns {
	return &ListRuns{runs: runs}
}

// Execute returns the runs that match the filter.
func (u *ListRuns) Execute(ctx context.Context, filter outbound.RunFilter) ([]models.HookRun, error) {
	runs, err := u.runs.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	return runs, nil
}
