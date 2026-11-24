package application

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	memerrors "iworkflow/internal/modules/memories/domain/errors"
	"iworkflow/internal/modules/memories/domain/models"
)

func at(hour int) time.Time { return time.Date(2026, 1, 1, hour, 0, 0, 0, time.UTC) }

func TestListTaskMemoriesSplitsAtTheTaskStartKeepingBothGroups(t *testing.T) {
	feed := models.Feed{State: models.StateAvailable, Memories: []models.Memory{
		{ID: 4, Title: "new b", UpdatedAt: at(14)},
		{ID: 3, Title: "new a", UpdatedAt: at(12)}, // exactly at the start counts as the branch's
		{ID: 2, Title: "old", UpdatedAt: at(9)},
		{ID: 1, Title: "older", UpdatedAt: at(8)},
	}}
	tasks := fakeTasks{known: map[string]models.TaskRef{"t1": {ID: "t1", WorkspaceID: "w1", Branch: "iwf/x", Started: true, StartedAt: at(12)}}}
	uc := NewListTaskMemories(tasks, known(), fakeProvider{feed: feed})

	task, workspace, got, err := uc.Execute(context.Background(), "t1")

	require.NoError(t, err)
	assert.Equal(t, "iwf/x", task.Branch)
	assert.Equal(t, "proj", workspace.Name)
	assert.Equal(t, []int{4, 3}, ids(got.Branch))
	assert.Equal(t, []int{2, 1}, ids(got.Earlier))
}

func TestListTaskMemoriesBacklogTaskHasNoBranchMemories(t *testing.T) {
	feed := models.Feed{State: models.StateAvailable, Memories: []models.Memory{{ID: 1, UpdatedAt: at(20)}}}
	tasks := fakeTasks{known: map[string]models.TaskRef{"t1": {ID: "t1", WorkspaceID: "w1"}}}

	_, _, got, err := NewListTaskMemories(tasks, known(), fakeProvider{feed: feed}).Execute(context.Background(), "t1")

	require.NoError(t, err)
	assert.Empty(t, got.Branch)
	assert.Equal(t, []int{1}, ids(got.Earlier))
}

func TestListTaskMemoriesErrorsAndUnavailable(t *testing.T) {
	tasks := fakeTasks{known: map[string]models.TaskRef{"t1": {ID: "t1", WorkspaceID: "w1"}, "t2": {ID: "t2", WorkspaceID: "gone"}}}

	_, _, _, err := NewListTaskMemories(tasks, known(), nil).Execute(context.Background(), "zz")
	assert.ErrorIs(t, err, memerrors.ErrTaskNotFound)

	_, _, _, err = NewListTaskMemories(tasks, known(), nil).Execute(context.Background(), "t2")
	assert.ErrorIs(t, err, memerrors.ErrWorkspaceNotFound)

	_, _, got, err := NewListTaskMemories(tasks, known(), nil).Execute(context.Background(), "t1")
	require.NoError(t, err)
	assert.Equal(t, models.StateUnavailable, got.State)
}

func ids(memories []models.Memory) []int {
	out := make([]int, 0, len(memories))
	for _, memory := range memories {
		out = append(out, memory.ID)
	}
	return out
}
