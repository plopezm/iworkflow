//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/domain/models"
	"iworkflow/internal/modules/agents_files/domain/ports/outbound"
)

// InstallAgentsFile writes a stored AGENTS.md at the root of a folder.
type InstallAgentsFile struct {
	repository outbound.AgentsFileRepository
	storage    outbound.AgentsFileStorage
	installer  outbound.AgentsFileInstaller
}

// NewInstallAgentsFile builds the use case.
func NewInstallAgentsFile(
	repository outbound.AgentsFileRepository,
	storage outbound.AgentsFileStorage,
	installer outbound.AgentsFileInstaller,
) *InstallAgentsFile {
	return &InstallAgentsFile{repository: repository, storage: storage, installer: installer}
}

// Execute reads the stored content and installs it into dir.
func (u *InstallAgentsFile) Execute(ctx context.Context, id models.AgentsFileID, dir string, overwrite bool) (models.InstallOutcome, error) {
	file, err := u.repository.FindByID(ctx, id)
	if err != nil {
		return models.InstallOutcome{}, err
	}
	if file == nil {
		return models.InstallOutcome{}, errors.NotFound(id.String())
	}
	content, err := u.storage.Read(ctx, file.Path)
	if err != nil {
		return models.InstallOutcome{}, err
	}
	return u.installer.Install(ctx, content, dir, overwrite)
}
