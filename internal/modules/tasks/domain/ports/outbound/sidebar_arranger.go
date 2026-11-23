package outbound

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
)

// SidebarArranger splits the workspaces of the sidebar into pinned, grouped and ungrouped. It is consumer-side:
// the composition root implements it with the workspace_groups module, so tasks never imports that module.
type SidebarArranger interface {
	// Arrange returns the layout of the given workspace ids, which keep their order inside each block.
	Arrange(ctx context.Context, workspaceIDs []string) (models.SidebarLayout, error)
}
