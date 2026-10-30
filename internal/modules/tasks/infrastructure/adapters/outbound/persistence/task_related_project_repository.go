//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package persistence

import (
	"context"
	"database/sql"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/persistence/gen"
)

// TaskRelatedProjectRepository stores the projects a task relates to. Replace runs in one BEGIN IMMEDIATE
// transaction, so a reader never sees a half-written list.
type TaskRelatedProjectRepository struct {
	db      *sql.DB
	queries *gen.Queries
}

var _ outbound.TaskRelatedProjectRepository = (*TaskRelatedProjectRepository)(nil)

// NewTaskRelatedProjectRepository builds the repository on an open database.
func NewTaskRelatedProjectRepository(db *sql.DB) *TaskRelatedProjectRepository {
	return &TaskRelatedProjectRepository{db: db, queries: gen.New(db)}
}

// Replace swaps the related projects of the task for workspaceIDs, keeping their order.
func (r *TaskRelatedProjectRepository) Replace(ctx context.Context, taskID models.TaskID, workspaceIDs []string) error {
	return withImmediateTx(ctx, r.db, func(queries *gen.Queries) error {
		if err := queries.DeleteTaskRelatedProjects(ctx, taskID.String()); err != nil {
			return dbError(err)
		}
		for index, workspaceID := range workspaceIDs {
			err := queries.InsertTaskRelatedProject(ctx, gen.InsertTaskRelatedProjectParams{
				TaskID:      taskID.String(),
				WorkspaceID: workspaceID,
				Position:    int64(index),
			})
			if err != nil {
				return dbError(err)
			}
		}
		return nil
	})
}

// ListByTask returns the related workspace ids of the task in the stored order.
func (r *TaskRelatedProjectRepository) ListByTask(ctx context.Context, taskID models.TaskID) ([]string, error) {
	ids, err := r.queries.ListTaskRelatedProjects(ctx, taskID.String())
	if err != nil {
		return nil, dbError(err)
	}
	return ids, nil
}
