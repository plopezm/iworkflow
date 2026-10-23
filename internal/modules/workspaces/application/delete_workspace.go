//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
)

// DeleteWorkspace removes the registration of a workspace. It does not touch the folder on disk.
// Its tasks are deleted first, so a failure there keeps the workspace registered.
type DeleteWorkspace struct {
	repository outbound.WorkspaceRepository
	tasks      outbound.WorkspaceTasks
}

// NewDeleteWorkspace builds the use case.
func NewDeleteWorkspace(repository outbound.WorkspaceRepository, tasks outbound.WorkspaceTasks) *DeleteWorkspace {
	return &DeleteWorkspace{repository: repository, tasks: tasks}
}

// Execute deletes the tasks and then the workspace.
func (u *DeleteWorkspace) Execute(ctx context.Context, id models.WorkspaceID) error {
	if err := u.tasks.DeleteAll(ctx, id); err != nil {
		return err
	}
	return u.repository.Delete(ctx, id)
}
