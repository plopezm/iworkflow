package application_test

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/workspace_groups/application"
	gerrors "iworkflow/internal/modules/workspace_groups/domain/errors"
	"iworkflow/internal/modules/workspace_groups/domain/models"
)

func TestCreateGroupStoresATrimmedGroupWithAGeneratedID(t *testing.T) {
	repo := newFakeRepo()
	created, err := application.NewCreateGroup(repo, &sequenceIDs{}).Execute(context.Background(), application.GroupInput{Name: " Backend ", Color: "blue"})

	require.NoError(t, err)
	assert.Equal(t, "g1", created.ID())
	assert.Equal(t, "Backend", created.Name())
	assert.Equal(t, []string{"g1"}, repo.groupIDs())
}

func TestCreateGroupRejectsInvalidInputAndStoresNothing(t *testing.T) {
	repo := newFakeRepo()
	create := application.NewCreateGroup(repo, &sequenceIDs{})
	for _, input := range []application.GroupInput{{Name: " "}, {Name: "a", Color: "nope"}} {
		_, err := create.Execute(context.Background(), input)
		require.ErrorIs(t, err, gerrors.ErrInvalidGroup)
	}
	assert.Empty(t, repo.groups)
}

func TestUpdateGroupRenamesAndRecolorsKeepingTheCollapsedState(t *testing.T) {
	repo := newFakeRepo()
	ctx := context.Background()
	_, err := application.NewCreateGroup(repo, &sequenceIDs{}).Execute(ctx, application.GroupInput{Name: "Old"})
	require.NoError(t, err)
	require.NoError(t, application.NewSetGroupCollapsed(repo).Execute(ctx, "g1", true))

	updated, err := application.NewUpdateGroup(repo).Execute(ctx, "g1", application.GroupInput{Name: "New", Color: "red"})

	require.NoError(t, err)
	assert.Equal(t, "New", updated.Name())
	assert.Equal(t, "red", updated.Color())
	assert.True(t, updated.Collapsed())
	_, err = application.NewUpdateGroup(repo).Execute(ctx, "ghost", application.GroupInput{Name: "x"})
	require.ErrorIs(t, err, gerrors.ErrGroupNotFound)
	_, err = application.NewUpdateGroup(repo).Execute(ctx, "g1", application.GroupInput{Name: ""})
	require.ErrorIs(t, err, gerrors.ErrInvalidGroup)
}

func TestDeleteGroupFreesItsWorkspaces(t *testing.T) {
	repo := newFakeRepo("w1")
	ctx := context.Background()
	_, err := application.NewCreateGroup(repo, &sequenceIDs{}).Execute(ctx, application.GroupInput{Name: "G"})
	require.NoError(t, err)
	require.NoError(t, application.NewMoveWorkspace(repo).Execute(ctx, "w1", "g1"))

	require.NoError(t, application.NewDeleteGroup(repo).Execute(ctx, "g1"))

	assert.Empty(t, repo.groups)
	assert.Empty(t, repo.members)
	require.ErrorIs(t, application.NewDeleteGroup(repo).Execute(ctx, "g1"), gerrors.ErrGroupNotFound)
}

func TestMoveWorkspaceAssignsReassignsAndEmptyGroupUnassigns(t *testing.T) {
	repo := newFakeRepo("w1")
	ctx := context.Background()
	create := application.NewCreateGroup(repo, &sequenceIDs{})
	_, _ = create.Execute(ctx, application.GroupInput{Name: "A"})
	_, _ = create.Execute(ctx, application.GroupInput{Name: "B"})
	move := application.NewMoveWorkspace(repo)

	require.NoError(t, move.Execute(ctx, "w1", "g1"))
	require.NoError(t, move.Execute(ctx, "w1", "g2"))
	assert.Equal(t, map[string]string{"w1": "g2"}, repo.members)
	require.NoError(t, move.Execute(ctx, "w1", ""))
	assert.Empty(t, repo.members)
	require.ErrorIs(t, move.Execute(ctx, "w1", "ghost"), gerrors.ErrGroupNotFound)
	require.ErrorIs(t, move.Execute(ctx, "ghost", "g1"), gerrors.ErrWorkspaceNotFound)
}

