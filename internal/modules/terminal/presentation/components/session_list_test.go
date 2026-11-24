package components

import (
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/terminal/presentation/state"
)

func sessionRow(workspaceName string, workspaceColor string) state.SessionRow {
	return state.SessionRow{
		ID: "0123456789abcdef", ShortID: "01234567", WorkspaceName: workspaceName, WorkspaceColor: workspaceColor,
		StartedAt: "02/01/2026 03:04 UTC", StatusLabel: "Running", StatusClass: "badge badge-ok",
		Href: "/terminals/0123456789abcdef",
	}
}

func TestSessionsListMarksAColoredWorkspaceWithItsPill(t *testing.T) {
	body := render(t, SessionsList(state.ListView{Rows: []state.SessionRow{sessionRow("api", "blue")}}))

	// The style attribute sits between the two (it carries --ws for a custom color), so they are checked apart.
	require.Contains(t, body, `<span class="ws-name ws-blue"`)
	require.Contains(t, body, `data-workspace-color="blue"`)
	require.Contains(t, body, `<span class="min-w-0 truncate">api</span>`)
}

func TestSessionsListKeepsTheNameOfAnUncoloredWorkspaceAsItWas(t *testing.T) {
	body := render(t, SessionsList(state.ListView{Rows: []state.SessionRow{sessionRow("api", "")}}))

	require.Contains(t, body, `<td class="font-medium text-fg">api</td>`)
	require.NotContains(t, body, "ws-name")
	require.NotContains(t, body, "data-workspace-color")
}

func TestSessionsListKeepsTheNameOfAnUnknownColor(t *testing.T) {
	body := render(t, SessionsList(state.ListView{Rows: []state.SessionRow{sessionRow("api", "magenta")}}))

	require.Contains(t, body, `<td class="font-medium text-fg">api</td>`)
	require.NotContains(t, body, "ws-name")
}
