package workspace_groups_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/modules/workspace_groups/application"
	gerrors "iworkflow/internal/modules/workspace_groups/domain/errors"
	"iworkflow/internal/modules/workspace_groups/domain/models"
	"iworkflow/internal/modules/workspace_groups/infrastructure/adapters/outbound/persistence"
)

func newRepo(t *testing.T) (*persistence.GroupRepository, *sql.DB) {
	t.Helper()
	db := dbtest.New(t)
	return persistence.NewGroupRepository(db), db
}

func insertWorkspace(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO workspaces (id, name, path, base_branch, created_at) VALUES (?, ?, ?, 'main', '2026-01-01 00:00:00')`, id, id, "/tmp/"+id)
	require.NoError(t, err)
}

func group(t *testing.T, id string, name string, color string, collapsed bool) models.Group {
	t.Helper()
	built, err := models.NewGroup(id, name, color, collapsed)
	require.NoError(t, err)
	return built
}

func TestGroupsListInCreationOrderAndRoundTripColorAndCollapsed(t *testing.T) {
	ctx := context.Background()
	repo, _ := newRepo(t)
	first := group(t, "b", "Backend", "blue", true)
	second := group(t, "a", "Alpha", "", false)
	require.NoError(t, repo.SaveGroup(ctx, first))
	require.NoError(t, repo.SaveGroup(ctx, second))

	list, err := repo.ListGroups(ctx)

	require.NoError(t, err)
	require.Equal(t, []models.Group{first, second}, list)
}

func TestUpdateAndCollapseReportAMissingGroup(t *testing.T) {
	ctx := context.Background()
	repo, _ := newRepo(t)
	require.ErrorIs(t, repo.UpdateGroup(ctx, group(t, "x", "X", "", false)), gerrors.ErrGroupNotFound)
	require.ErrorIs(t, repo.SetCollapsed(ctx, "x", true), gerrors.ErrGroupNotFound)
	require.ErrorIs(t, repo.DeleteGroup(ctx, "x"), gerrors.ErrGroupNotFound)

	require.NoError(t, repo.SaveGroup(ctx, group(t, "g", "G", "red", false)))
	require.NoError(t, repo.SetCollapsed(ctx, "g", true))
	require.NoError(t, repo.UpdateGroup(ctx, group(t, "g", "Renamed", "", true)))
	list, err := repo.ListGroups(ctx)
	require.NoError(t, err)
	require.Equal(t, []models.Group{group(t, "g", "Renamed", "", true)}, list)
}

func TestUpdateGroupLeavesTheStoredCollapsedStateAlone(t *testing.T) {
	ctx := context.Background()
	repo, _ := newRepo(t)
	require.NoError(t, repo.SaveGroup(ctx, group(t, "g", "G", "red", false)))
	require.NoError(t, repo.SetCollapsed(ctx, "g", true))
	// The update carries a stale collapsed=false, as a concurrent collapse would leave it.
	require.NoError(t, repo.UpdateGroup(ctx, group(t, "g", "Renamed", "blue", false)))
	list, err := repo.ListGroups(ctx)
	require.NoError(t, err)
	require.Equal(t, []models.Group{group(t, "g", "Renamed", "blue", true)}, list)
}

func TestAssignMovesAWorkspaceBetweenGroupsAndUnassignFreesIt(t *testing.T) {
	ctx := context.Background()
	repo, db := newRepo(t)
	insertWorkspace(t, db, "w1")
	require.NoError(t, repo.SaveGroup(ctx, group(t, "g1", "One", "", false)))
	require.NoError(t, repo.SaveGroup(ctx, group(t, "g2", "Two", "", false)))

	require.NoError(t, repo.Assign(ctx, "w1", "g1"))
	require.NoError(t, repo.Assign(ctx, "w1", "g2"))
	members, err := repo.Memberships(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"w1": "g2"}, members)

	require.NoError(t, repo.Unassign(ctx, "w1"))
	require.NoError(t, repo.Unassign(ctx, "w1"), "unassigning twice is not an error")
	members, err = repo.Memberships(ctx)
	require.NoError(t, err)
	require.Empty(t, members)
}

func TestAssignRejectsAMissingWorkspaceOrGroupAndLeavesNoRow(t *testing.T) {
	ctx := context.Background()
	repo, db := newRepo(t)
	insertWorkspace(t, db, "w1")
	require.NoError(t, repo.SaveGroup(ctx, group(t, "g1", "One", "", false)))

	require.ErrorIs(t, repo.Assign(ctx, "ghost", "g1"), gerrors.ErrWorkspaceNotFound)
	require.ErrorIs(t, repo.Assign(ctx, "w1", "ghost"), gerrors.ErrGroupNotFound)

	members, err := repo.Memberships(ctx)
	require.NoError(t, err)
	require.Empty(t, members)
}

func TestDeletingAGroupKeepsItsWorkspacesAndDropsTheMembership(t *testing.T) {
	ctx := context.Background()
	repo, db := newRepo(t)
	insertWorkspace(t, db, "w1")
	require.NoError(t, repo.SaveGroup(ctx, group(t, "g1", "One", "", false)))
	require.NoError(t, repo.Assign(ctx, "w1", "g1"))

	require.NoError(t, repo.DeleteGroup(ctx, "g1"))

	members, err := repo.Memberships(ctx)
	require.NoError(t, err)
	require.Empty(t, members)
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM workspaces`).Scan(&count))
	require.Equal(t, 1, count)
}

