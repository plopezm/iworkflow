package inbound_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/terminal/application"
	"iworkflow/internal/modules/terminal/domain/models"
	"iworkflow/internal/modules/terminal/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/terminal/infrastructure/adapters/outbound/resolver"
)

// spanishContext is the request context of the e2e scripts.
func spanishContext() context.Context {
	return i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})
}

// tabsFor builds the module's handlers over pty and returns their dock routes.
func tabsFor(pty *fakePty) []dock.Route {
	handlers := inbound.NewHandlers(inbound.UseCases{
		List:          application.NewListTerminals(pty),
		OpenWorkspace: application.NewOpenTerminal(pty, resolver.NewWorkspacePathAdapter(nothingFound)),
		OpenTask:      application.NewOpenTaskTerminal(pty, resolver.NewTaskPathAdapter(noTask)),
		OpenHome:      application.NewOpenHomeTerminal(pty, "/home/u"),
		Close:         application.NewCloseTerminal(pty),
	})
	return handlers.Tabs()
}

// routeNamed returns the dock route with the given name.
func routeNamed(t *testing.T, routes []dock.Route, name string) dock.Route {
	t.Helper()
	for _, route := range routes {
		if route.Name == name {
			return route
		}
	}
	require.FailNow(t, "no dock route named "+name)
	return dock.Route{}
}

// renderTab renders a component of a dock route with the given parameters.
func renderTab(t *testing.T, component templ.Component) string {
	t.Helper()
	var buffer bytes.Buffer
	require.NoError(t, component.Render(spanishContext(), &buffer))
	return buffer.String()
}

func TestTabsRegisterEveryTerminalPathInTheRegistry(t *testing.T) {
	registry, err := dock.NewRegistry(tabsFor(&fakePty{})...)
	require.NoError(t, err)

	for path, name := range map[string]string{
		"/terminals":              "terminals",
		"/terminals/abc":          "terminal_session",
		"/workspaces/w1/terminal": "workspace_terminal",
		"/tasks/t1/terminal":      "task_terminal",
	} {
		route, _, ok := registry.Match(path)
		require.True(t, ok, "no route for %s", path)
		require.Equal(t, name, route.Name, "wrong route for %s", path)
	}
}

func TestSessionTabNamesTheWorkspaceWhenTheSessionIsListed(t *testing.T) {
	pty := &fakePty{}
	workspaceID := uuid.New()
	session, err := pty.Spawn(context.Background(), workspaceID, "api", "/repos/api", models.TerminalSize{Cols: 80, Rows: 24})
	require.NoError(t, err)
	route := routeNamed(t, tabsFor(pty), "terminal_session")

	body := renderTab(t, route.Render(spanishContext(), dock.Params{"id": session.ID.String()}))

	require.Contains(t, body, "api · "+session.ID.String()[:8])
	require.Contains(t, body, `data-terminal-session="`+session.ID.String()+`"`)
}

func TestSessionTabFallsBackToTheShortIDWhenTheSessionIsUnknown(t *testing.T) {
	route := routeNamed(t, tabsFor(&fakePty{}), "terminal_session")

	body := renderTab(t, route.Render(spanishContext(), dock.Params{"id": "0123456789abcdef"}))

	require.Contains(t, body, "Terminal 01234567")
}

func TestSessionTabTitleIsTheWordTerminalPlusTheShortID(t *testing.T) {
	route := routeNamed(t, tabsFor(&fakePty{}), "terminal_session")

	require.Equal(t, "Terminal 01234567", route.Title(spanishContext(), dock.Params{"id": "0123456789abcdef"}))
}

func TestWorkspaceTabPostsOnLoadToItsOwnEndpoint(t *testing.T) {
	route := routeNamed(t, tabsFor(&fakePty{}), "workspace_terminal")

	body := renderTab(t, route.Render(spanishContext(), dock.Params{"id": "w1"}))

	require.Contains(t, body, `hx-post="/terminal/workspaces/w1/open"`)
	require.Contains(t, body, `hx-trigger="load"`)
	require.Contains(t, body, "Abriendo terminal")
}

func TestTaskTabPostsOnLoadToItsOwnEndpoint(t *testing.T) {
	route := routeNamed(t, tabsFor(&fakePty{}), "task_terminal")

	body := renderTab(t, route.Render(spanishContext(), dock.Params{"id": "t1"}))

	require.Contains(t, body, `hx-post="/terminal/tasks/t1/open"`)
}

func TestNewTerminalTabEscapesTheIDInTheEndpoint(t *testing.T) {
	route := routeNamed(t, tabsFor(&fakePty{}), "workspace_terminal")

	body := renderTab(t, route.Render(spanishContext(), dock.Params{"id": `a"b`}))

	require.NotContains(t, body, `hx-post="/terminal/workspaces/a"b/open"`)
	require.Contains(t, body, `hx-post="/terminal/workspaces/a%22b/open"`)
}

func TestTabTitlesFollowTheRequestLanguage(t *testing.T) {
	routes := tabsFor(&fakePty{})
	es := spanishContext()

	require.Equal(t, "Terminales", routeNamed(t, routes, "terminals").Title(es, nil))
	require.Equal(t, "Terminal", routeNamed(t, routes, "workspace_terminal").Title(es, dock.Params{}))
	require.Equal(t, "❯", routeNamed(t, routes, "terminals").Icon)
}

func TestTabsCarryTheSameIconAsTheRustVersion(t *testing.T) {
	for _, route := range tabsFor(&fakePty{}) {
		require.Equal(t, "❯", route.Icon, route.Name)
	}
}
