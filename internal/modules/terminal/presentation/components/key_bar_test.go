package components

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/terminal/presentation/state"
)

// render renders a component in Spanish, the language the e2e scripts run in.
func render(t *testing.T, component templ.Component) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})
	var buffer bytes.Buffer
	require.NoError(t, component.Render(ctx, &buffer))
	return buffer.String()
}

func TestKeyBarOffersTheNineKeysInOrder(t *testing.T) {
	labels := make([]string, 0, len(terminalKeys))
	keys := make([]string, 0, len(terminalKeys))
	for _, key := range terminalKeys {
		labels = append(labels, key.Label)
		keys = append(keys, key.Key)
	}

	require.Equal(t, []string{"Esc", "Tab", "Up", "Left", "Right", "Down", "Ctrl+C", "Copy", "Paste"}, labels)
	require.Equal(t, []string{"esc", "tab", "up", "left", "right", "down", "ctrl_c", "copy", "paste"}, keys)
}

func TestKeyBarRendersOneButtonPerKeyWithItsAccessibleName(t *testing.T) {
	body := render(t, KeyBar())

	require.Equal(t, len(terminalKeys), strings.Count(body, "<button"))
	require.Contains(t, body, `aria-label="Ctrl+C"`)
	require.Contains(t, body, `data-terminal-key="ctrl_c"`)
	require.Contains(t, body, `aria-label="Teclas especiales"`)
}

func TestTerminalKeysAreHandledByTheTerminalScript(t *testing.T) {
	// The test runs in the package directory; the repository root is five levels up.
	content, err := os.ReadFile("../../../../../web/static/terminal.js")
	require.NoError(t, err)
	script := string(content)

	for _, key := range terminalKeys {
		require.Contains(t, script, key.Key+":", "terminal.js does not define the key %q", key.Key)
	}
}

func TestTerminalViewMountsTheSessionAndIncludesTheKeyBar(t *testing.T) {
	body := render(t, TerminalView("s1"))

	require.Contains(t, body, `data-terminal-session="s1"`)
	require.Contains(t, body, `data-terminal-view`)
	require.Contains(t, body, `src="/static/js/terminal_view.js"`)
	require.Contains(t, body, `role="toolbar"`)
}

func TestSessionsListShowsTheEmptyMessageWithoutRows(t *testing.T) {
	body := render(t, SessionsList(state.ListView{}))

	require.Contains(t, body, "No hay terminales abiertas.")
	require.NotContains(t, body, "<table")
}

func TestSessionsListRendersTheRowsWithItsActions(t *testing.T) {
	body := render(t, SessionsList(state.ListView{Rows: []state.SessionRow{{
		ID: "0123456789abcdef", ShortID: "01234567", WorkspaceName: "api",
		StartedAt: "02/01/2026 03:04 UTC", StatusLabel: "Running", StatusClass: "badge badge-ok",
		Href: "/terminals/0123456789abcdef",
	}}}))

	require.Contains(t, body, "api")
	require.Contains(t, body, `hx-post="/terminal/sessions/0123456789abcdef/close"`)
	require.Contains(t, body, `hx-target="#terminal-sessions"`)
	require.Contains(t, body, `href="/terminals/0123456789abcdef"`)
	require.Contains(t, body, "Cerrar")
}

func TestSessionsListShowsTheErrorsInsteadOfTheTable(t *testing.T) {
	body := render(t, SessionsList(state.ListView{LoadError: "sin acceso"}))
	require.Contains(t, body, "sin acceso")
	require.NotContains(t, body, "<table")

	action := render(t, SessionsList(state.ListView{ActionError: "cierre fallido"}))
	require.Contains(t, action, "cierre fallido")
	require.Contains(t, action, "No hay terminales abiertas.")
}

func TestDockRedirectCarriesThePathAsAnAttribute(t *testing.T) {
	body := render(t, DockRedirect("/terminals/s1"))

	require.Equal(t, `<span data-dock-redirect="/terminals/s1"></span>`, body)
}
