// Package outbound holds the interfaces the agents_files use cases need from infrastructure.
package outbound

import (
	"context"

	"iworkflow/internal/modules/agents_files/domain/models"
)

// AgentsFileRepository persists the references to stored AGENTS.md files.
type AgentsFileRepository interface {
	Save(ctx context.Context, file models.AgentsFile) error
	// List returns every reference, oldest first.
	List(ctx context.Context) ([]models.AgentsFile, error)
	// FindByID returns nil and no error when the identifier is unknown.
	FindByID(ctx context.Context, id models.AgentsFileID) (*models.AgentsFile, error)
	// Delete returns errors.NotFound when no row matches.
	Delete(ctx context.Context, id models.AgentsFileID) error
}
