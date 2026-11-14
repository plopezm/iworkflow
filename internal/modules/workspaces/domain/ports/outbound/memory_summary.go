package outbound

import (
	"context"

	"iworkflow/internal/modules/workspaces/domain/models"
)

// MemorySummaryPort reads what the memory provider remembers about many workspaces. It is implemented by an
// adapter in the composition root, so this module never imports the memory provider. An unreachable provider is
// reported as models.MemoryUnavailable for every workspace, with a nil error; errors are for unexpected failures.
type MemorySummaryPort interface {
	// Summaries returns one memory summary per workspace, in the order of the input, from one read of the
	// provider. It must be read-only.
	Summaries(ctx context.Context, workspaces []models.Workspace) ([]models.MemorySummary, error)
}
