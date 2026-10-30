//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	werrors "iworkflow/internal/modules/workspaces/domain/errors"
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

// Execute deletes the tasks and then the workspace. A workspace that does not exist is reported as
// NotFound before any task is touched. The two steps are not atomic: when the workspace delete fails
// after the tasks are gone, the tasks stay deleted (there is no transaction across the ports).
func (u *DeleteWorkspace) Execute(ctx context.Context, id models.WorkspaceID) error {
	found, err := u.repository.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if found == nil {
		return werrors.NotFound(id.String())
	}
	if err := u.tasks.DeleteAll(ctx, id); err != nil {
		return err
	}
	return u.repository.Delete(ctx, id)
}