func TestSetPinnedPinsAndUnpins(t *testing.T) {
	repo := newFakeRepo("w1")
	ctx := context.Background()
	pin := application.NewSetPinned(repo)

	require.NoError(t, pin.Execute(ctx, "w1", true))
	assert.Equal(t, []string{"w1"}, repo.pins)
	require.NoError(t, pin.Execute(ctx, "w1", false))
	assert.Empty(t, repo.pins)
	require.ErrorIs(t, pin.Execute(ctx, "ghost", true), gerrors.ErrWorkspaceNotFound)
}

func TestSetGroupCollapsedReportsAMissingGroup(t *testing.T) {
	repo := newFakeRepo()
	require.ErrorIs(t, application.NewSetGroupCollapsed(repo).Execute(context.Background(), "ghost", true), gerrors.ErrGroupNotFound)
}

// repoState is what the fake repository stores. A failed write must leave it exactly as it was.
type repoState struct {
	groups  []models.Group
	members map[string]string
	pins    []string
}

func stateOf(repo *fakeRepo) repoState {
	members := make(map[string]string, len(repo.members))
	for workspace, group := range repo.members {
		members[workspace] = group
	}
	return repoState{groups: append([]models.Group(nil), repo.groups...), members: members, pins: append([]string(nil), repo.pins...)}
}

// TestStorageFailuresReportTheCauseAndLeaveTheStateUnchanged runs every write of the use cases against a failing
// repository: the error must wrap the storage cause and nothing may be stored.
func TestStorageFailuresReportTheCauseAndLeaveTheStateUnchanged(t *testing.T) {
	ctx := context.Background()
	failure := errors.New("disk full")
	seedGroup := func(t *testing.T, repo *fakeRepo) {
		repo.groups = []models.Group{group(t, "g1", "Old", true)}
	}
	seedMember := func(t *testing.T, repo *fakeRepo) {
		seedGroup(t, repo)
		repo.members = map[string]string{"w1": "g1"}
	}
	seedPin := func(_ *testing.T, repo *fakeRepo) { repo.pins = []string{"w1"} }
	noSeed := func(*testing.T, *fakeRepo) {}
	tests := []struct {
		name  string
		seed  func(t *testing.T, repo *fakeRepo)
		write func(repo *fakeRepo) error
	}{
		{name: "create group", seed: noSeed, write: func(repo *fakeRepo) error {
			_, err := application.NewCreateGroup(repo, &sequenceIDs{}).Execute(ctx, application.GroupInput{Name: "x"})
			return err
		}},
		{name: "update group", seed: seedGroup, write: func(repo *fakeRepo) error {
			_, err := application.NewUpdateGroup(repo).Execute(ctx, "g1", application.GroupInput{Name: "New", Color: "red"})
			return err
		}},
		{name: "delete group", seed: seedMember, write: func(repo *fakeRepo) error {
			return application.NewDeleteGroup(repo).Execute(ctx, "g1")
		}},
		{name: "collapse group", seed: seedGroup, write: func(repo *fakeRepo) error {
			return application.NewSetGroupCollapsed(repo).Execute(ctx, "g1", false)
		}},
		{name: "move workspace into a group", seed: seedGroup, write: func(repo *fakeRepo) error {
			return application.NewMoveWorkspace(repo).Execute(ctx, "w1", "g1")
		}},
		{name: "ungroup workspace", seed: seedMember, write: func(repo *fakeRepo) error {
			return application.NewMoveWorkspace(repo).Execute(ctx, "w1", "")
		}},
		{name: "pin workspace", seed: noSeed, write: func(repo *fakeRepo) error {
			return application.NewSetPinned(repo).Execute(ctx, "w1", true)
		}},
		{name: "unpin workspace", seed: seedPin, write: func(repo *fakeRepo) error {
			return application.NewSetPinned(repo).Execute(ctx, "w1", false)
		}},
		{name: "arrange workspaces", seed: seedMember, write: func(repo *fakeRepo) error {
			_, err := application.NewArrangeWorkspaces(repo).Execute(ctx, []string{"w1"})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepo("w1")
			tt.seed(t, repo)
			before := stateOf(repo)
			repo.failWith = failure

			require.ErrorIs(t, tt.write(repo), failure)
			assert.Equal(t, before, stateOf(repo))
		})
	}
}

