package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/outbound/persistence/gen"
)

// RunRepository stores the run records of the hooks in the task_hook_runs table. Times are stored as
// RFC 3339 text; an unfinished run has an empty finish time.
type RunRepository struct {
	queries *gen.Queries
}

var _ outbound.RunRepository = (*RunRepository)(nil)

// NewRunRepository builds the repository on an open database.
func NewRunRepository(db *sql.DB) *RunRepository {
	return &RunRepository{queries: gen.New(db)}
}

// Save inserts the record, or updates the status, exit code, output and finish time of an existing one.
func (r *RunRepository) Save(ctx context.Context, run models.HookRun) error {
	finished := ""
	if !run.FinishedAt.IsZero() {
		finished = run.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	err := r.queries.UpsertHookRun(ctx, gen.UpsertHookRunParams{
		ID: run.ID, TaskID: run.TaskID, HookID: run.HookID, Phase: string(run.Phase), Event: string(run.Event),
		Status: string(run.Status), ExitCode: int64(run.ExitCode), Output: run.Output,
		StartedAt: run.StartedAt.UTC().Format(time.RFC3339Nano), FinishedAt: finished,
	})
	if err != nil {
		return repositoryError(err)
	}
	return nil
}

// ListByTask returns the records of one task in start order.
func (r *RunRepository) ListByTask(ctx context.Context, taskID string) ([]models.HookRun, error) {
	rows, err := r.queries.ListHookRunsByTask(ctx, taskID)
	if err != nil {
		return nil, repositoryError(err)
	}
	runs := make([]models.HookRun, 0, len(rows))
	for _, row := range rows {
		started, err := time.Parse(time.RFC3339Nano, row.StartedAt)
		if err != nil {
			return nil, repositoryError(fmt.Errorf("decode run %s start: %w", row.ID, err))
		}
		var finished time.Time
		if row.FinishedAt != "" {
			if finished, err = time.Parse(time.RFC3339Nano, row.FinishedAt); err != nil {
				return nil, repositoryError(fmt.Errorf("decode run %s finish: %w", row.ID, err))
			}
		}
		runs = append(runs, models.HookRun{
			ID: row.ID, TaskID: row.TaskID, HookID: row.HookID, Phase: models.Phase(row.Phase),
			Event: models.Event(row.Event), Status: models.RunStatus(row.Status), ExitCode: int(row.ExitCode),
			Output: row.Output, StartedAt: started, FinishedAt: finished,
		})
	}
	return runs, nil
}
