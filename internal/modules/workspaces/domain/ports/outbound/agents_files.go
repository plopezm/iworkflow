package outbound

import (
	"context"

	"iworkflow/internal/modules/workspaces/domain/models"
)

// AgentsFiles gives access to the stored AGENTS.md files. It is implemented by the agents_files
// module and wired by the composition root: this module never imports it.
type AgentsFiles interface {
	ListOptions(ctx context.Context) ([]models.AgentsFileOption, error)
	// Install writes the file as AGENTS.md in root. With overwrite it replaces an existing one;
	// without it the call fails with AgentsFileTargetExists.
	Install(ctx context.Context, agentsFileID string, root string, overwrite bool) error
}
