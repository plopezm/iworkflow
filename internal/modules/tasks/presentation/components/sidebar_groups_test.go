package components_test

import (
	"html"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// sidebarVals is an hx-vals attribute value as templ writes it: the JSON with its quotes escaped.
func sidebarVals(json string) string { return html.EscapeString(json) }

// groupedRows is an arranged sidebar: a pinned workspace, a collapsed group, one ungrouped workspace.
func groupedRows() state.SidebarRows {
	return state.SidebarRows{
		Arranged: true,
		Pinned:   []state.SidebarWorkspace{{ID: "p1", Name: "pinned", Pinned: true}},
		Groups: []state.SidebarGroup{{
			ID:             "g1",
			Name:           "Backend",
			Color:          "blue",
			Collapsed:      true,
			NeedsAttention: true,
			Workspaces:     []state.SidebarWorkspace{{ID: "w1", Name: "api", GroupID: "g1"}},
		}},
		Ungrouped: []state.SidebarWorkspace{{ID: "w2", Name: "web"}},
		Workspaces: []state.SidebarWorkspace{
			{ID: "p1", Name: "pinned", Pinned: true},
			{ID: "w1", Name: "api", GroupID: "g1"},
			{ID: "w2", Name: "web"},
		},
	}
}

func TestSidebarWithoutGroupsOrPinsListsEveryWorkspace(t *testing.T) {
	t.Parallel()

	rows := state.BuildSidebar(nil, nil)
	rows.Workspaces = []state.SidebarWorkspace{{ID: "w1", Name: "alpha"}, {ID: "w2", Name: "beta"}}
	rows.Ungrouped = rows.Workspaces

	html := renderComponent(t, i18n.En, components.SidebarBody(rows))

	assert.Contains(t, html, `data-workspace-row="w1"`)
	assert.Contains(t, html, `data-workspace-row="w2"`)
	assert.NotContains(t, html, `data-pinned-section`)
	assert.NotContains(t, html, `data-group-id`)
}

func TestCollapsedGroupHidesItsRowsButShowsCountAndAttentionMarker(t *testing.T) {
	t.Parallel()

	html := renderComponent(t, i18n.En, components.SidebarBody(groupedRows()))

	assert.Contains(t, html, `data-group-id="g1"`)
	assert.Contains(t, html, `data-group-collapsed="true"`)
	assert.Contains(t, html, `data-group-count="1"`)
	assert.Contains(t, html, `data-group-attention`, "a collapsed group with attention shows the marker")
	assert.NotContains(t, html, `data-workspace-row="w1"`, "the rows of a collapsed group are not rendered")
}

func TestExpandedGroupShowsItsRowsAndNoGroupMarker(t *testing.T) {
	t.Parallel()
	rows := groupedRows()
	rows.Groups[0].Collapsed = false

	html := renderComponent(t, i18n.En, components.SidebarBody(rows))

	assert.Contains(t, html, `data-group-collapsed="false"`)
	assert.Contains(t, html, `data-workspace-row="w1"`)
	assert.NotContains(t, html, `data-group-attention`)
}

func TestSidebarPinnedBlockAndUngroupedRowsRenderAroundTheGroups(t *testing.T) {
	t.Parallel()
	rows := groupedRows()
	rows.Groups[0].Collapsed = false

	html := renderComponent(t, i18n.En, components.SidebarBody(rows))

	pinned := strings.Index(html, `data-pinned-section`)
	group := strings.Index(html, `data-group-id="g1"`)
	loose := strings.Index(html, `data-workspace-row="w2"`)
	assert.Positive(t, pinned)
	assert.Less(t, pinned, group, "pinned workspaces come above the groups")
	assert.Less(t, group, loose, "ungrouped workspaces come below the groups")
	assert.Equal(t, 1, strings.Count(html, `data-workspace-row="p1"`), "a pinned workspace is listed once")
}

func TestGroupMenusPostToTheGroupEndpoints(t *testing.T) {
	t.Parallel()
	rows := groupedRows()
	rows.Groups[0].Collapsed = false

	html := renderComponent(t, i18n.En, components.SidebarBody(rows))

	assert.Contains(t, html, `hx-post="/workspace-groups"`, "the + button creates a group")
	assert.Contains(t, html, `hx-post="/workspace-groups/g1/collapse"`)
	assert.Contains(t, html, `hx-post="/workspace-groups/g1"`, "rename and recolor")
	assert.Contains(t, html, `hx-post="/workspace-groups/g1/delete"`)
	assert.Contains(t, html, `hx-post="/workspace-groups/workspaces/w2/group"`, "the ungrouped row can be moved")
	assert.Contains(t, html, `hx-post="/workspace-groups/workspaces/w1/pin"`, "the grouped row can be pinned")
	assert.Contains(t, html, `hx-post="/workspace-groups/workspaces/w1/group"`, "the grouped row can be moved or ungrouped")
	assert.Contains(t, html, sidebarVals(`{"group":""}`), "a grouped row can be removed from its group")
	assert.Contains(t, html, `hx-swap="none"`, "menu actions answer 204 and never swap the sidebar")
}

func TestWorkspaceMovesOfferEveryGroupButItsOwn(t *testing.T) {
	t.Parallel()
	rows := state.SidebarRows{
		Arranged: true,
		Groups: []state.SidebarGroup{
			{ID: "g1", Name: "Backend", Workspaces: []state.SidebarWorkspace{{ID: "w1", Name: "api", GroupID: "g1"}}},
			{ID: "g2", Name: "Frontend"},
		},
	}
	rows.Workspaces = []state.SidebarWorkspace{{ID: "w1", Name: "api", GroupID: "g1"}}

	html := renderComponent(t, i18n.En, components.SidebarBody(rows))

	assert.Contains(t, html, sidebarVals(`{"group":"g2"}`), "the other group is a move target")
	assert.NotContains(t, html, sidebarVals(`{"group":"g1"}`), "moving a workspace to the group it is already in is not offered")
}

func TestBoardSidebarRootKeepsTheMorphSwap(t *testing.T) {
	t.Parallel()

	html := renderComponent(t, i18n.En, components.BoardSidebar(groupedRows()))

	opening := html[strings.Index(html, `id="board-sidebar"`):]
	opening = opening[:strings.Index(opening, ">")]
	assert.Contains(t, opening, `hx-swap="morph:innerHTML"`)
}

// An arranged layout with every workspace pinned renders them in the pinned block and no ungrouped row after it.
func TestArrangedLayoutWithEverythingPinnedRendersNoUngroupedRows(t *testing.T) {
	t.Parallel()
	rows := state.ApplyLayout(state.BuildSidebar([]dto.WorkspaceOptionDTO{{ID: "w1", Name: "alpha"}, {ID: "w2", Name: "beta"}}, nil), models.SidebarLayout{Pinned: []string{"w1", "w2"}})

	html := renderComponent(t, i18n.En, components.SidebarBody(rows))

	assert.Equal(t, 2, strings.Count(html, `data-workspace-row=`), "each workspace is rendered once, in the pinned block")
	assert.Less(t, strings.Index(html, `data-pinned-section`), strings.Index(html, `data-workspace-row="w2"`))
}
