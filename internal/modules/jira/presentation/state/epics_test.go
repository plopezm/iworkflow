package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/jira/presentation/state"
)

func TestParseBoardModeDefaultsToEpics(t *testing.T) {
	assert.Equal(t, state.BoardModeLanes, state.ParseBoardMode(" lanes "))
	assert.Equal(t, state.BoardModeEpics, state.ParseBoardMode(""))
	assert.Equal(t, state.BoardModeEpics, state.ParseBoardMode("nonsense"))
}

func TestEpicTitleFallsBackToTheKey(t *testing.T) {
	assert.Equal(t, "Checkout", state.EpicTitle(dto.EpicGroupDTO{Key: "A-1", Epic: &dto.IssueDTO{Summary: "Checkout"}}))
	assert.Equal(t, "A-1", state.EpicTitle(dto.EpicGroupDTO{Key: "A-1"}))
	assert.True(t, state.IsNoEpic(dto.EpicGroupDTO{Key: "none"}))
}

func TestProgressPercentRoundsDownAndHandlesEmptyEpics(t *testing.T) {
	assert.Equal(t, 0, state.ProgressPercent(dto.EpicGroupDTO{}))
	group := dto.EpicGroupDTO{Children: make([]dto.IssueDTO, 3), Done: 1}
	assert.Equal(t, 33, state.ProgressPercent(group))
}

func TestStatusVariantFollowsTheStatusName(t *testing.T) {
	cases := map[string]string{
		"Done":        "done",
		"Hecho":       "done",
		"In Progress": "working",
		"En curso":    "working",
		"In Review":   "review",
		"Blocked":     "attention",
		"To Do":       "backlog",
	}
	for status, want := range cases {
		assert.Equal(t, want, state.StatusVariant(status), status)
	}
}
