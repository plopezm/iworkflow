package components

import (
	"strings"
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

// When the groups request fails, the slot must switch to an error text with a Retry button that asks for the same URL
// into the same slot. The hidden group_id stays in the slot, so a quick submit keeps the chosen group.
func TestRegisterGroupsSlotRecoversWhenTheRequestFails(t *testing.T) {
	body := render(t, WorkspaceRegister(state.RegisterForm{GroupID: "g 2"}))

	assert.Contains(t, body, `hx-on:htmx:before-swap="`, "a non-2xx response is not swapped in and switches the slot to its error part")
	assert.Contains(t, body, "shouldSwap=false")
	assert.Contains(t, body, `hx-on:htmx:send-error="`, "a network error switches the slot to its error part")
	assert.Contains(t, body, `data-groups-error`)
	assert.Contains(t, body, "No se pudieron cargar los grupos")
	assert.Contains(t, body, "Reintentar")
	assert.Equal(t, 2, strings.Count(body, `hx-get="/workspaces/register/groups?group_id=g+2"`), "the slot and the Retry button ask for the same URL")
	assert.Contains(t, body, `hx-trigger="click"`)
	assert.Contains(t, body, `hx-target="#workspace-register-groups"`)
	assert.Contains(t, body, `<input type="hidden" name="group_id" value="g 2">`)
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
