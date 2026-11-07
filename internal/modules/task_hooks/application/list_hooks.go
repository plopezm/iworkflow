package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// ListHooks returns the hooks of a workspace in position order, for the editor of the workspace.
type ListHooks struct {
	hooks outbound.HookRepository
}

// NewListHooks builds the use case.
func NewListHooks(hooks outbound.HookRepository) *ListHooks {
	return &ListHooks{hooks: hooks}
}

// Execute returns the hooks of the workspace, enabled or not.
func (u *ListHooks) Execute(ctx context.Context, workspaceID string) ([]models.Hook, error) {
	hooks, err := u.hooks.List(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list hooks: %w", err)
	}
	return hooks, nil
}
