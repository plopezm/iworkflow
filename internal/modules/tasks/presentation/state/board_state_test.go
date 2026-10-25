package state_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// task builds a DTO with the fields the board reads.
func task(id string, stage string, priority string, updated string) dto.TaskDTO {
	return dto.TaskDTO{ID: id, WorkspaceID: "w1", Title: "t-" + id, Stage: stage, Priority: priority, UpdatedAt: updated}
}

func TestGroupByStageKeepsEveryStageInOrderEvenWhenEmpty(t *testing.T) {
	groups := state.GroupByStage(nil)

	require.Len(t, groups, 5)
	stages := make([]string, 0, len(groups))
	for _, group := range groups {
		stages = append(stages, group.Stage)
		assert.Empty(t, group.Tasks)
	}
	assert.Equal(t, []string{"backlog", "working", "attention", "review", "done"}, stages)
}

func TestGroupByStagePlacesEveryTaskInItsStage(t *testing.T) {
	tasks := []dto.TaskDTO{
		task("a", "backlog", "normal", "2026-01-01T00:00:00+00:00"),
		task("b", "working", "normal", "2026-01-01T00:00:00+00:00"),
		task("c", "attention", "normal", "2026-01-01T00:00:00+00:00"),
		task("d", "review", "normal", "2026-01-01T00:00:00+00:00"),
		task("e", "done", "normal", "2026-01-01T00:00:00+00:00"),
		task("f", "working", "normal", "2026-01-01T00:00:00+00:00"),
	}

	groups := state.GroupByStage(tasks)

	ids := make(map[string][]string)
	for _, group := range groups {
		for _, member := range group.Tasks {
			assert.Equal(t, group.Stage, member.Stage, "task %s is in the wrong column", member.ID)
			ids[group.Stage] = append(ids[group.Stage], member.ID)
		}
	}
	assert.Equal(t, []string{"a"}, ids["backlog"])
	assert.ElementsMatch(t, []string{"b", "f"}, ids["working"])
	assert.Equal(t, []string{"c"}, ids["attention"])
	assert.Equal(t, []string{"d"}, ids["review"])
	assert.Equal(t, []string{"e"}, ids["done"])
}

func TestGroupByStageOrdersByPriorityThenUpdateThenId(t *testing.T) {
	tasks := []dto.TaskDTO{
		task("low-new", "backlog", "low", "2026-03-01T00:00:00+00:00"),
		task("urgent-old", "backlog", "urgent", "2026-01-01T00:00:00+00:00"),
		task("normal-old", "backlog", "normal", "2026-01-01T00:00:00+00:00"),
		task("normal-new", "backlog", "normal", "2026-02-01T00:00:00+00:00"),
		task("normal-tie-b", "backlog", "normal", "2026-02-01T00:00:00+00:00"),
		task("high", "backlog", "high", "2025-01-01T00:00:00+00:00"),
	}

	backlog := state.GroupByStage(tasks)[0].Tasks

	order := make([]string, 0, len(backlog))
	for _, member := range backlog {
		order = append(order, member.ID)
	}
	assert.Equal(t, []string{"urgent-old", "high", "normal-new", "normal-tie-b", "normal-old", "low-new"}, order)
}

func TestGroupByStageDoesNotReorderTheCallersSlice(t *testing.T) {
	tasks := []dto.TaskDTO{
		task("low", "backlog", "low", "2026-01-01T00:00:00+00:00"),
		task("urgent", "backlog", "urgent", "2026-01-01T00:00:00+00:00"),
	}

	state.GroupByStage(tasks)

	assert.Equal(t, "low", tasks[0].ID)
}

func TestFilterWorkspaceKeepsOnlyTheSelectedWorkspace(t *testing.T) {
	tasks := []dto.TaskDTO{
		{ID: "a", WorkspaceID: "w1"},
		{ID: "b", WorkspaceID: "w2"},
		{ID: "c", WorkspaceID: "w1"},
	}

	kept := state.FilterWorkspace(tasks, "w1")

	assert.Equal(t, []string{"a", "c"}, []string{kept[0].ID, kept[1].ID})
	assert.Len(t, state.FilterWorkspace(tasks, "unknown"), 0)
}

func TestFilterWorkspaceWithoutAnIdentifierKeepsEverything(t *testing.T) {
	tasks := []dto.TaskDTO{{ID: "a", WorkspaceID: "w1"}, {ID: "b", WorkspaceID: "w2"}}

	assert.Len(t, state.FilterWorkspace(tasks, ""), 2)
}

func TestCombineTasksPrefersTheActiveCopyAndDropsRepeats(t *testing.T) {
	active := []dto.TaskDTO{{ID: "a", Stage: "working"}, {ID: "b", Stage: "backlog"}}
	done := []dto.TaskDTO{{ID: "b", Stage: "done"}, {ID: "c", Stage: "done"}, {ID: "c", Stage: "done"}}

	combined := state.CombineTasks(active, done)

	require.Len(t, combined, 3)
	assert.Equal(t, "backlog", combined[1].Stage, "the active copy of b wins")
	assert.Equal(t, "c", combined[2].ID)
}

func TestDonePagingHelpers(t *testing.T) {
	assert.Equal(t, int64(0), state.DoneOffset(0))
	assert.Equal(t, int64(50), state.DoneOffset(1))
	assert.Equal(t, int64(0), state.DoneOffset(-3), "a negative page never asks for a negative offset")

	assert.False(t, state.DoneHasMore(-1), "no page loaded means no more")
	assert.False(t, state.DoneHasMore(49))
	assert.True(t, state.DoneHasMore(50), "a full last page may have more after it")
}

