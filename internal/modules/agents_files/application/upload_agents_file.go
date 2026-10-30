//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"fmt"
	"log/slog"

	"iworkflow/internal/modules/agents_files/domain/models"
	"iworkflow/internal/modules/agents_files/domain/ports/outbound"
)

// UploadAgentsFile validates and stores an AGENTS.md: content on disk, reference in the database.
type UploadAgentsFile struct {
	repository outbound.AgentsFileRepository
	storage    outbound.AgentsFileStorage
	clock      Clock
	newID      IDGenerator
	logger     *slog.Logger
}

// NewUploadAgentsFile builds the use case.
func NewUploadAgentsFile(
	repository outbound.AgentsFileRepository,
	storage outbound.AgentsFileStorage,
	clock Clock,
	newID IDGenerator,
	logger *slog.Logger,
) *UploadAgentsFile {
	return &UploadAgentsFile{repository: repository, storage: storage, clock: clock, newID: newID, logger: logger}
}

// Execute validates the input, stores the content and saves the reference. When saving the
// reference fails, the stored content is removed so no orphan file is left behind.
func (u *UploadAgentsFile) Execute(ctx context.Context, input models.NewAgentsFile) (models.AgentsFile, error) {
	validated, err := input.Validated()
	if err != nil {
		return models.AgentsFile{}, err
	}
	id, err := u.newID()
	if err != nil {
		return models.AgentsFile{}, fmt.Errorf("generate agents file id: %w", err)
	}
	path, err := u.storage.Store(ctx, id, validated.Content)
	if err != nil {
		return models.AgentsFile{}, err
	}
	file := models.AgentsFile{
		ID:          id,
		Description: validated.Description,
		Path:        path,
		CreatedAt:   u.clock().UTC(),
	}
	if err := u.repository.Save(ctx, file); err != nil {
		if cleanupErr := u.storage.Remove(ctx, path); cleanupErr != nil {
			u.logger.WarnContext(ctx, "could not remove stored agents file after a failed save",
				"path", path, "error", cleanupErr)
		}
		return models.AgentsFile{}, err
	}
	return file, nil
}
