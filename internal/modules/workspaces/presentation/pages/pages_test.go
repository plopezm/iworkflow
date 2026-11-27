package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/workspaces/presentation/state"
)

func render(t *testing.T, component templ.Component) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.En, Chosen: true})
	var buffer bytes.Buffer
	require.NoError(t, component.Render(ctx, &buffer))
	return buffer.String()
}

func TestWorkspacesPageOnlyLinksToGroupsAndNeverManagesThem(t *testing.T) {
	body := render(t, WorkspacesPage(state.WorkspacesPage{NewHref: state.NewPath, GroupsHref: state.GroupsPath}))

	assert.Contains(t, body, `data-dock-path="/workspaces/groups"`)
	assert.NotContains(t, body, "group-manager")
	assert.NotContains(t, body, "workspace-groups")
}

func TestEditPageLoadsTheGroupsInTheBackgroundWithALoadingText(t *testing.T) {
	body := render(t, WorkspaceEditPage(state.EditPage{ID: "w1", Name: "alpha"}))

	assert.Contains(t, body, `hx-get="/workspaces/w1/edit/groups"`)
	assert.Contains(t, body, `hx-trigger="load"`)
	assert.Contains(t, body, `aria-busy="true"`)
	assert.Contains(t, body, "Loading groups…")
	assert.NotContains(t, body, "data-workspace-group-select", "no selector until the groups arrive")
}

// When the groups request fails (network error or non-2xx), htmx does not swap it: the slot must switch to an error text
// with a Retry button that asks for the same URL into the same slot, and it must stop being busy.
func TestEditGroupsSlotRecoversWhenTheRequestFails(t *testing.T) {
	body := render(t, WorkspaceEditPage(state.EditPage{ID: "w1", Name: "alpha"}))

	assert.Contains(t, body, `hx-on:htmx:before-swap="`, "a non-2xx response is not swapped in and switches the slot to its error part")
	assert.Contains(t, body, "shouldSwap=false")
	assert.Contains(t, body, `hx-on:htmx:send-error="`, "a network error switches the slot to its error part")
	assert.Contains(t, body, `data-groups-error`)
	assert.Contains(t, body, "Could not load the groups")
	assert.Contains(t, body, "Retry")
	assert.Equal(t, 2, strings.Count(body, `hx-get="/workspaces/w1/edit/groups"`), "the slot and the Retry button ask for the same URL")
	assert.Contains(t, body, `hx-trigger="click"`)
	assert.Contains(t, body, `hx-target="#workspace-edit-groups"`)
}

func TestEditGroupsOffersTheSelectorThatSavesOnItsOwn(t *testing.T) {
	body := render(t, EditGroups(state.EditGroups{
		ID: "w1", Selected: "g2",
		Options: []state.GroupOption{{ID: "g1", Name: "Backend"}, {ID: "g2", Name: "Docs"}},
	}))

	assert.Contains(t, body, `data-workspace-group-select`)
	assert.Contains(t, body, `name="group"`)
	assert.Contains(t, body, `hx-post="/workspace-groups/workspaces/w1/group"`)
	assert.Contains(t, body, `hx-trigger="change"`)
	assert.Contains(t, body, `<option value="g2" selected>Docs</option>`)
	assert.Contains(t, body, "No group")
}

func TestEditGroupsWithoutGroupsLeavesAnEmptySlot(t *testing.T) {
	body := render(t, EditGroups(state.EditGroups{ID: "w1"}))

	assert.NotContains(t, body, "data-workspace-group-select")
	assert.NotContains(t, body, `name="group"`)
	assert.Contains(t, body, `id="workspace-edit-groups"`)
}
