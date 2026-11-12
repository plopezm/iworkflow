package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/jira/presentation/state"
)

func TestTaskPanelViewAddressesItsOwnRootAndActions(t *testing.T) {
	t.Parallel()
	view := state.TaskPanelView{TaskID: "t1"}

	assert.Equal(t, "jira-task-panel-t1", view.RootID())
	assert.Equal(t, "/jira/tasks/t1/worklog", view.Path("worklog"))
}

func TestTaskPanelViewBannerNeedsALinkedFinishedTaskWithTimePending(t *testing.T) {
	t.Parallel()
	issue := &dto.IssueDTO{Key: "B-1"}
	tests := []struct {
		name string
		view state.TaskPanelView
		want bool
	}{
		{"finished, linked and pending", state.TaskPanelView{Issue: issue, Finished: true, Pending: true}, true},
		{"still running", state.TaskPanelView{Issue: issue, Finished: false, Pending: true}, false},
		{"nothing pending", state.TaskPanelView{Issue: issue, Finished: true, Pending: false}, false},
		{"not linked", state.TaskPanelView{Finished: true, Pending: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.view.ShowBanner())
		})
	}
}

func TestTaskPanelViewListsTheLatestThreeCommentsOldestFirst(t *testing.T) {
	t.Parallel()
	view := state.TaskPanelView{}
	assert.Empty(t, view.LatestComments())

	view.Comments = []dto.CommentDTO{{Author: "a"}, {Author: "b"}}
	assert.Equal(t, view.Comments, view.LatestComments())

	view.Comments = []dto.CommentDTO{{Author: "a"}, {Author: "b"}, {Author: "c"}, {Author: "d"}}
	got := view.LatestComments()
	if assert.Len(t, got, 3) {
		assert.Equal(t, "b", got[0].Author)
		assert.Equal(t, "d", got[2].Author)
	}
}

func TestTaskPanelViewHasLoggedFollowsTheTotal(t *testing.T) {
	t.Parallel()
	assert.False(t, state.TaskPanelView{}.HasLogged())
	assert.True(t, state.TaskPanelView{Logged: "1h"}.HasLogged())
}
