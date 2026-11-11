package components_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/jira/presentation/components"
	"iworkflow/internal/modules/jira/presentation/state"
)

// renderEnglish draws a component in English and returns its HTML.
func renderEnglish(t *testing.T, component templ.Component) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.En, Chosen: true})
	var out bytes.Buffer
	require.NoError(t, component.Render(ctx, &out))
	return out.String()
}

// epicBoardGroups is an Epic with two children and an Epic with one child, both imported.
func epicBoardGroups() []dto.EpicGroupDTO {
	firstEpic := dto.IssueDTO{Key: "PROJ-1", Summary: "Checkout", Status: "In Progress", URL: "https://jira.example/browse/PROJ-1"}
	secondEpic := dto.IssueDTO{Key: "PROJ-2", Summary: "Search", Status: "To Do"}
	return []dto.EpicGroupDTO{
		{
			Key:    "PROJ-1",
			Epic:   &firstEpic,
			Done:   1,
			Counts: []dto.StatusCountDTO{{Status: "Done", Count: 1}, {Status: "To Do", Count: 1}},
			Children: []dto.IssueDTO{
				{Key: "PROJ-11", Summary: "Cart", Status: "Done", IssueType: "Story", JiraUpdatedAt: "2026-11-10T10:00:00Z"},
				{Key: "PROJ-12", Summary: "Payment", Status: "To Do", IssueType: "Task", JiraUpdatedAt: "2026-11-10T10:00:00Z"},
			},
		},
		{
			Key:      "PROJ-2",
			Epic:     &secondEpic,
			Counts:   []dto.StatusCountDTO{{Status: "In Review", Count: 1}},
			Children: []dto.IssueDTO{{Key: "PROJ-21", Summary: "Index", Status: "In Review", IssueType: "Task", JiraUpdatedAt: "2026-11-10T10:00:00Z"}},
		},
	}
}

func TestIssuesPanelEpicsModeRendersMasterDetail(t *testing.T) {
	view := state.IssuesPanelView{
		Mode:         state.BoardModeEpics,
		Epics:        epicBoardGroups(),
		SelectedEpic: "PROJ-1",
		Total:        3,
	}

	html := renderEnglish(t, components.IssuesPanel(view))

	require.Contains(t, html, `id="jira-board"`)
	require.Contains(t, html, `hx-include="#jira-filters"`, "sync keeps the filters")
	require.Contains(t, html, "PROJ-1")
	require.Contains(t, html, "PROJ-2")
	require.Contains(t, html, `role="progressbar"`)
	require.Contains(t, html, `aria-current="true"`)

	// Selected epic: its children are listed with a status control that loads the transitions.
	require.Contains(t, html, `id="status-PROJ-11"`)
	require.Contains(t, html, `hx-get="/jira/epics/PROJ-1/issues/PROJ-11/transitions"`)
	require.Contains(t, html, `hx-target="#status-PROJ-11"`)
	// Another epic's children stay out of the detail panel until that epic is opened.
	require.NotContains(t, html, `id="status-PROJ-21"`)

	// Mode switch: Epics is pressed, Lanes is offered with the filters kept.
	require.Contains(t, html, `aria-pressed="true"`)
	require.Contains(t, html, `hx-get="/jira/board?mode=lanes"`)
	// The lane-only controls stay out of the Epics mode.
	require.NotContains(t, html, `/jira/lanes/new`)
}

func TestIssuesPanelEpicsModeEmptyStates(t *testing.T) {
	tests := []struct {
		name string
		view state.IssuesPanelView
		want string
	}{
		{
			name: "no imported issues asks for a sync",
			view: state.IssuesPanelView{Mode: state.BoardModeEpics, Total: 0},
			want: "No issues imported yet",
		},
		{
			name: "filters hide every epic",
			view: state.IssuesPanelView{Mode: state.BoardModeEpics, Total: 3},
			want: "No epics match the filters",
		},
		{
			name: "no epic open shows a hint",
			view: state.IssuesPanelView{Mode: state.BoardModeEpics, Total: 3, Epics: epicBoardGroups()},
			want: "Select an epic to see its issues.",
		},
		{
			name: "epic without children",
			view: state.IssuesPanelView{
				Mode:         state.BoardModeEpics,
				Total:        1,
				Epics:        []dto.EpicGroupDTO{{Key: "PROJ-9", Epic: &dto.IssueDTO{Key: "PROJ-9", Summary: "Empty"}}},
				SelectedEpic: "PROJ-9",
			},
			want: "This epic has no issues yet.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			html := renderEnglish(t, components.IssuesPanel(tc.view))
			require.Contains(t, html, tc.want)
		})
	}
}

func TestIssuesPanelLanesModeKeepsLaneControls(t *testing.T) {
	view := state.IssuesPanelView{Mode: state.BoardModeLanes, Total: 0}

	html := renderEnglish(t, components.IssuesPanel(view))

	require.Contains(t, html, `hx-get="/jira/lanes/new"`)
	require.Contains(t, html, `aria-pressed="false"`)
	require.NotContains(t, html, `role="progressbar"`)
}

func TestEpicStatusMenu(t *testing.T) {
	tests := []struct {
		name     string
		menu     state.StatusMenuView
		contains []string
		absent   []string
	}{
		{
			name: "lists the allowed transitions as a form",
			menu: state.StatusMenuView{
				EpicKey:     "PROJ-1",
				IssueKey:    "PROJ-11",
				Status:      "To Do",
				Transitions: []dto.TransitionDTO{{ID: "21", Name: "Start", ToStatus: "In Progress"}},
			},
			contains: []string{
				`id="status-PROJ-11"`,
				`hx-post="/jira/epics/PROJ-1/issues/PROJ-11/transition"`,
				`name="transition_id" value="21"`,
				"Start",
				"→ In Progress",
				`hx-target="#jira-board"`,
				`hx-get="/jira/board?mode=epics&amp;epic=PROJ-1"`,
			},
		},
		{
			name: "no transitions shows a muted message and no form",
			menu: state.StatusMenuView{EpicKey: "PROJ-1", IssueKey: "PROJ-11", Status: "Done"},
			contains: []string{
				`id="status-PROJ-11"`,
				"No status changes available.",
			},
			absent: []string{`hx-post=`, `name="transition_id"`},
		},
		{
			name: "a failed lookup is shown as an error",
			menu: state.StatusMenuView{EpicKey: "PROJ-1", IssueKey: "PROJ-11", Status: "To Do", Err: "Jira unreachable"},
			contains: []string{
				"Jira unreachable",
				"text-error",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			html := renderEnglish(t, components.EpicStatusMenu(tc.menu))
			for _, want := range tc.contains {
				require.Contains(t, html, want)
			}
			for _, unwanted := range tc.absent {
				require.NotContains(t, html, unwanted)
			}
		})
	}
}
