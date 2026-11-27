package components_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/jira/presentation/components"
	"iworkflow/internal/modules/jira/presentation/state"
)

func TestTheBoardResyncsWithJiraOnItsOwnWithAMorphSwap(t *testing.T) {
	for _, mode := range []state.BoardMode{state.BoardModeLanes, state.BoardModeEpics} {
		html := renderEnglish(t, components.IssuesPanel(state.IssuesPanelView{Mode: mode}))

		assert.Contains(t, html, `hx-post="/jira/sync"`, mode)
		assert.Contains(t, html, `hx-trigger="every 300s"`, mode)
		assert.Contains(t, html, `hx-swap="morph:outerHTML"`, mode)
		assert.Contains(t, html, `hx-sync="this:replace"`, mode)
		assert.Contains(t, html, `data-jira-autosync`, mode)
	}
}

func TestTheBoardDoesNotAutoSyncWhileTheLaneFormIsOpen(t *testing.T) {
	for _, mode := range []state.LaneFormMode{state.LaneFormCreating, state.LaneFormEditing} {
		view := state.IssuesPanelView{Mode: state.BoardModeLanes, LaneForm: state.LaneFormView{Mode: mode}}

		html := renderEnglish(t, components.IssuesPanel(view))

		assert.NotContains(t, html, `data-jira-autosync`)
		assert.NotContains(t, html, `every 300s`)
	}
}
