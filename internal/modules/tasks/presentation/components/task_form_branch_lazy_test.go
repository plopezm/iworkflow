package components_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

func TestBranchSelectLazyShowsTheStoredBranchAndLoadsTheListInTheBackground(t *testing.T) {
	src := state.EditBranchesPath("ws1", "s1")
	body := renderRelatedIn(t, i18n.En, components.BranchSelectLazy(
		"task-edit-branch-s1", state.BranchChoices(nil, "origin/develop"), "origin/develop", "main", src))

	assert.Contains(t, body, `hx-trigger="load"`)
	assert.Contains(t, body, `hx-swap="outerHTML"`)
	assert.Contains(t, body, `hx-target="this"`, "the form around it must not lend its result target")
	assert.Contains(t, body, `hx-disabled-elt="unset"`, "loading must not disable the save button of the form")
	assert.Contains(t, body, `hx-include="find select"`)
	assert.Contains(t, body, `slot=edit`)
	assert.Contains(t, body, `workspace_id=ws1`)
	assert.Contains(t, body, `<option value="origin/develop" selected`)
	assert.Contains(t, body, `aria-busy="true"`)
	assert.Contains(t, body, `Loading branches`, "the user sees that the list is loading")
}

func TestAgentSectionLazyShowsLoadingAndKeepsTheStoredChoices(t *testing.T) {
	agent := state.AgentFormState{SelectedRuntime: "claude", SelectedModel: "opus", SelectedThinking: "high", SelectedProfile: "dev"}
	body := renderRelatedIn(t, i18n.En, components.AgentSectionLazy("s1", agent, state.EditAgentSectionPath("t1", "s1")))

	assert.Contains(t, body, `hx-trigger="load"`)
	assert.Contains(t, body, `hx-swap="outerHTML"`)
	assert.Contains(t, body, `hx-target="this"`, "the form around it must not lend its result target")
	assert.Contains(t, body, `hx-disabled-elt="unset"`, "loading must not disable the save button of the form")
	assert.Contains(t, body, `task_id=t1`)
	assert.Contains(t, body, `Loading agent options`)
	for _, field := range []string{`name="agent_runtime" value="claude"`, `name="agent_model" value="opus"`, `name="agent_thinking" value="high"`, `name="agent_profile" value="dev"`} {
		assert.Contains(t, body, field)
	}
}

func TestCreateFieldsLoadTheWorkspacesAndTheAgentCatalogInTheBackground(t *testing.T) {
	form := state.CreateForm{Scope: "s1", WorkspaceID: "ws1", BaseBranch: "dev", DefaultBranch: "main"}
	body := renderRelatedIn(t, i18n.En, components.CreateTaskFields(form))

	assert.Contains(t, body, `/board/tasks/create-workspaces?base_branch=dev&amp;scope=s1&amp;workspace_id=ws1`)
	assert.Contains(t, body, `/board/tasks/agent-section?`)
	assert.Contains(t, body, `Loading workspaces`)
	assert.Contains(t, body, `Loading agent options`)
	assert.NotContains(t, body, `<select class="input" name="workspace_id"`, "the workspace select arrives with the loaded fields")
	assert.Contains(t, body, `name="workspace_id" value="ws1"`, "a submit before the load keeps the workspace")
	assert.Equal(t, 2, strings.Count(body, `hx-disabled-elt="unset"`), "loading must not disable the create button")
}

func TestWorkspaceFieldsLoadTheBranchesOnlyWithAWorkspace(t *testing.T) {
	with := renderRelatedIn(t, i18n.En, components.WorkspaceFields(state.CreateForm{Scope: "s1", WorkspaceID: "ws1", BaseBranch: "dev", DefaultBranch: "main"}))
	assert.Contains(t, with, `/board/tasks/branches?base_branch=dev&amp;scope=s1&amp;workspace_id=ws1`)
	assert.Contains(t, with, `Loading branches`)

	without := renderRelatedIn(t, i18n.En, components.WorkspaceFields(state.CreateForm{Scope: "s1"}))
	assert.NotContains(t, without, `Loading branches`)
	assert.Contains(t, without, `id="board-branch-slot-s1"`)
}
