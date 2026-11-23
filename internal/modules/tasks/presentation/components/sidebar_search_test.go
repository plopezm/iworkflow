package components_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

const (
	searchPanelID  = "board-sidebar-search"
	searchToggleID = "board-sidebar-search-toggle"
)

// hiddenAttr matches the bare `hidden` attribute of a tag, not a class or an aria attribute that contains the word.
var hiddenAttr = regexp.MustCompile(`\shidden[\s>]`)

func TestSidebarSearchIsHiddenByDefault(t *testing.T) {
	t.Parallel()

	body := renderComponent(t, i18n.En, components.SidebarBody(groupedRows()))

	panel := openingTag(t, body, searchPanelID)
	assert.Regexp(t, hiddenAttr, panel, "the search wrapper is closed until the icon is pressed")
	assert.Contains(t, panel, `data-search-panel`)
	assert.Contains(t, body, `id="board-sidebar-filter"`, "the input is still rendered, inside the closed wrapper")
}

func TestSidebarSearchIsVisibleWhileAQueryIsActive(t *testing.T) {
	t.Parallel()

	for _, query := range []string{"api", "api "} {
		rows := state.FilterSidebar(groupedRows(), query)

		body := renderComponent(t, i18n.En, components.SidebarBody(rows))

		assert.NotRegexp(t, hiddenAttr, openingTag(t, body, searchPanelID), "query %q forces the search open", query)
	}
}

// A query of spaces only narrows nothing, so it renders the search closed, as no query does.
func TestSidebarSearchStaysClosedForASpacesOnlyQuery(t *testing.T) {
	t.Parallel()

	rows := state.FilterSidebar(groupedRows(), "   ")

	body := renderComponent(t, i18n.En, components.SidebarBody(rows))

	assert.Regexp(t, hiddenAttr, openingTag(t, body, searchPanelID))
	assert.Contains(t, openingTag(t, body, searchToggleID), `aria-expanded="false"`)
}

func TestSidebarSearchToggleControlsThePanelAndReportsItsState(t *testing.T) {
	t.Parallel()

	closed := renderComponent(t, i18n.En, components.SidebarBody(groupedRows()))

	toggle := openingTag(t, closed, searchToggleID)
	assert.Contains(t, toggle, `aria-controls="board-sidebar-search"`, "the button names the wrapper it shows")
	assert.Contains(t, toggle, `aria-expanded="false"`)
	assert.Contains(t, toggle, `data-search-toggle`)
	assert.Contains(t, toggle, `type="button"`)
	assert.Contains(t, toggle, `aria-label="Show workspace search"`)
	assert.Contains(t, toggle, `title="Show workspace search"`)

	open := renderComponent(t, i18n.En, components.SidebarBody(state.FilterSidebar(groupedRows(), "api")))

	assert.Contains(t, openingTag(t, open, searchToggleID), `aria-expanded="true"`, "an active query shows the search open, so the icon says so")
}

func TestSidebarSearchToggleIsAnIconInTheWorkspacesHeaderBeforeTheNewGroupButton(t *testing.T) {
	t.Parallel()

	body := renderComponent(t, i18n.En, components.SidebarBody(groupedRows()))

	toggleAt := strings.Index(body, `id="board-sidebar-search-toggle"`)
	require.GreaterOrEqual(t, toggleAt, 0, "the toggle must be rendered")
	plusAt := strings.Index(body, `aria-label="New group"`)
	require.GreaterOrEqual(t, plusAt, 0, "the new group button must be rendered")
	assert.Less(t, toggleAt, plusAt, "the search icon sits left of the + button, in the header actions")

	toggleEnd := strings.Index(body[toggleAt:], "</button>")
	require.GreaterOrEqual(t, toggleEnd, 0)
	assert.Contains(t, body[toggleAt:toggleAt+toggleEnd], "<svg", "the icon is an inline SVG")
}

func TestSidebarSearchStillRendersTheSameFilterInput(t *testing.T) {
	t.Parallel()

	body := renderComponent(t, i18n.En, components.SidebarBody(state.FilterSidebar(groupedRows(), "api")))

	input := openingTag(t, body, "board-sidebar-filter")
	assert.Contains(t, input, `name="q"`)
	assert.Contains(t, input, `value="api"`)
	assert.Contains(t, input, `hx-get="/board/sidebar"`)
	panelAt := strings.Index(body, `id="board-sidebar-search"`)
	require.GreaterOrEqual(t, panelAt, 0)
	assert.Greater(t, strings.Index(body, `id="board-sidebar-filter"`), panelAt, "the input lives inside the wrapper")
}