func TestPinsKeepTheirOrderIgnoreDuplicatesAndFollowTheWorkspace(t *testing.T) {
	ctx := context.Background()
	repo, db := newRepo(t)
	insertWorkspace(t, db, "a")
	insertWorkspace(t, db, "b")

	require.NoError(t, repo.Pin(ctx, "b"))
	require.NoError(t, repo.Pin(ctx, "a"))
	require.NoError(t, repo.Pin(ctx, "b"))
	pinned, err := repo.Pinned(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"b", "a"}, pinned)
	require.ErrorIs(t, repo.Pin(ctx, "ghost"), gerrors.ErrWorkspaceNotFound)

	require.NoError(t, repo.Unpin(ctx, "b"))
	require.NoError(t, repo.Unpin(ctx, "b"))
	_, err = db.Exec(`DELETE FROM workspaces WHERE id = 'a'`)
	require.NoError(t, err)
	pinned, err = repo.Pinned(ctx)
	require.NoError(t, err)
	require.Empty(t, pinned, "deleting a workspace drops its pin")
}

func groupInput(name string) application.GroupInput { return application.GroupInput{Name: name} }

func TestDeletingAGroupedWorkspaceDropsItsMembershipButKeepsTheGroup(t *testing.T) {
	ctx := context.Background()
	repo, db := newRepo(t)
	insertWorkspace(t, db, "w1")
	require.NoError(t, repo.SaveGroup(ctx, group(t, "g1", "One", "", false)))
	require.NoError(t, repo.Assign(ctx, "w1", "g1"))

	_, err := db.Exec(`DELETE FROM workspaces WHERE id = 'w1'`)
	require.NoError(t, err)

	members, err := repo.Memberships(ctx)
	require.NoError(t, err)
	require.Empty(t, members)
	groups, err := repo.ListGroups(ctx)
	require.NoError(t, err)
	require.Len(t, groups, 1)
}

func TestAStoredGroupThatBreaksAnInvariantIsReportedAsAStorageFailure(t *testing.T) {
	ctx := context.Background()
	repo, db := newRepo(t)
	_, err := db.Exec(`INSERT INTO workspace_groups (id, name, color, collapsed, position) VALUES ('bad', 'x', 'chartreuse', 0, 1)`)
	require.NoError(t, err)

	_, err = repo.ListGroups(ctx)

	require.ErrorIs(t, err, gerrors.ErrRepository)
	require.ErrorIs(t, err, gerrors.ErrInvalidGroup)
	require.ErrorContains(t, err, "bad")
}
