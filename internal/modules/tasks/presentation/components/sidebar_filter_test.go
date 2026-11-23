package components_test

import (
	"html"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

var rootIncludePattern = regexp.MustCompile(`hx-include="([^"]*)"`)

// openingTag is the start tag of the element with the given id, from its "<" to its first ">".
func openingTag(t *testing.T, html string, id string) string {
	t.Helper()
	start := strings.Index(html, `id="`+id+`"`)
	require.GreaterOrEqual(t, start, 0, "the element %s must be rendered", id)
	tag := html[start:]
	return tag[:strings.Index(tag, ">")]
}

func TestSidebarSearchInputHasAStableIdAndSearchesTheSidebar(t *testing.T) {
	t.Parallel()

	html := renderComponent(t, i18n.En, components.SidebarBody(groupedRows()))

	input := openingTag(t, html, "board-sidebar-filter")
	assert.Contains(t, input, `type="search"`)
	assert.Contains(t, input, `name="q"`)
	assert.Contains(t, input, `hx-get="/board/sidebar"`)
	assert.Contains(t, input, `hx-target="#board-sidebar"`)
	assert.Contains(t, input, `hx-swap="morph:innerHTML"`, "the typed text is kept by a morph, not a replacement")
	assert.Contains(t, input, `hx-sync="#board-sidebar:replace"`)
	assert.Contains(t, input, `delay:200ms`)
	assert.Contains(t, input, `aria-label="Search workspaces"`)
	assert.Contains(t, input, `placeholder="Search workspaces"`)
}

func TestBoardSidebarRootKeepsTheMorphSwapAndIncludesTheQuery(t *testing.T) {
	t.Parallel()

	html := renderComponent(t, i18n.En, components.BoardSidebar(groupedRows()))

	root := openingTag(t, html, "board-sidebar")
	assert.Contains(t, root, `hx-swap="morph:innerHTML"`)
	include := rootIncludePattern.FindStringSubmatch(root)
	require.Len(t, include, 2, "the root must carry hx-include so every refresh keeps the query")
	assert.Equal(t, "[name='q']", html_unescape(include[1]))
}

func TestSidebarSearchKeepsTheQueryInTheInput(t *testing.T) {
	t.Parallel()
	rows := state.FilterSidebar(groupedRows(), "api")

	html := renderComponent(t, i18n.En, components.SidebarBody(rows))

	assert.Contains(t, openingTag(t, html, "board-sidebar-filter"), `value="api"`, "a refresh re-renders the typed text")
}

func TestSidebarFilteredRowsShowTheMatchesAndTheirGroupExpanded(t *testing.T) {
	t.Parallel()
	rows := state.FilterSidebar(groupedRows(), "api")

	html := renderComponent(t, i18n.En, components.SidebarBody(rows))

	assert.Contains(t, html, `data-workspace-row="w1"`, "the match inside the collapsed group is shown")
	assert.Contains(t, html, `data-group-collapsed="false"`, "the matching group is shown expanded")
	assert.NotContains(t, html, `data-workspace-row="w2"`, "an ungrouped row that does not match is hidden")
	assert.NotContains(t, html, `data-workspace-row="p1"`, "a pinned row that does not match is hidden")
	assert.Contains(t, html, `<span class="sidebar-count">1</span>`, "the count of the section is the number of matches")
}

func TestSidebarWithAQueryWithoutMatchesShowsTheEmptyLine(t *testing.T) {
	t.Parallel()
	rows := state.FilterSidebar(groupedRows(), "zzz")

	html := renderComponent(t, i18n.En, components.SidebarBody(rows))

	assert.Contains(t, html, "No workspaces match.")
	assert.NotContains(t, html, "No workspaces yet.")
	assert.NotContains(t, html, `data-workspace-row=`)
}

func TestSidebarWithoutAQueryRendersTheSameRowsAsAnUnfilteredSidebar(t *testing.T) {
	t.Parallel()
	rows := groupedRows()

	plain := renderComponent(t, i18n.En, components.SidebarBody(rows))
	blank := renderComponent(t, i18n.En, components.SidebarBody(state.FilterSidebar(rows, "   ")))

	assert.Equal(t, plain, blank)
	assert.NotContains(t, plain, "No workspaces match.")
	assert.Contains(t, plain, `data-group-collapsed="true"`, "the stored state of a group is rendered as it is")
}

// groupToggleTag is the opening tag of the toggle button of the group with the given id.
func groupToggleTag(t *testing.T, html string, id string) string {
	t.Helper()
	start := strings.Index(html, `data-group-id="`+id+`"`)
	require.GreaterOrEqual(t, start, 0, "the group %s must be rendered", id)
	rest := html[start:]
	button := rest[strings.Index(rest, "<button"):]
	return button[:strings.Index(button, ">")+1]
}

// While a filter is active the matching groups are shown expanded whatever their stored state, so their toggle must
// not change that state: it renders disabled and posts nothing.
func TestSidebarGroupToggleIsDisabledWhileFiltering(t *testing.T) {
	t.Parallel()
	rows := state.FilterSidebar(groupedRows(), "api")

	html := renderComponent(t, i18n.En, components.SidebarBody(rows))

	toggle := groupToggleTag(t, html, "g1")
	assert.Contains(t, toggle, " disabled", "the toggle must be disabled while the filter forces the group open")
	assert.Contains(t, toggle, `aria-disabled="true"`)
	assert.NotContains(t, toggle, `hx-post=`, "a disabled toggle must not change the stored state")
	assert.NotContains(t, html, `hx-post="/workspace-groups/g1/collapse"`)
}

// Without a filter the toggle of a group keeps posting its collapse change.
func TestSidebarGroupToggleIsEnabledWithoutAFilter(t *testing.T) {
	t.Parallel()

	html := renderComponent(t, i18n.En, components.SidebarBody(groupedRows()))

	toggle := groupToggleTag(t, html, "g1")
	assert.NotContains(t, toggle, "disabled")
	assert.NotContains(t, toggle, `aria-disabled`)
	assert.Contains(t, toggle, `hx-post="/workspace-groups/g1/collapse"`)
}

// html_unescape decodes the entities templ writes into an attribute value.
func html_unescape(value string) string { return html.UnescapeString(value) }
