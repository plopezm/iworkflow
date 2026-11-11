package outbound

import (
	"context"

	"iworkflow/internal/modules/jira/domain/models"
)

// WorklogRepository stores the worklogs iworkflow sent to Jira, per task.
type WorklogRepository interface {
	Add(ctx context.Context, record models.WorklogRecord) error
	// ListByTask returns the worklogs of a task, oldest first.
	ListByTask(ctx context.Context, taskID string) ([]models.WorklogRecord, error)
}
