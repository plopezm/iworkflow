package link_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/dock/link"
)

func TestPathDropsTheFragment(t *testing.T) {
	require.Equal(t, "/", link.Path("/#lane-1"))
	require.Equal(t, "/tasks/a", link.Path("/tasks/a"))
}

func TestTabRendersTheDockContractAttributes(t *testing.T) {
	props := link.Props{Href: "/#lane-1", Class: "menu-item", Title: "Board", Beside: true, Reuse: "board"}
	var out bytes.Buffer
	ctx := templ.WithChildren(context.Background(), templ.Raw("Board"))
	require.NoError(t, link.Tab(props).Render(ctx, &out))

	html := out.String()
	require.Contains(t, html, `href="/#lane-1"`)
	require.Contains(t, html, `data-dock-link`)
	require.Contains(t, html, `data-dock-path="/"`)
	require.Contains(t, html, `data-dock-title="Board"`)
	require.Contains(t, html, `data-dock-beside="true"`)
	require.Contains(t, html, `data-dock-reuse="board"`)
	require.Contains(t, html, `>Board</a>`)
}

func TestTabOmitsOptionalAttributesWhenEmpty(t *testing.T) {
	var out bytes.Buffer
	ctx := templ.WithChildren(context.Background(), templ.Raw("x"))
	require.NoError(t, link.Tab(link.Props{Href: "/jira"}).Render(ctx, &out))

	html := out.String()
	require.NotContains(t, html, "data-dock-title")
	require.NotContains(t, html, "data-dock-beside")
	require.NotContains(t, html, "data-dock-reuse")
}
