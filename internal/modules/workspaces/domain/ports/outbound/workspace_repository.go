// Package outbound holds the interfaces the workspaces use cases need from infrastructure and from
// other modules. They are implemented by adapters and wired in the composition root.
package outbound

import (
	"context"

	"iworkflow/internal/modules/workspaces/domain/models"
)

// WorkspaceRepository persists workspaces.
type WorkspaceRepository interface {
	Save(ctx context.Context, workspace models.Workspace) error
	List(ctx context.Context) ([]models.Workspace, error)
	// Update changes the name and base branch only. It returns NotFound when there is no such workspace.
	Update(ctx context.Context, workspace models.Workspace) error
	// FindByID returns nil and no error when there is none.
	FindByID(ctx context.Context, id models.WorkspaceID) (*models.Workspace, error)
	// FindByPath returns nil and no error when there is none.
	FindByPath(ctx context.Context, path string) (*models.Workspace, error)
	// Delete returns NotFound when there is no such workspace. It returns InUse when a task still
	// references it: the foreign key is the safety net against races, the normal path deletes the
	// tasks first through WorkspaceTasks.
	Delete(ctx context.Context, id models.WorkspaceID) error
}
