//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package persistence

import (
	"context"
	"database/sql"
	"fmt"

	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
	"iworkflow/internal/modules/workspaces/infrastructure/adapters/outbound/persistence/gen"
)

// TemporaryWorkspaceTasks deletes the tasks of a workspace straight from the tasks table.
//
// TODO(tasks): temporary stand-in for the tasks module's WorkspaceTasks implementation, which is not
// migrated yet. Unlike the Rust cascade (DeleteWorkspaceTasks) it does not remove task worktrees,
// task branches or attachment folders, and it publishes no removal notification. Replace it with the
// tasks module adapter when that module lands, and remove it from internal/app/app.go at the same time.
type TemporaryWorkspaceTasks struct {
	db      *sql.DB
	queries *gen.Queries
}

var _ outbound.WorkspaceTasks = (*TemporaryWorkspaceTasks)(nil)

// NewTemporaryWorkspaceTasks builds the stand-in on an open database.
func NewTemporaryWorkspaceTasks(db *sql.DB) *TemporaryWorkspaceTasks {
	return &TemporaryWorkspaceTasks{db: db, queries: gen.New(db)}
}

// DeleteAll deletes every task of the workspace in one transaction. It returns InUse, and deletes
// nothing, when any task is still active.
func (t *TemporaryWorkspaceTasks) DeleteAll(ctx context.Context, id models.WorkspaceID) error {
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return werrors.Repository(fmt.Sprintf("begin task cleanup: %v", err))
	}
	defer func() { _ = tx.Rollback() }()

	queries := t.queries.WithTx(tx)
	active, err := queries.CountActiveTasksByWorkspace(ctx, id.String())
	if err != nil {
		return werrors.Repository(fmt.Sprintf("count active tasks: %v", err))
	}
	if active > 0 {
		return werrors.InUse()
	}
	if _, err := queries.DeleteTasksByWorkspace(ctx, id.String()); err != nil {
		return werrors.Repository(fmt.Sprintf("delete tasks: %v", err))
	}
	if err := tx.Commit(); err != nil {
		return werrors.Repository(fmt.Sprintf("commit task cleanup: %v", err))
	}
	return nil
}
