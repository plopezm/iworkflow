// Package persistence implements the workspaces repository port on SQLite through sqlc.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package persistence

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"modernc.org/sqlite"

	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
	"iworkflow/internal/modules/workspaces/infrastructure/adapters/outbound/persistence/gen"
)

// SQLite extended result codes. The driver does not export them under stable names, so they are
// listed here: UNIQUE and PRIMARYKEY both mean "already registered", FOREIGNKEY means "in use".
const (
	sqliteConstraintUnique     = 2067
	sqliteConstraintPrimaryKey = 1555
	sqliteConstraintForeignKey = 787
)

// foreignKeyMessage is how SQLite reports a foreign key violation; it is matched when the code is not
// classified, as the Rust adapter does.
const foreignKeyMessage = "FOREIGN KEY constraint failed"

// WorkspaceRepository stores workspaces in the workspaces table.
type WorkspaceRepository struct {
	queries *gen.Queries
}

var _ outbound.WorkspaceRepository = (*WorkspaceRepository)(nil)

// NewWorkspaceRepository builds the repository on an open database.
func NewWorkspaceRepository(db *sql.DB) *WorkspaceRepository {
	return &WorkspaceRepository{queries: gen.New(db)}
}

// Save inserts a workspace. A duplicate path is reported as AlreadyRegistered.
func (r *WorkspaceRepository) Save(ctx context.Context, workspace models.Workspace) error {
	err := r.queries.InsertWorkspace(ctx, gen.InsertWorkspaceParams{
		ID:         workspace.ID.String(),
		Name:       workspace.Name,
		Path:       workspace.Path,
		BaseBranch: workspace.BaseBranch,
		CreatedAt:  formatTimestamp(workspace.CreatedAt),
	})
	if err != nil {
		return mapError(err, &workspace)
	}
	return nil
}

// List returns every workspace, oldest first (the use case sorts them by name).
func (r *WorkspaceRepository) List(ctx context.Context) ([]models.Workspace, error) {
	rows, err := r.queries.ListWorkspaces(ctx)
	if err != nil {
		return nil, mapError(err, nil)
	}
	workspaces := make([]models.Workspace, 0, len(rows))
	for _, row := range rows {
		workspace, err := rowToDomain(row)
		if err != nil {
			return nil, err
		}
		workspaces = append(workspaces, workspace)
	}
	return workspaces, nil
}

// Update changes the name and base branch. It returns NotFound when no row has the identifier.
func (r *WorkspaceRepository) Update(ctx context.Context, workspace models.Workspace) error {
	updated, err := r.queries.UpdateWorkspaceNameAndBranch(ctx, gen.UpdateWorkspaceNameAndBranchParams{
		ID:         workspace.ID.String(),
		Name:       workspace.Name,
		BaseBranch: workspace.BaseBranch,
	})
	if err != nil {
		return mapError(err, &workspace)
	}
	if updated == 0 {
		return werrors.NotFound(workspace.ID.String())
	}
	return nil
}

// FindByID returns nil and no error when no row has the identifier.
func (r *WorkspaceRepository) FindByID(ctx context.Context, id models.WorkspaceID) (*models.Workspace, error) {
	row, err := r.queries.GetWorkspaceByID(ctx, id.String())
	return r.single(row, err)
}

// FindByPath returns nil and no error when no row has the path.
func (r *WorkspaceRepository) FindByPath(ctx context.Context, path string) (*models.Workspace, error) {
	row, err := r.queries.GetWorkspaceByPath(ctx, path)
	return r.single(row, err)
}

// Delete removes the workspace. It returns NotFound when there is none and InUse when a task still
// references it.
func (r *WorkspaceRepository) Delete(ctx context.Context, id models.WorkspaceID) error {
	deleted, err := r.queries.DeleteWorkspace(ctx, id.String())
	if err != nil {
		return mapError(err, nil)
	}
	if deleted == 0 {
		return werrors.NotFound(id.String())
	}
	return nil
}

func (r *WorkspaceRepository) single(row gen.Workspace, err error) (*models.Workspace, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapError(err, nil)
	}
	workspace, err := rowToDomain(row)
	if err != nil {
		return nil, err
	}
	return &workspace, nil
}

// mapError translates driver errors into domain errors. workspace, when given, supplies the path
// reported for a duplicate.
func mapError(err error, workspace *models.Workspace) error {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		switch sqliteErr.Code() {
		case sqliteConstraintUnique, sqliteConstraintPrimaryKey:
			path := ""
			if workspace != nil {
				path = workspace.Path
			}
			return werrors.AlreadyRegistered(path)
		case sqliteConstraintForeignKey:
			return werrors.InUse()
		}
		if strings.Contains(sqliteErr.Error(), foreignKeyMessage) {
			return werrors.InUse()
		}
	}
	return werrors.Repository(err.Error())
}
