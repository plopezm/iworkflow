package state

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/terminal/infrastructure/adapters/inbound/dto"
)

// spanish returns a request context in Spanish, the language the e2e scripts run in.
func spanish() context.Context {
	return i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})
}

func TestShortIDTakesThePrefix(t *testing.T) {
	require.Equal(t, "01234567", ShortID("0123456789abcdef"))
	require.Equal(t, "abc", ShortID("abc"))
}

func TestStartedAtLabelIsHumanizedOrFallsBack(t *testing.T) {
	require.Equal(t, "05/10/2026 18:14 UTC", StartedAtLabel("2026-10-05T18:14:42+00:00"))
	require.Equal(t, "ayer", StartedAtLabel("ayer"))
}

func TestStatusLabelAndClassFollowTheStatus(t *testing.T) {
	require.Equal(t, "Running", StatusLabel(spanish(), dto.TerminalStatusRunningDTO))
	require.Equal(t, "Exited", StatusLabel(spanish(), dto.TerminalStatusExitedDTO))
	require.Equal(t, "badge badge-ok", StatusClass(dto.TerminalStatusRunningDTO))
	require.Equal(t, "badge badge-muted", StatusClass(dto.TerminalStatusExitedDTO))
}

func TestSessionTitleUsesTheWorkspaceNameOrFallsBackToTheShortID(t *testing.T) {
	require.Equal(t, "api · 01234567", SessionTitle(spanish(), "api", "0123456789abcdef"))
	require.Equal(t, "Terminal 01234567", SessionTitle(spanish(), "", "0123456789abcdef"))
}

func TestTabTitleIsTheWordTerminalPlusTheShortID(t *testing.T) {
	require.Equal(t, "Terminal 01234567", TabTitle(spanish(), "0123456789abcdef"))
}

func TestSessionPathBuildsTheAddress(t *testing.T) {
	require.Equal(t, "/terminals", TerminalsPath)
	require.Equal(t, "/terminals/s1", SessionPath("s1"))
}

func TestNewListViewMapsSessionsToRows(t *testing.T) {
	sessions := []dto.TerminalSessionDTO{{
		ID:            "0123456789abcdef",
		WorkspaceName: "api",
		CreatedAt:     "2026-01-02T03:04:05+00:00",
		Status:        dto.TerminalStatusExitedDTO,
	}}

	view := NewListView(spanish(), sessions, "", "")

	require.Equal(t, []SessionRow{{
		ID:            "0123456789abcdef",
		ShortID:       "01234567",
		WorkspaceName: "api",
		StartedAt:     "02/01/2026 03:04 UTC",
		StatusLabel:   "Exited",
		StatusClass:   "badge badge-muted",
		Href:          "/terminals/0123456789abcdef",
	}}, view.Rows)
	require.Empty(t, view.ActionError)
	require.Empty(t, view.LoadError)
}

func TestNewListViewCarriesTheErrorsWithoutRows(t *testing.T) {
	view := NewListView(spanish(), nil, "", "fallo")

	require.Equal(t, "fallo", view.LoadError)
	require.Empty(t, view.Rows)

	withAction := NewListView(spanish(), nil, "cierre fallido", "")
	require.Equal(t, "cierre fallido", withAction.ActionError)
}
