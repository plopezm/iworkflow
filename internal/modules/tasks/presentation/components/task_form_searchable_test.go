package components_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// baseBranchSelect is the opening tag of the base branch select in a rendered body: the element web/static/js/searchable_select.js
// upgrades into a combobox, and the field the form submits.
func baseBranchSelect(t *testing.T, body string) string {
	t.Helper()
	field := strings.Index(body, `name="base_branch"`)
	require.GreaterOrEqual(t, field, 0, "the base branch select is rendered")
	start := strings.LastIndex(body[:field], "<select")
	require.GreaterOrEqual(t, start, 0)
	end := strings.Index(body[field:], ">")
	require.GreaterOrEqual(t, end, 0)
	return body[start : field+end+1]
}

func TestBranchSelectNoticeIsMarkedSearchable(t *testing.T) {
	body := renderRelatedIn(t, i18n.En, components.BranchSelectNotice(
		"board-branch-slot", state.BranchChoices(nil, "origin/develop"), "origin/develop", "main", ""))

	tag := baseBranchSelect(t, body)
	assert.Contains(t, tag, " data-searchable", "the select is upgraded into a combobox")
	assert.Contains(t, tag, `data-search-placeholder="Filter…"`)
	assert.Contains(t, tag, `data-search-empty="No matches"`)
	assert.Contains(t, tag, `name="base_branch"`, "the select stays the submitted field")
	assert.Contains(t, body, `<option value="origin/develop" selected`, "the stored branch stays selected")
}

func TestBranchSelectLazyIsMarkedSearchable(t *testing.T) {
	body := renderRelatedIn(t, i18n.En, components.BranchSelectLazy(
		"task-edit-branch-s1", state.BranchChoices(nil, "origin/develop"), "origin/develop", "main", state.EditBranchesPath("ws1", "s1")))

	tag := baseBranchSelect(t, body)
	assert.Contains(t, tag, " data-searchable", "the placeholder select of the edit page is upgraded too")
	assert.Contains(t, tag, `name="base_branch"`)
	assert.Contains(t, body, `hx-include="find select"`, "the lazy container still includes the select")
}

func TestBranchSelectSearchableLabelsFollowTheLocale(t *testing.T) {
	body := renderRelatedIn(t, i18n.Es, components.BranchSelectNotice(
		"board-branch-slot", state.BranchChoices(nil, ""), "", "main", ""))

	tag := baseBranchSelect(t, body)
	assert.Contains(t, tag, `data-search-placeholder="Filtrar…"`)
	assert.Contains(t, tag, `data-search-empty="Sin coincidencias"`)
}
