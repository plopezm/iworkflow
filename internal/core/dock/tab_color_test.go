package dock

import (
	"context"
	"net/url"
	"testing"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

func colorRouter(t *testing.T) chi.Router {
	t.Helper()
	page := func(context.Context, Params) templ.Component { return templ.Raw("<p>page</p>") }
	registry, err := NewRegistry(
		Route{Pattern: "/blue", Icon: "▦", Title: func(context.Context, Params) string { return "Blue" }, Render: page,
			Color: func(context.Context, Params) string { return "blue" }},
		Route{Pattern: "/custom", Icon: "▦", Title: func(context.Context, Params) string { return "Custom" }, Render: page,
			Color: func(context.Context, Params) string { return "#1a2b3c" }},
		Route{Pattern: "/plain", Icon: "◈", Title: func(context.Context, Params) string { return "Plain" }, Render: page},
	)
	require.NoError(t, err)
	router := chi.NewRouter()
	NewHandlers(registry, nil).Routes(router)
	return router
}

func restore(t *testing.T, path string) string {
	t.Helper()
	saved := New()
	saved.Open(path, "T", 0)
	data, err := saved.ToJSON()
	require.NoError(t, err)
	return post(t, colorRouter(t), "/dock/layout", url.Values{"layout": {data}}).Body.String()
}

func TestTabOfAColoredWorkspaceCarriesItsColorAndADot(t *testing.T) {
	body := restore(t, "/blue")

	require.Contains(t, body, "dock-tab-ws ws-blue")
	require.Contains(t, body, `<span class="ws-dot"></span>`)
	require.NotContains(t, body, "▦")
}

func TestTabOfACustomColorSetsTheColorVariable(t *testing.T) {
	body := restore(t, "/custom")

	require.Contains(t, body, "dock-tab-ws ws-custom")
	require.Contains(t, body, "--ws:#1a2b3c;")
}

func TestTabWithoutAColorKeepsItsIcon(t *testing.T) {
	body := restore(t, "/plain")

	require.NotContains(t, body, "dock-tab-ws")
	require.NotContains(t, body, "ws-dot")
	require.Contains(t, body, "◈")
}
