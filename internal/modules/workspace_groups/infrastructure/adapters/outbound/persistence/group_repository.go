// Package persistence implements the group repository port on SQLite through sqlc.
package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"modernc.org/sqlite"

	gerrors "iworkflow/internal/modules/workspace_groups/domain/errors"
	"iworkflow/internal/modules/workspace_groups/domain/models"
	"iworkflow/internal/modules/workspace_groups/domain/ports/outbound"
	"iworkflow/internal/modules/workspace_groups/infrastructure/adapters/outbound/persistence/gen"
)

// GroupRepository stores groups, memberships and pins in the workspace_groups, workspace_group_members and
// workspace_pins tables.
type GroupRepository struct {
	db      *sql.DB
	queries *gen.Queries
}

// sqliteConstraintForeignKey is the extended result code SQLite reports for a broken foreign key.
const sqliteConstraintForeignKey = 787

var _ outbound.GroupRepository = (*GroupRepository)(nil)

// NewGroupRepository builds the repository on an open database.
func NewGroupRepository(db *sql.DB) *GroupRepository {
	return &GroupRepository{db: db, queries: gen.New(db)}
}

// ListGroups returns the groups in display order.
func (r *GroupRepository) ListGroups(ctx context.Context) ([]models.Group, error) {
	rows, err := r.queries.ListGroups(ctx)
	if err != nil {
		return nil, repositoryError(err)
	}
	groups := make([]models.Group, 0, len(rows))
	for _, row := range rows {
		group, err := models.NewGroup(row.ID, row.Name, row.Color.String, row.Collapsed != 0)
		if err != nil {
			return nil, fmt.Errorf("%w: stored group %s: %w", gerrors.ErrRepository, row.ID, err)
		}
		groups = append(groups, group)
	}
	return groups, nil
}

// SaveGroup inserts a new group at the end of the list.
func (r *GroupRepository) SaveGroup(ctx context.Context, group models.Group) error {
	err := r.queries.InsertGroup(ctx, gen.InsertGroupParams{
		ID: group.ID(), Name: group.Name(), Color: optionalText(group.Color()), Collapsed: boolInt(group.Collapsed()),
	})
	if err != nil {
		return repositoryError(err)
	}
	return nil
}

// UpdateGroup changes the name, color and collapsed flag.
func (r *GroupRepository) UpdateGroup(ctx context.Context, group models.Group) error {
	changed, err := r.queries.UpdateGroup(ctx, gen.UpdateGroupParams{
		ID: group.ID(), Name: group.Name(), Color: optionalText(group.Color()), Collapsed: boolInt(group.Collapsed()),
	})
	return affected(changed, err)
}

// DeleteGroup removes the group; its memberships go with it, the workspaces stay.
func (r *GroupRepository) DeleteGroup(ctx context.Context, id string) error {
	changed, err := r.queries.DeleteGroup(ctx, id)
	return affected(changed, err)
}

// SetCollapsed stores only the open/closed state.
func (r *GroupRepository) SetCollapsed(ctx context.Context, id string, collapsed bool) error {
	changed, err := r.queries.SetGroupCollapsed(ctx, gen.SetGroupCollapsedParams{ID: id, Collapsed: boolInt(collapsed)})
	return affected(changed, err)
}

// Memberships maps each grouped workspace id to its group id.
func (r *GroupRepository) Memberships(ctx context.Context) (map[string]string, error) {
	rows, err := r.queries.ListMemberships(ctx)
	if err != nil {
		return nil, repositoryError(err)
	}
	members := make(map[string]string, len(rows))
	for _, row := range rows {
		members[row.WorkspaceID] = row.GroupID
	}
	return members, nil
}

// Assign puts the workspace in the group. The foreign keys tell a missing group from a missing workspace;
// the upsert and that check run in one transaction, so a concurrent delete cannot change the verdict.
func (r *GroupRepository) Assign(ctx context.Context, workspaceID string, groupID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return repositoryError(err)
	}
	defer func() { _ = tx.Rollback() }() //nolint:errcheck // Rollback after Commit reports ErrTxDone, which is expected.
	queries := r.queries.WithTx(tx)
	err = queries.UpsertMembership(ctx, gen.UpsertMembershipParams{WorkspaceID: workspaceID, GroupID: groupID})
	if err == nil {
		if err := tx.Commit(); err != nil {
			return repositoryError(err)
		}
		return nil
	}
	if !isForeignKey(err) {
		return repositoryError(err)
	}
	groups, listErr := queries.ListGroups(ctx)
	if listErr != nil {
		return repositoryError(listErr)
	}
	for _, group := range groups {
		if group.ID == groupID {
			return gerrors.ErrWorkspaceNotFound
		}
	}
	return gerrors.ErrGroupNotFound
}

// Unassign removes the workspace from its group.
func (r *GroupRepository) Unassign(ctx context.Context, workspaceID string) error {
	if err := r.queries.DeleteMembership(ctx, workspaceID); err != nil {
		return repositoryError(err)
	}
	return nil
}

// Pinned returns the pinned workspace ids in pin order.
func (r *GroupRepository) Pinned(ctx context.Context) ([]string, error) {
	ids, err := r.queries.ListPins(ctx)
	if err != nil {
		return nil, repositoryError(err)
	}
	return ids, nil
}

// Pin pins the workspace at the end of the pins; pinning twice keeps the first position.
func (r *GroupRepository) Pin(ctx context.Context, workspaceID string) error {
	if err := r.queries.InsertPin(ctx, workspaceID); err != nil {
		if isForeignKey(err) {
			return gerrors.ErrWorkspaceNotFound
		}
		return repositoryError(err)
	}
	return nil
}

// Unpin removes the pin.
func (r *GroupRepository) Unpin(ctx context.Context, workspaceID string) error {
	if err := r.queries.DeletePin(ctx, workspaceID); err != nil {
		return repositoryError(err)
	}
	return nil
}

func affected(changed int64, err error) error {
	if err != nil {
		return repositoryError(err)
	}
	if changed == 0 {
		return gerrors.ErrGroupNotFound
	}
	return nil
}

func optionalText(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func boolInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func isForeignKey(err error) bool {
	var driverErr *sqlite.Error
	return errors.As(err, &driverErr) && driverErr.Code() == sqliteConstraintForeignKey
}

func repositoryError(err error) error {
	return fmt.Errorf("%w: %w", gerrors.ErrRepository, err)
}
