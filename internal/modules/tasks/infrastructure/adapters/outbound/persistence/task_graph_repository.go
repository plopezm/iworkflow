//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package persistence

import (
	"context"
	"database/sql"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/persistence/gen"
)

// TaskGraphRepository stores the dependency edges and the node positions of the board in SQLite.
//
// The edges of a workspace are validated and written in one BEGIN IMMEDIATE transaction, so two concurrent
// inserts cannot close a cycle between them. Detaching a task is one transaction too; the other methods are
// single statements.
type TaskGraphRepository struct {
	db      *sql.DB
	queries *gen.Queries
	clock   Clock
}

var _ outbound.TaskGraphRepository = (*TaskGraphRepository)(nil)

// NewTaskGraphRepository builds the repository on an open database.
func NewTaskGraphRepository(db *sql.DB, clock Clock) *TaskGraphRepository {
	return &TaskGraphRepository{db: db, queries: gen.New(db), clock: clock}
}

// InsertDependency validates the edge against the edges of its workspace (no self edge, no duplicate, no
// cycle) and stores it in the same transaction. A rejected edge stores nothing.
func (r *TaskGraphRepository) InsertDependency(ctx context.Context, dependency models.TaskDependency) error {
	params := gen.InsertTaskDependencyParams{
		PredecessorID: dependency.Predecessor.String(),
		SuccessorID:   dependency.Successor.String(),
		WorkspaceID:   dependency.WorkspaceID,
		TriggerEvent:  string(dependency.Trigger),
		CreatedAt:     formatTimestamp(r.clock()),
	}
	return withImmediateTx(ctx, r.db, func(queries *gen.Queries) error {
		rows, err := queries.ListTaskDependencies(ctx, dependency.WorkspaceID)
		if err != nil {
			return dbError(err)
		}
		existing, err := dependenciesFromRows(rows)
		if err != nil {
			return err
		}
		if err := models.ValidateNewDependency(existing, dependency); err != nil {
			return err
		}
		if err := queries.InsertTaskDependency(ctx, params); err != nil {
			return dbError(err)
		}
		return nil
	})
}

// RemoveDependency deletes the edge and reports whether it existed.
func (r *TaskGraphRepository) RemoveDependency(ctx context.Context, predecessor models.TaskID, successor models.TaskID) (bool, error) {
	removed, err := r.queries.DeleteTaskDependency(ctx, gen.DeleteTaskDependencyParams{
		PredecessorID: predecessor.String(),
		SuccessorID:   successor.String(),
	})
	if err != nil {
		return false, dbError(err)
	}
	return removed > 0, nil
}

// ListDependencies returns the edges of one workspace, or of every workspace when the filter is nil.
func (r *TaskGraphRepository) ListDependencies(ctx context.Context, workspaceID *string) ([]models.TaskDependency, error) {
	rows, err := r.queries.ListTaskDependencies(ctx, nullableWorkspace(workspaceID))
	if err != nil {
		return nil, dbError(err)
	}
	return dependenciesFromRows(rows)
}

// SetPosition stores or replaces the position of a task node. The statement inserts only while the task
// exists, so a task deleted in parallel leaves no orphan row and no error.
func (r *TaskGraphRepository) SetPosition(ctx context.Context, taskID models.TaskID, position models.NodePosition) error {
	err := r.queries.UpsertTaskPosition(ctx, gen.UpsertTaskPositionParams{
		TaskID: taskID.String(),
		X:      position.X,
		Y:      position.Y,
	})
	if err != nil {
		return dbError(err)
	}
	return nil
}

// DetachTask deletes every edge of the task, in both directions, and its position in one transaction.
func (r *TaskGraphRepository) DetachTask(ctx context.Context, taskID models.TaskID) error {
	id := taskID.String()
	return withImmediateTx(ctx, r.db, func(queries *gen.Queries) error {
		if err := queries.DeleteTaskDependenciesOfTask(ctx, id); err != nil {
			return dbError(err)
		}
		if err := queries.DeleteTaskPosition(ctx, id); err != nil {
			return dbError(err)
		}
		return nil
	})
}

// ListPositions returns the saved positions of the tasks of one workspace.
func (r *TaskGraphRepository) ListPositions(ctx context.Context, workspaceID string) ([]models.TaskPosition, error) {
	rows, err := r.queries.ListTaskPositionsOfWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, dbError(err)
	}
	positions := make([]models.TaskPosition, 0, len(rows))
	for _, row := range rows {
		position, err := positionFromRow(row)
		if err != nil {
			return nil, err
		}
		positions = append(positions, position)
	}
	return positions, nil
}
