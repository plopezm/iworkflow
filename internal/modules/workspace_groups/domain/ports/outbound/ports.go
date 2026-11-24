// Package outbound holds the interfaces the workspace_groups use cases need from infrastructure.
package outbound

import (
	"context"

	"iworkflow/internal/modules/workspace_groups/domain/models"
)

// GroupRepository persists groups, their members and the pinned workspaces.
type GroupRepository interface {
	// ListGroups returns the groups in display order.
	ListGroups(ctx context.Context) ([]models.Group, error)
	// SaveGroup inserts a new group at the end of the list.
	SaveGroup(ctx context.Context, group models.Group) error
	// UpdateGroup changes the name and color; the collapsed flag is left as stored (see SetCollapsed). It fails with ErrGroupNotFound.
	UpdateGroup(ctx context.Context, group models.Group) error
	// DeleteGroup removes the group and its memberships, never the workspaces. It fails with ErrGroupNotFound.
	DeleteGroup(ctx context.Context, id string) error
	// SetCollapsed stores only the open/closed state. It fails with ErrGroupNotFound.
	SetCollapsed(ctx context.Context, id string, collapsed bool) error
	// Memberships maps each grouped workspace id to its group id.
	Memberships(ctx context.Context) (map[string]string, error)
	// Assign puts the workspace in the group. It fails with ErrGroupNotFound or ErrWorkspaceNotFound.
	Assign(ctx context.Context, workspaceID string, groupID string) error
	// Unassign removes the workspace from its group; a workspace without group is not an error.
	Unassign(ctx context.Context, workspaceID string) error
	// Pinned returns the pinned workspace ids in pin order.
	Pinned(ctx context.Context) ([]string, error)
	// Pin pins the workspace at the end of the pins. It fails with ErrWorkspaceNotFound.
	Pin(ctx context.Context, workspaceID string) error
	// Unpin removes the pin; an unpinned workspace is not an error.
	Unpin(ctx context.Context, workspaceID string) error
}

// IDGenerator creates the identifiers of new groups, so tests can fix them.
type IDGenerator interface {
	NewID() string
}
