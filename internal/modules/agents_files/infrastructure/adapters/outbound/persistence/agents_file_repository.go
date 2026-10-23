// Package persistence implements the agents_files repository port on SQLite through sqlc.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package persistence

import (
	"context"
	"database/sql"
	"errors"

	agentserrors "iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/domain/models"
	"iworkflow/internal/modules/agents_files/domain/ports/outbound"
	"iworkflow/internal/modules/agents_files/infrastructure/adapters/outbound/persistence/gen"
)

// AgentsFileRepository stores the AGENTS.md references in the agents_files table.
type AgentsFileRepository struct {
	queries *gen.Queries
}

var _ outbound.AgentsFileRepository = (*AgentsFileRepository)(nil)

// NewAgentsFileRepository builds the repository on an open database.
func NewAgentsFileRepository(db *sql.DB) *AgentsFileRepository {
	return &AgentsFileRepository{queries: gen.New(db)}
}

// Save inserts a new reference. A duplicate path is rejected by the UNIQUE constraint.
func (r *AgentsFileRepository) Save(ctx context.Context, file models.AgentsFile) error {
	err := r.queries.InsertAgentsFile(ctx, gen.InsertAgentsFileParams{
		ID:          file.ID.String(),
		Description: file.Description,
		Path:        file.Path,
		CreatedAt:   formatTimestamp(file.CreatedAt),
	})
	if err != nil {
		return agentserrors.Repository(err.Error())
	}
	return nil
}

// List returns every reference ordered by creation time (the use case sorts by description).
func (r *AgentsFileRepository) List(ctx context.Context) ([]models.AgentsFile, error) {
	rows, err := r.queries.ListAgentsFiles(ctx)
	if err != nil {
		return nil, agentserrors.Repository(err.Error())
	}
	files := make([]models.AgentsFile, 0, len(rows))
	for _, row := range rows {
		file, err := rowToDomain(row)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

// FindByID returns nil and no error when no row has the identifier.
func (r *AgentsFileRepository) FindByID(ctx context.Context, id models.AgentsFileID) (*models.AgentsFile, error) {
	row, err := r.queries.GetAgentsFileByID(ctx, id.String())
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, agentserrors.Repository(err.Error())
	}
	file, err := rowToDomain(row)
	if err != nil {
		return nil, err
	}
	return &file, nil
}

// Delete removes the reference, or returns agentserrors.NotFound when there is none.
func (r *AgentsFileRepository) Delete(ctx context.Context, id models.AgentsFileID) error {
	deleted, err := r.queries.DeleteAgentsFile(ctx, id.String())
	if err != nil {
		return agentserrors.Repository(err.Error())
	}
	if deleted == 0 {
		return agentserrors.NotFound(id.String())
	}
	return nil
}