// TestCreateGroupKeepsEveryIDUniqueBeyondNineGroups guards the generated ids: the tenth group is g10, not a rune.
func TestCreateGroupKeepsEveryIDUniqueBeyondNineGroups(t *testing.T) {
	repo := newFakeRepo()
	create := application.NewCreateGroup(repo, &sequenceIDs{})
	for range 10 {
		_, err := create.Execute(context.Background(), application.GroupInput{Name: "x"})
		require.NoError(t, err)
	}

	ids := repo.groupIDs()
	assert.Len(t, ids, 10)
	assert.Contains(t, ids, "g"+strconv.Itoa(10))
}

func group(t *testing.T, id string, name string, collapsed bool) models.Group {
	t.Helper()
	built, err := models.NewGroup(id, name, "", collapsed)
	require.NoError(t, err)
	return built
}

func TestArrangeSplitsPinnedGroupedAndUngroupedKeepingTheGivenOrder(t *testing.T) {
	repo := newFakeRepo("a", "b", "c", "d", "e")
	repo.groups = []models.Group{group(t, "g1", "One", false), group(t, "g2", "Two", true)}
	repo.members = map[string]string{"a": "g1", "c": "g1", "d": "g2", "e": "g1"}
	repo.pins = []string{"c", "gone"}

	layout, err := application.NewArrangeWorkspaces(repo).Execute(context.Background(), []string{"a", "b", "c", "d", "e"})

	require.NoError(t, err)
	assert.Equal(t, []string{"c"}, layout.Pinned, "a pin of a missing workspace is dropped")
	require.Len(t, layout.Groups, 2)
	assert.Equal(t, []string{"a", "e"}, layout.Groups[0].Workspaces, "a pinned workspace is not repeated in its group")
	assert.Equal(t, []string{"d"}, layout.Groups[1].Workspaces)
	assert.True(t, layout.Groups[1].Group.Collapsed())
	assert.Equal(t, []string{"b"}, layout.Ungrouped)
}

// TestArrangePinnedFollowsThePinOrderOfTheRepositoryAndDropsDuplicates: the pinned block lists the pins in the order the
// repository returns them, not in the order of the given ids, and a pin repeated by the repository shows once.
func TestArrangePinnedFollowsThePinOrderOfTheRepositoryAndDropsDuplicates(t *testing.T) {
	repo := newFakeRepo("a", "b", "c", "d")
	repo.pins = []string{"d", "b", "d"}

	layout, err := application.NewArrangeWorkspaces(repo).Execute(context.Background(), []string{"a", "b", "c", "d"})

	require.NoError(t, err)
	assert.Equal(t, []string{"d", "b"}, layout.Pinned, "pins keep the repository order, each once")
	assert.Equal(t, []string{"a", "c"}, layout.Ungrouped)
}

func TestArrangeKeepsEmptyGroupsAndTreatsAnUnknownGroupAsUngrouped(t *testing.T) {
	repo := newFakeRepo("a")
	repo.groups = []models.Group{group(t, "g1", "Empty", false)}
	repo.members = map[string]string{"a": "stale"}

	layout, err := application.NewArrangeWorkspaces(repo).Execute(context.Background(), []string{"a"})

	require.NoError(t, err)
	require.Len(t, layout.Groups, 1)
	assert.Empty(t, layout.Groups[0].Workspaces)
	assert.Equal(t, []string{"a"}, layout.Ungrouped)
	assert.Empty(t, layout.Pinned)
}
