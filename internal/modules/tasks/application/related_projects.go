//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// ensureWorkspacesExist returns WorkspaceNotFound for the first related workspace that does not exist.
// It only reads, so the use cases can call it before they write anything.
func ensureWorkspacesExist(ctx context.Context, lifecycle *TaskLifecycle, workspaceIDs []string) error {
	for _, workspaceID := range workspaceIDs {
		if _, err := lifecycle.Workspace(ctx, workspaceID); err != nil {
			return err
		}
	}
	return nil
}

// ListTaskRelatedProjects returns the workspaces a task relates to, in the order they were stored.
// A related workspace that no longer exists is skipped: the list shows what is still there.
type ListTaskRelatedProjects struct {
	relatedProjects outbound.TaskRelatedProjectRepository
	workspaces      outbound.WorkspaceLookup
}

// NewListTaskRelatedProjects builds the use case.
func NewListTaskRelatedProjects(relatedProjects outbound.TaskRelatedProjectRepository, workspaces outbound.WorkspaceLookup) *ListTaskRelatedProjects {
	return &ListTaskRelatedProjects{relatedProjects: relatedProjects, workspaces: workspaces}
}

// Execute returns the related workspaces of the task. A task without related projects gets an empty list.
func (u *ListTaskRelatedProjects) Execute(ctx context.Context, id models.TaskID) ([]models.WorkspaceRef, error) {
	workspaceIDs, err := u.relatedProjects.ListByTask(ctx, id)
	if err != nil {
		return nil, err
	}
	refs := make([]models.WorkspaceRef, 0, len(workspaceIDs))
	for _, workspaceID := range workspaceIDs {
		found, err := u.workspaces.Find(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		if found == nil {
			continue
		}
		refs = append(refs, *found)
	}
	return refs, nil
}
