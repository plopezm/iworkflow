package dock

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
)

func routeFixture(name, pattern string) Route {
	return Route{
		Name:    name,
		Pattern: pattern,
		Icon:    "◇",
		Title: func(_ context.Context, params Params) string {
			return name + ":" + params["id"]
		},
		Render: func(_ context.Context, params Params) templ.Component {
			return templ.Raw("<p>" + name + "/" + params["id"] + "</p>")
		},
	}
}

func newRegistry(t *testing.T, routes ...Route) *Registry {
	t.Helper()
	registry, err := NewRegistry(routes...)
	require.NoError(t, err)
	return registry
}

func TestMatchCapturesPathSegments(t *testing.T) {
	registry := newRegistry(t,
		routeFixture("task", "/tasks/{id}"),
		routeFixture("edit", "/tasks/{id}/edit"),
	)

	route, params, ok := registry.Match("/tasks/a1")
	require.True(t, ok)
	require.Equal(t, "task", route.Name)
	require.Equal(t, "a1", params["id"])

	route, params, ok = registry.Match("/tasks/a1/edit")
	require.True(t, ok)
	require.Equal(t, "edit", route.Name)
	require.Equal(t, "a1", params["id"])
}

func TestMatchRejectsWrongShapes(t *testing.T) {
	registry := newRegistry(t, routeFixture("task", "/tasks/{id}"))
	for _, path := range []string{"/tasks", "/tasks/a1/extra", "/nope/a1", "/tasks//"} {
		t.Run(path, func(t *testing.T) {
			route, _, ok := registry.Match(path)
			require.False(t, ok)
			require.Empty(t, route.Name, "falls back to the not-found route")
		})
	}
}

func TestRootPatternMatchesTheEmptyPath(t *testing.T) {
	registry := newRegistry(t, routeFixture("board", "/"), routeFixture("board-named", "/board"))

	route, _, ok := registry.Match("/")
	require.True(t, ok)
	require.Equal(t, "board", route.Name)

	route, _, ok = registry.Match("/board")
	require.True(t, ok)
	require.Equal(t, "board-named", route.Name)
}

func TestRestCaptureTakesTheDecodedRemainder(t *testing.T) {
	registry := newRegistry(t, routeFixture("file", "/workspaces/{id}/files/{path...}"))

	_, params, ok := registry.Match("/workspaces/w/files/src/a%20b/main.rs")
	require.True(t, ok)
	require.Equal(t, "w", params["id"])
	require.Equal(t, "src/a b/main.rs", params["path"])

	_, _, ok = registry.Match("/workspaces/w/files")
	require.False(t, ok, "the rest capture needs at least one segment")
}

func TestDecodePathKeepsInvalidEscapesAsWritten(t *testing.T) {
	cases := map[string]string{
		"a%20b":   "a b",
		"100%":    "100%",
		"%zz":     "%zz",
		"a%2Fb":   "a/b",
		"ñ%C3%B1": "ññ",
	}
	for encoded, want := range cases {
		require.Equal(t, want, decodePath(encoded), encoded)
	}
}

func TestNewRegistryRejectsBadPatterns(t *testing.T) {
	cases := map[string]Route{
		"rest not last":  {Pattern: "/a/{rest...}/b"},
		"empty capture":  {Pattern: "/a/{}"},
		"empty rest":     {Pattern: "/a/{...}"},
		"duplicate name": {Name: "x", Pattern: "/a"},
	}
	for name, route := range cases {
		t.Run(name, func(t *testing.T) {
			routes := []Route{route}
			if name == "duplicate name" {
				routes = append(routes, Route{Name: "x", Pattern: "/b"})
			}
			_, err := NewRegistry(routes...)
			require.Error(t, err)
		})
	}
}

func TestReuseByNameIgnoresEmptyNamesAndUnmatchedPaths(t *testing.T) {
	registry := newRegistry(t, routeFixture("board", "/board/{id}"), routeFixture("", "/misc"))

	require.True(t, registry.MatchesName("/board/w1", "board"))
	require.False(t, registry.MatchesName("/misc", ""), "an empty name never reuses")
	require.False(t, registry.MatchesName("/unknown", "board"))
}

func TestTitleIconAndRenderFallBackToNotFound(t *testing.T) {
	registry := newRegistry(t, routeFixture("task", "/tasks/{id}"))
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})

	require.Equal(t, "task:a1", registry.Title(ctx, "/tasks/a1"))
	require.Equal(t, "◇", registry.Icon("/tasks/a1"))
	require.Equal(t, "No encontrada", registry.Title(ctx, "/nope"))
	require.Equal(t, "?", registry.Icon("/nope"))

	var out bytes.Buffer
	require.NoError(t, registry.Render(ctx, "/nope").Render(ctx, &out))
	require.Contains(t, out.String(), "Página no encontrada.")
}
