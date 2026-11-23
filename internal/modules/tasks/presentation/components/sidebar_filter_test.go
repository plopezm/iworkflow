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

var (
	rootIncludePattern = regexp.MustCompile(`hx-include="([^"]*)"`)
	filterValuePattern = regexp.MustCompile(`(id="board-sidebar-filter"[^>]*?) value="[^"]*"`)
)

// openingTag is the start tag of the element with the given id, from its "<" to its first ">".
func openingTag(t *testing.T, body string, id string) string {
	t.Helper()
	start := strings.Index(body, `id="`+id+`"`)
	require.GreaterOrEqual(t, start, 0, "the element %s must be rendered", id)
	tag := body[start:]
	return tag[:strings.Index(tag, ">")]
}

// withoutFilterValue drops the typed text of the search input, to compare the rows of two renders.
func withoutFilterValue(body string) string {
	return filterValuePattern.ReplaceAllString(body, "$1")
}

func TestSidebarSearchInputHasAStableIdAndSearchesTheSidebar(t *testing.T) {
	t.Parallel()

	body := renderComponent(t, i18n.En, components.SidebarBody(groupedRows()))

	input := openingTag(t, body, "board-sidebar-filter")
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

	body := renderComponent(t, i18n.En, components.BoardSidebar(groupedRows()))

	root := openingTag(t, body, "board-sidebar")
	assert.Contains(t, root, `hx-swap="morph:innerHTML"`)
	include := rootIncludePattern.FindStringSubmatch(root)
	require.Len(t, include, 2, "the root must carry hx-include so every refresh keeps the query")
	assert.Equal(t, "[name='q']", html.UnescapeString(include[1]))
}

// The root's hx-include is inherited by every element below it, so each group and workspace action would send the
// query too. The root disinherits it: only the root's own refresh includes the query.
func TestBoardSidebarRootDoesNotPassTheQueryDownToItsActions(t *testing.T) {
	t.Parallel()

	body := renderComponent(t, i18n.En, components.BoardSidebar(groupedRows()))

	assert.Contains(t, openingTag(t, body, "board-sidebar"), `hx-disinherit="hx-include"`)
	assert.Equal(t, 1, strings.Count(body, " hx-include="), "only the root carries hx-include; the actions below inherit nothing")
}

func TestSidebarSearchKeepsTheQueryInTheInput(t *testing.T) {
	t.Parallel()
	rows := state.FilterSidebar(groupedRows(), "api")

	body := renderComponent(t, i18n.En, components.SidebarBody(rows))

	assert.Contains(t, openingTag(t, body, "board-sidebar-filter"), `value="api"`, "a refresh re-renders the typed text")
}

// The input echoes the query as the user typed it, a trailing space included, while the matching trims it.
func TestSidebarSearchEchoesTheTypedQueryAndStillMatchesTrimmed(t *testing.T) {
	t.Parallel()
	rows := state.FilterSidebar(groupedRows(), "api ")

	body := renderComponent(t, i18n.En, components.SidebarBody(rows))

	assert.Contains(t, openingTag(t, body, "board-sidebar-filter"), ` value="api "`, "the trailing space the user typed stays in the input")
	assert.Contains(t, body, `data-workspace-row="w1"`, "the match is still computed from the trimmed query")
}

func TestSidebarFilteredRowsShowTheMatchesAndTheirGroupExpanded(t *testing.T) {
	t.Parallel()
	rows := state.FilterSidebar(groupedRows(), "api")

	body := renderComponent(t, i18n.En, components.SidebarBody(rows))

	assert.Contains(t, body, `data-workspace-row="w1"`, "the match inside the collapsed group is shown")
	assert.Contains(t, body, `data-group-collapsed="false"`, "the matching group is shown expanded")
	assert.NotContains(t, body, `data-workspace-row="w2"`, "an ungrouped row that does not match is hidden")
	assert.NotContains(t, body, `data-workspace-row="p1"`, "a pinned row that does not match is hidden")
	assert.Contains(t, body, `<span class="sidebar-count">1</span>`, "the count of the section is the number of matches")
}

func TestSidebarWithAQueryWithoutMatchesShowsTheEmptyLine(t *testing.T) {
	t.Parallel()
	rows := state.FilterSidebar(groupedRows(), "zzz")

	body := renderComponent(t, i18n.En, components.SidebarBody(rows))

	assert.Contains(t, body, "No workspaces match.")
	assert.NotContains(t, body, "No workspaces yet.")
	assert.NotContains(t, body, `data-workspace-row=`)
}

func TestSidebarWithoutAQueryRendersTheSameRowsAsAnUnfilteredSidebar(t *testing.T) {
	t.Parallel()
	rows := groupedRows()

	plain := renderComponent(t, i18n.En, components.SidebarBody(rows))
	blank := renderComponent(t, i18n.En, components.SidebarBody(state.FilterSidebar(rows, "   ")))

	assert.Equal(t, withoutFilterValue(plain), withoutFilterValue(blank))
	assert.Contains(t, openingTag(t, blank, "board-sidebar-filter"), ` value="   "`, "the input keeps what the user typed")
	assert.NotContains(t, plain, "No workspaces match.")
	assert.Contains(t, plain, `data-group-collapsed="true"`, "the stored state of a group is rendered as it is")
}

// groupToggleTag is the opening tag of the toggle button of the group with the given id.
func groupToggleTag(t *testing.T, body string, id string) string {
	t.Helper()
	start := strings.Index(body, `data-group-id="`+id+`"`)
	require.GreaterOrEqual(t, start, 0, "the group %s must be rendered", id)
	rest := body[start:]
	button := rest[strings.Index(rest, "<button"):]
	return button[:strings.Index(button, ">")+1]
}

// While a filter is active the matching groups are shown expanded whatever their stored state, so their toggle must
// not change that state: it renders disabled and posts nothing.
func TestSidebarGroupToggleIsDisabledWhileFiltering(t *testing.T) {
	t.Parallel()
	rows := state.FilterSidebar(groupedRows(), "api")

	body := renderComponent(t, i18n.En, components.SidebarBody(rows))

	toggle := groupToggleTag(t, body, "g1")
	assert.Contains(t, toggle, " disabled", "the toggle must be disabled while the filter forces the group open")
	assert.Contains(t, toggle, `aria-disabled="true"`)
	assert.NotContains(t, toggle, `hx-post=`, "a disabled toggle must not change the stored state")
	assert.NotContains(t, body, `hx-post="/workspace-groups/g1/collapse"`)
}

// Without a filter the toggle of a group keeps posting its collapse change.
func TestSidebarGroupToggleIsEnabledWithoutAFilter(t *testing.T) {
	t.Parallel()

	body := renderComponent(t, i18n.En, components.SidebarBody(groupedRows()))

	toggle := groupToggleTag(t, body, "g1")
	assert.NotContains(t, toggle, "disabled")
	assert.NotContains(t, toggle, `aria-disabled`)
	assert.Contains(t, toggle, `hx-post="/workspace-groups/g1/collapse"`)
}

// A query of spaces only is no filter: the toggles stay enabled.
func TestSidebarGroupToggleStaysEnabledForAWhitespaceOnlyQuery(t *testing.T) {
	t.Parallel()
	rows := state.FilterSidebar(groupedRows(), " \t ")

	body := renderComponent(t, i18n.En, components.SidebarBody(rows))

	toggle := groupToggleTag(t, body, "g1")
	assert.NotContains(t, toggle, "disabled")
	assert.Contains(t, toggle, `hx-post="/workspace-groups/g1/collapse"`)
}
