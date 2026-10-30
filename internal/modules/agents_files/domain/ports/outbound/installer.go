package outbound

import (
	"context"

	"iworkflow/internal/modules/agents_files/domain/models"
)

// AgentsFileInstaller writes an AGENTS.md at the root of a project folder.
type AgentsFileInstaller interface {
	// Install returns errors.TargetExists when an AGENTS.md is already there and overwrite is false.
	Install(ctx context.Context, content []byte, dir string, overwrite bool) (models.InstallOutcome, error)
}
