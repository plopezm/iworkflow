package pages

import (
	"bytes"
	"context"
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
