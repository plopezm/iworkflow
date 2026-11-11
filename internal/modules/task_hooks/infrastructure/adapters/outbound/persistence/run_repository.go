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

// runTimeLayout is the stored form of a run time: UTC with a fixed nine-digit fraction, so that the text order of
// the column is the chronological order. Rows written before it hold RFC 3339 text with a variable fraction.
const runTimeLayout = "2006-01-02T15:04:05.000000000Z"

// defaultRunLimit bounds a history query that names no limit.
const defaultRunLimit = 200

// RunRepository stores the run records of the hooks in the task_hook_runs table. Times are stored as fixed-width
// UTC text; an unfinished run has an empty finish time.
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
		finished = formatRunTime(run.FinishedAt)
	}
	err := r.queries.UpsertHookRun(ctx, gen.UpsertHookRunParams{
		ID: run.ID, TaskID: run.TaskID, HookID: run.HookID, AssignmentID: run.AssignmentID, HookName: run.HookName,
		WorkspaceID: run.WorkspaceID, Phase: string(run.Phase), Event: string(run.Event),
		Status: string(run.Status), ExitCode: int64(run.ExitCode), Output: run.Output,
		StartedAt: formatRunTime(run.StartedAt), FinishedAt: finished,
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
	return runsFromRows(rows)
}

// List returns the history that matches the filter, newest first.
func (r *RunRepository) List(ctx context.Context, filter outbound.RunFilter) ([]models.HookRun, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultRunLimit
	}
	rows, err := r.queries.ListHookRuns(ctx, gen.ListHookRunsParams{
		HookID: filter.HookID, WorkspaceID: filter.WorkspaceID, FailedOnly: boolToInt(filter.FailedOnly),
		RowLimit: int64(limit),
	})
	if err != nil {
		return nil, repositoryError(err)
	}
	return runsFromRows(rows)
}

// LastFailures returns the latest failed record of each hook that has one, by hook id.
func (r *RunRepository) LastFailures(ctx context.Context) (map[string]models.HookRun, error) {
	rows, err := r.queries.ListLastFailedRuns(ctx)
	if err != nil {
		return nil, repositoryError(err)
	}
	runs, err := runsFromRows(rows)
	if err != nil {
		return nil, err
	}
	failures := make(map[string]models.HookRun, len(runs))
	for _, run := range runs {
		failures[run.HookID] = run
	}
	return failures, nil
}

func runsFromRows(rows []gen.TaskHookRun) ([]models.HookRun, error) {
	runs := make([]models.HookRun, 0, len(rows))
	for _, row := range rows {
		started, err := parseRunTime(row.StartedAt)
		if err != nil {
			return nil, repositoryError(fmt.Errorf("decode run %s start: %w", row.ID, err))
		}
		var finished time.Time
		if row.FinishedAt != "" {
			if finished, err = parseRunTime(row.FinishedAt); err != nil {
				return nil, repositoryError(fmt.Errorf("decode run %s finish: %w", row.ID, err))
			}
		}
		runs = append(runs, models.HookRun{
			ID: row.ID, TaskID: row.TaskID, HookID: row.HookID, AssignmentID: row.AssignmentID, HookName: row.HookName,
			WorkspaceID: row.WorkspaceID, Phase: models.Phase(row.Phase), Event: models.Event(row.Event),
			Status: models.RunStatus(row.Status), ExitCode: int(row.ExitCode), Output: row.Output,
			StartedAt: started, FinishedAt: finished,
		})
	}
	return runs, nil
}

// FailStaleRunning closes every run still running as errored at finishedAt, with models.StaleRunOutput as output,
// and returns how many it closed.
func (r *RunRepository) FailStaleRunning(ctx context.Context, finishedAt time.Time) (int64, error) {
	closed, err := r.queries.FailStaleRunning(ctx, gen.FailStaleRunningParams{
		Output: models.StaleRunOutput, FinishedAt: formatRunTime(finishedAt),
	})
	if err != nil {
		return 0, repositoryError(err)
	}
	return closed, nil
}

func formatRunTime(value time.Time) string {
	return value.UTC().Format(runTimeLayout)
}

// parseRunTime reads a stored time: the fixed-width form, or the RFC 3339 text of the rows written before it.
func parseRunTime(value string) (time.Time, error) {
	parsed, err := time.Parse(runTimeLayout, value)
	if err == nil {
		return parsed, nil
	}
	legacy, legacyErr := time.Parse(time.RFC3339Nano, value)
	if legacyErr != nil {
		return time.Time{}, fmt.Errorf("%w (RFC 3339: %w)", err, legacyErr)
	}
	return legacy, nil
}
