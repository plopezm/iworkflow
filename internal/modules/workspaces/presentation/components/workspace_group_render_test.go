package components

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/workspaces/presentation/state"
)

// The groups are not read while the page is built: the form carries a slot that asks for them once it is shown.
func TestRegisterFormLoadsTheGroupsInTheBackgroundWithALoadingText(t *testing.T) {
	body := render(t, WorkspaceRegister(state.RegisterForm{GroupID: "g 2"}))

	assert.Contains(t, body, `hx-get="/workspaces/register/groups?group_id=g+2"`)
	assert.Contains(t, body, `hx-trigger="load"`)
	assert.Contains(t, body, `aria-busy="true"`)
	assert.Contains(t, body, "Cargando grupos…")
	assert.Contains(t, body, `<input type="hidden" name="group_id" value="g 2">`, "a quick submit keeps the chosen group")
	assert.NotContains(t, body, `data-register-group`, "no selector until the groups arrive")
}

func TestRegisterGroupsListsTheGroupsAndKeepsTheChosenOne(t *testing.T) {
	body := render(t, RegisterGroups(state.RegisterGroups{
		Options:  []state.GroupOption{{ID: "g1", Name: "Backend"}, {ID: "g2", Name: "Family"}},
		Selected: "g2",
	}))

	assert.Contains(t, body, `data-register-group`)
	assert.Contains(t, body, `name="group_id"`)
	assert.Contains(t, body, `<option value="g1"`)
	assert.Contains(t, body, "Backend")
	assert.Regexp(t, `value="g2"[^>]*selected`, body, "the group chosen before a failed submit stays selected")
	assert.NotContains(t, body, "Cargando grupos")
}

func TestRegisterGroupsLeavesAnEmptySlotWhenThereAreNoGroups(t *testing.T) {
	body := render(t, RegisterGroups(state.RegisterGroups{}))

	assert.NotContains(t, body, `data-register-group`)
	assert.NotContains(t, body, `name="group_id"`)
	assert.NotContains(t, body, "Cargando grupos")
	assert.Contains(t, body, `id="workspace-register-groups"`, "the slot stays, so it is not requested again")
}
