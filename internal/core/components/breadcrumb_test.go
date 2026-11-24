package components_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/components"
)

func TestBreadcrumbLinksEveryStepButTheLastOne(t *testing.T) {
	html := renderHTML(t, components.Breadcrumb("You are here", []components.Crumb{
		{Label: "Workspaces", Href: "/workspaces"},
		{Label: "Groups", Href: "/workspaces/groups"},
		{Label: "Family"},
	}))

	require.Contains(t, html, `aria-label="You are here"`)
	require.Contains(t, html, `data-dock-path="/workspaces"`)
	require.Contains(t, html, `data-dock-path="/workspaces/groups"`)
	require.Contains(t, html, `aria-current="page">Family</span>`)
	require.Equal(t, 2, countOf(html, "&gt;"), "one separator between each pair of steps")
}

func countOf(haystack string, needle string) int { return strings.Count(haystack, needle) }
