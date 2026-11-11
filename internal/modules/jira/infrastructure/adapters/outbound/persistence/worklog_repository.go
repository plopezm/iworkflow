//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible.
package persistence

import (
	"context"
	"database/sql"

	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
	"iworkflow/internal/modules/jira/infrastructure/adapters/outbound/persistence/gen"
)

// WorklogRepository stores the worklogs sent to Jira in jira_worklogs.
type WorklogRepository struct {
	queries *gen.Queries
}

var _ outbound.WorklogRepository = (*WorklogRepository)(nil)

// NewWorklogRepository builds the repository on an open database.
func NewWorklogRepository(db *sql.DB) *WorklogRepository {
	return &WorklogRepository{queries: gen.New(db)}
}

// Add stores one sent worklog.
func (r *WorklogRepository) Add(ctx context.Context, record models.WorklogRecord) error {
	err := r.queries.InsertJiraWorklog(ctx, gen.InsertJiraWorklogParams{
		TaskID:        record.TaskID,
		IssueKey:      record.IssueKey,
		JiraWorklogID: record.JiraID,
		Seconds:       int64(record.Seconds),
		Comment:       record.Comment,
		StartedAt:     record.StartedAt.UTC(),
		LoggedAt:      record.LoggedAt.UTC(),
	})
	if err != nil {
		return dbError(err)
	}
	return nil
}

// ListByTask returns the worklogs of a task, oldest first.
func (r *WorklogRepository) ListByTask(ctx context.Context, taskID string) ([]models.WorklogRecord, error) {
	rows, err := r.queries.ListJiraWorklogsByTask(ctx, taskID)
	if err != nil {
		return nil, dbError(err)
	}
	records := make([]models.WorklogRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, models.WorklogRecord{
			TaskID: row.TaskID, IssueKey: row.IssueKey, JiraID: row.JiraWorklogID,
			Seconds: int(row.Seconds), Comment: row.Comment, StartedAt: row.StartedAt.UTC(), LoggedAt: row.LoggedAt.UTC(),
		})
	}
	return records, nil
}