func TestClampPagesKeepsAtLeastOneAndAtMostTwenty(t *testing.T) {
	assert.Equal(t, 1, state.ClampPages(0))
	assert.Equal(t, 3, state.ClampPages(3))
	assert.Equal(t, 20, state.ClampPages(500))
}

// fakePages serves pages of the given sizes; every row has its own identifier.
func fakePages(sizes ...int) func(context.Context, int) ([]dto.TaskDTO, error) {
	return func(_ context.Context, page int) ([]dto.TaskDTO, error) {
		if page >= len(sizes) {
			return nil, nil
		}
		rows := make([]dto.TaskDTO, 0, sizes[page])
		for index := range sizes[page] {
			rows = append(rows, dto.TaskDTO{ID: fmt.Sprintf("p%d-%d", page, index)})
		}
		return rows, nil
	}
}

func TestCollectDonePagesStopsAtTheFirstShortPage(t *testing.T) {
	calls := 0
	fetch := func(ctx context.Context, page int) ([]dto.TaskDTO, error) {
		calls++
		return fakePages(3, 50)(ctx, page)
	}

	result, err := state.CollectDonePages(context.Background(), 4, fetch)

	require.NoError(t, err)
	assert.Equal(t, 1, calls, "a short first page ends the walk")
	assert.Len(t, result.Tasks, 3)
	assert.Equal(t, 3, result.LastPageLen)
	assert.False(t, state.DoneHasMore(result.LastPageLen))
}

func TestCollectDonePagesReportsMoreWhenTheLastPageIsFull(t *testing.T) {
	result, err := state.CollectDonePages(context.Background(), 2, fakePages(50, 50))

	require.NoError(t, err)
	assert.Len(t, result.Tasks, 100)
	assert.True(t, state.DoneHasMore(result.LastPageLen))
}

func TestCollectDonePagesWithNoPagesRequestedHasNoPage(t *testing.T) {
	result, err := state.CollectDonePages(context.Background(), 0, fakePages())

	require.NoError(t, err)
	assert.Empty(t, result.Tasks)
	assert.Equal(t, -1, result.LastPageLen)
	assert.False(t, state.DoneHasMore(result.LastPageLen))
}

func TestCollectDonePagesDropsRowsRepeatedAcrossPages(t *testing.T) {
	fetch := func(_ context.Context, page int) ([]dto.TaskDTO, error) {
		if page == 0 {
			full := make([]dto.TaskDTO, 0, 50)
			for index := range 50 {
				full = append(full, dto.TaskDTO{ID: fmt.Sprintf("row-%d", index)})
			}
			return full, nil
		}
		return []dto.TaskDTO{{ID: "row-49"}, {ID: "fresh"}}, nil
	}

	result, err := state.CollectDonePages(context.Background(), 2, fetch)

	require.NoError(t, err)
	assert.Len(t, result.Tasks, 51, "row-49 repeated across pages is kept once")
	assert.Equal(t, 2, result.LastPageLen, "the length is the raw count of the last page")
}

func TestCollectDonePagesPropagatesErrors(t *testing.T) {
	boom := errors.New("database is locked")
	fetch := func(context.Context, int) ([]dto.TaskDTO, error) { return nil, boom }

	_, err := state.CollectDonePages(context.Background(), 1, fetch)

	require.ErrorIs(t, err, boom)
}

func TestChooseWorkspaceShowsTheFirstByNameOnTheRootAddress(t *testing.T) {
	workspaces := []dto.WorkspaceOptionDTO{
		{ID: "w2", Name: "zeta"},
		{ID: "w1", Name: "Alpha"},
	}

	choice := state.ChooseWorkspace("", workspaces)

	assert.Equal(t, state.ChoiceSelected, choice.Kind)
	assert.Equal(t, "w1", choice.ID)
	assert.Equal(t, "Alpha", choice.Name)
	assert.Equal(t, "zeta", workspaces[0].Name, "the caller's list keeps its order")
}

func TestChooseWorkspaceShowsTheRequestedOneOrMissing(t *testing.T) {
	workspaces := []dto.WorkspaceOptionDTO{{ID: "w1", Name: "Alpha"}, {ID: "w2", Name: "Beta"}}

	selected := state.ChooseWorkspace("w2", workspaces)
	missing := state.ChooseWorkspace("gone", workspaces)

	assert.Equal(t, state.WorkspaceChoice{Kind: state.ChoiceSelected, ID: "w2", Name: "Beta"}, selected)
	assert.Equal(t, state.WorkspaceChoice{Kind: state.ChoiceMissing, ID: "gone"}, missing)
}

func TestChooseWorkspaceIsEmptyWithoutWorkspaces(t *testing.T) {
	assert.Equal(t, state.ChoiceEmpty, state.ChooseWorkspace("", nil).Kind)
	assert.Equal(t, state.ChoiceEmpty, state.ChooseWorkspace("w1", nil).Kind, "no workspaces wins over an address")
}

func TestPathsOfTheBoardAndTheTask(t *testing.T) {
	assert.Equal(t, "/board/w1", state.BoardPath("w1"))
	assert.Equal(t, "/tasks/t1", state.TaskPath("t1"))
}
