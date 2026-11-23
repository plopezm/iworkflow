// Package application holds the use cases of the workspace_groups module.
package application

import (
	"context"
	"fmt"

	gerrors "iworkflow/internal/modules/workspace_groups/domain/errors"
	"iworkflow/internal/modules/workspace_groups/domain/models"
	"iworkflow/internal/modules/workspace_groups/domain/ports/outbound"
)

// GroupInput is the editable data of a group.
type GroupInput struct {
	Name  string
	Color string
}

// CreateGroup adds a group at the end of the list.
type CreateGroup struct {
	repo outbound.GroupRepository
	ids  outbound.IDGenerator
}

// NewCreateGroup builds the use case.
func NewCreateGroup(repo outbound.GroupRepository, ids outbound.IDGenerator) *CreateGroup {
	return &CreateGroup{repo: repo, ids: ids}
}

// Execute validates the input and stores a new, expanded group.
func (u *CreateGroup) Execute(ctx context.Context, input GroupInput) (models.Group, error) {
	group, err := models.NewGroup(u.ids.NewID(), input.Name, input.Color, false)
	if err != nil {
		return models.Group{}, fmt.Errorf("create group: %w", err)
	}
	if err := u.repo.SaveGroup(ctx, group); err != nil {
		return models.Group{}, fmt.Errorf("create group: %w", err)
	}
	return group, nil
}

// UpdateGroup renames and recolors a group, keeping its collapsed state.
type UpdateGroup struct{ repo outbound.GroupRepository }

// NewUpdateGroup builds the use case.
func NewUpdateGroup(repo outbound.GroupRepository) *UpdateGroup { return &UpdateGroup{repo: repo} }

// Execute fails with ErrGroupNotFound or ErrInvalidGroup.
func (u *UpdateGroup) Execute(ctx context.Context, id string, input GroupInput) (models.Group, error) {
	groups, err := u.repo.ListGroups(ctx)
	if err != nil {
		return models.Group{}, fmt.Errorf("update group: %w", err)
	}
	for _, current := range groups {
		if current.ID() != id {
			continue
		}
		updated, err := models.NewGroup(id, input.Name, input.Color, current.Collapsed())
		if err != nil {
			return models.Group{}, fmt.Errorf("update group: %w", err)
		}
		if err := u.repo.UpdateGroup(ctx, updated); err != nil {
			return models.Group{}, fmt.Errorf("update group: %w", err)
		}
		return updated, nil
	}
	return models.Group{}, fmt.Errorf("update group: %w", gerrors.ErrGroupNotFound)
}

// DeleteGroup removes a group; its workspaces become ungrouped.
type DeleteGroup struct{ repo outbound.GroupRepository }

// NewDeleteGroup builds the use case.
func NewDeleteGroup(repo outbound.GroupRepository) *DeleteGroup { return &DeleteGroup{repo: repo} }

// Execute fails with ErrGroupNotFound.
func (u *DeleteGroup) Execute(ctx context.Context, id string) error {
	if err := u.repo.DeleteGroup(ctx, id); err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	return nil
}

// SetGroupCollapsed stores whether the sidebar shows a group closed.
type SetGroupCollapsed struct{ repo outbound.GroupRepository }

// NewSetGroupCollapsed builds the use case.
func NewSetGroupCollapsed(repo outbound.GroupRepository) *SetGroupCollapsed {
	return &SetGroupCollapsed{repo: repo}
}

// Execute fails with ErrGroupNotFound.
func (u *SetGroupCollapsed) Execute(ctx context.Context, id string, collapsed bool) error {
	if err := u.repo.SetCollapsed(ctx, id, collapsed); err != nil {
		return fmt.Errorf("collapse group: %w", err)
	}
	return nil
}

// MoveWorkspace puts a workspace in a group, or frees it when the group id is empty.
type MoveWorkspace struct{ repo outbound.GroupRepository }

// NewMoveWorkspace builds the use case.
func NewMoveWorkspace(repo outbound.GroupRepository) *MoveWorkspace {
	return &MoveWorkspace{repo: repo}
}

// Execute fails with ErrGroupNotFound or ErrWorkspaceNotFound.
func (u *MoveWorkspace) Execute(ctx context.Context, workspaceID string, groupID string) error {
	if groupID == "" {
		if err := u.repo.Unassign(ctx, workspaceID); err != nil {
			return fmt.Errorf("ungroup workspace: %w", err)
		}
		return nil
	}
	if err := u.repo.Assign(ctx, workspaceID, groupID); err != nil {
		return fmt.Errorf("move workspace: %w", err)
	}
	return nil
}

// SetPinned pins or unpins a workspace.
type SetPinned struct{ repo outbound.GroupRepository }

// NewSetPinned builds the use case.
func NewSetPinned(repo outbound.GroupRepository) *SetPinned { return &SetPinned{repo: repo} }

// Execute fails with ErrWorkspaceNotFound when pinning a workspace that does not exist.
func (u *SetPinned) Execute(ctx context.Context, workspaceID string, pinned bool) error {
	if pinned {
		if err := u.repo.Pin(ctx, workspaceID); err != nil {
			return fmt.Errorf("pin workspace: %w", err)
		}
		return nil
	}
	if err := u.repo.Unpin(ctx, workspaceID); err != nil {
		return fmt.Errorf("unpin workspace: %w", err)
	}
	return nil
}
