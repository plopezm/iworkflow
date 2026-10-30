//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/domain/models"
	"iworkflow/internal/modules/agents_files/domain/ports/outbound"
)

// DeleteAgentsFile removes the reference and the file on disk. It does not touch the AGENTS.md
// files already installed in projects.
type DeleteAgentsFile struct {
	repository outbound.AgentsFileRepository
	storage    outbound.AgentsFileStorage
}

// NewDeleteAgentsFile builds the use case.
func NewDeleteAgentsFile(repository outbound.AgentsFileRepository, storage outbound.AgentsFileStorage) *DeleteAgentsFile {
	return &DeleteAgentsFile{repository: repository, storage: storage}
}

// Execute deletes the row first, then the file. A failed row deletion keeps the file.
func (u *DeleteAgentsFile) Execute(ctx context.Context, id models.AgentsFileID) error {
	file, err := u.repository.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if file == nil {
		return errors.NotFound(id.String())
	}
	if err := u.repository.Delete(ctx, id); err != nil {
		return err
	}
	return u.storage.Remove(ctx, file.Path)
}
