package outbound

import (
	"context"

	"iworkflow/internal/modules/jira/domain/models"
)

// WorkspaceCatalog lists the workspaces and branches a converted task can be mapped onto. The
// composition root binds it to the workspaces module (Rust: WorkspaceCatalogBridge in src/app_state.rs).
type WorkspaceCatalog interface {
	Workspaces(ctx context.Context) ([]models.WorkspaceOption, error)
	Branches(ctx context.Context, workspaceID string) ([]models.BranchOption, error)
}
