package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/terminal/presentation/state"
)

// render renders a component in the given language.
func render(t *testing.T, component templ.Component, locale i18n.Locale) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: locale, Chosen: true})
	var buffer bytes.Buffer
	require.NoError(t, component.Render(ctx, &buffer))
	return buffer.String()
}

// localized returns a request context in the given language, for the components that resolve their texts when built.
func localized(locale i18n.Locale) context.Context {
	return i18n.WithState(context.Background(), i18n.State{Locale: locale, Chosen: true})
}

func TestTerminalsPageOffersTheHomeTerminalAndRefresh(t *testing.T) {
	body := render(t, TerminalsPage(state.ListView{}), i18n.Es)

	require.Contains(t, body, "Nueva terminal en $HOME")
	require.Contains(t, body, `hx-post="/terminal/home"`)
	require.Contains(t, body, `hx-target="#terminal-open-status"`)
	require.Contains(t, body, "Actualizar")
	require.Contains(t, body, `hx-get="/terminal/sessions"`)
	require.Contains(t, body, `<h1 class="page-title">Terminales</h1>`)
}

func TestTerminalsPageIsTranslatedToEnglish(t *testing.T) {
	body := render(t, TerminalsPage(state.ListView{}), i18n.En)

	require.Contains(t, body, "New terminal in $HOME")
	require.Contains(t, body, "No open terminals.")
}

func TestSessionPageShowsTheTitleCloseAndTheTerminal(t *testing.T) {
	body := render(t, SessionPage(state.SessionPage{ID: "s1", Title: "api · 01234567"}), i18n.Es)

	require.Contains(t, body, "api · 01234567")
	require.Contains(t, body, "Cerrar sesión")
	require.Contains(t, body, `hx-post="/terminal/sessions/s1/close/page"`)
	require.Contains(t, body, `href="/terminals"`)
	require.Contains(t, body, `data-terminal-session="s1"`)
	require.Contains(t, body, `id="terminal-session-status"`)
}

func TestSessionTitleIsEscaped(t *testing.T) {
	body := render(t, SessionPage(state.SessionPage{ID: "s1", Title: "<b>x</b>"}), i18n.Es)

	require.NotContains(t, body, "<b>x</b>")
	require.Contains(t, body, "&lt;b&gt;x&lt;/b&gt;")
}

func TestOpeningPlaceholderPostsOnLoad(t *testing.T) {
	body := render(t, Opening(localized(i18n.Es), "/terminal/workspaces/w1/open"), i18n.Es)

	require.Contains(t, body, `hx-post="/terminal/workspaces/w1/open"`)
	require.Contains(t, body, `hx-trigger="load"`)
	require.Contains(t, body, "Abriendo terminal…")
	require.Contains(t, body, "Abriendo terminal</h2>")
}

func TestOpenFailedShowsTheMessageAndTheWayBack(t *testing.T) {
	body := render(t, OpenFailed(localized(i18n.Es), "el workspace no existe", BackToWorkspaces()), i18n.Es)

	require.Contains(t, body, "el workspace no existe")
	require.Contains(t, body, `href="/workspaces"`)
	require.Contains(t, body, "Volver a workspaces")
}

func TestOpenFailedForATaskGoesBackToTheBoard(t *testing.T) {
	body := render(t, OpenFailed(localized(i18n.En), "x", BackToBoard()), i18n.En)

	require.True(t, strings.Contains(body, `href="/"`), body)
	require.Contains(t, body, "Back to board")
}
