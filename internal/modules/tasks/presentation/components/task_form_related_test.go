package components_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// renderRelatedIn renders a component in the locale, as the request of the page would.
func renderRelatedIn(t *testing.T, locale i18n.Locale, component templ.Component) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: locale, Chosen: true})
	var out bytes.Buffer
	require.NoError(t, component.Render(ctx, &out))
	return out.String()
}

// relatedInput is the checkbox of one workspace, as the page renders it: its opening tag.
func relatedInput(t *testing.T, body string, workspaceID string) string {
	t.Helper()
	start := strings.Index(body, `data-workspace-id="`+workspaceID+`"`)
	require.GreaterOrEqual(t, start, 0, "the checkbox of %s is rendered", workspaceID)
	open := strings.LastIndex(body[:start], "<input")
	return body[open : start+strings.Index(body[start:], ">")]
}

func TestRelatedProjectsFieldRendersOneCheckboxPerWorkspaceWithItsPath(t *testing.T) {
	t.Parallel()
	projects := []state.RelatedProjectChoice{{ID: "api-id", Name: "api", Path: "/repos/api"}}

	body := renderRelatedIn(t, i18n.En, components.RelatedProjectsField(projects))

	assert.Contains(t, body, `data-related-projects`)
	assert.Contains(t, body, `<legend class="text-sm font-medium text-fg">Related projects</legend>`)
	assert.Contains(t, body, "read their files and, when available, their engram and codegraph")
	input := relatedInput(t, body, "api-id")
	assert.Contains(t, input, `name="related_project_id"`)
	assert.Contains(t, input, `value="api-id"`)
	assert.NotContains(t, input, "checked", "an unchosen project is not checked")
	assert.Contains(t, body, `<span class="text-xs text-muted">/repos/api</span>`, "the path is shown muted under the name")
}

func TestRelatedProjectsFieldChecksTheChosenProjects(t *testing.T) {
	t.Parallel()
	projects := []state.RelatedProjectChoice{
		{ID: "api-id", Name: "api", Path: "/repos/api", Selected: true},
		{ID: "web-id", Name: "web", Path: "/repos/web"},
	}

	body := renderRelatedIn(t, i18n.En, components.RelatedProjectsField(projects))

	assert.Contains(t, relatedInput(t, body, "api-id"), "checked")
	assert.NotContains(t, relatedInput(t, body, "web-id"), "checked")
}

func TestRelatedProjectsFieldRendersNothingWithoutAnotherWorkspace(t *testing.T) {
	t.Parallel()

	body := renderRelatedIn(t, i18n.En, components.RelatedProjectsField(nil))

	assert.NotContains(t, body, "data-related-projects")
	assert.NotContains(t, body, "related_project_id")
}

func TestRelatedProjectsFieldEscapesNamesAndPaths(t *testing.T) {
	t.Parallel()
	projects := []state.RelatedProjectChoice{{ID: "x-id", Name: `<script>alert(1)</script>`, Path: `/repos/a&b "c"`}}

	body := renderRelatedIn(t, i18n.En, components.RelatedProjectsField(projects))

	assert.NotContains(t, body, "<script>alert(1)</script>")
	assert.Contains(t, body, "&lt;script&gt;alert(1)&lt;/script&gt;")
	assert.Contains(t, body, "/repos/a&amp;b &#34;c&#34;")
}

func TestRelatedProjectsFieldIsLocalised(t *testing.T) {
	t.Parallel()
	projects := []state.RelatedProjectChoice{{ID: "api-id", Name: "api", Path: "/repos/api"}}

	body := renderRelatedIn(t, i18n.Es, components.RelatedProjectsField(projects))

	assert.Contains(t, body, "Proyectos relacionados")
}

func TestCreateDialogPlacesTheRelatedProjectsAfterTheBaseBranchAndBeforeThePrompt(t *testing.T) {
	t.Parallel()
	form := state.CreateForm{
		Scope:           "s1",
		WorkspaceID:     "own-id",
		Workspaces:      []dto.WorkspaceOptionDTO{{ID: "own-id", Name: "app"}, {ID: "api-id", Name: "api", Path: "/repos/api"}},
		RelatedProjects: []state.RelatedProjectChoice{{ID: "api-id", Name: "api", Path: "/repos/api", Selected: true}},
	}

	body := renderRelatedIn(t, i18n.En, components.CreateTaskDialog(form))

	branch := strings.Index(body, `name="base_branch"`)
	related := strings.Index(body, `data-related-projects`)
	prompt := strings.Index(body, `name="prompt"`)
	require.True(t, branch >= 0 && related >= 0 && prompt >= 0)
	assert.Less(t, branch, related, "the list comes after the base branch")
	assert.Less(t, related, prompt, "the list comes before the prompt, so the user ticks projects before typing @")
	assert.Contains(t, relatedInput(t, body, "api-id"), "checked")
}

func TestEditFieldsPlaceTheRelatedProjectsAfterTheBaseBranchAndBeforeThePrompt(t *testing.T) {
	t.Parallel()
	form := state.EditForm{
		Scope:           "s1",
		TaskID:          "t1",
		RelatedProjects: []state.RelatedProjectChoice{{ID: "api-id", Name: "api", Path: "/repos/api"}},
	}

	body := renderRelatedIn(t, i18n.En, components.TaskEditFields(form))

	branch := strings.Index(body, `name="base_branch"`)
	related := strings.Index(body, `data-related-projects`)
	prompt := strings.Index(body, `name="prompt"`)
	require.True(t, branch >= 0 && related >= 0 && prompt >= 0)
	assert.Less(t, branch, related)
	assert.Less(t, related, prompt)
	assert.NotContains(t, relatedInput(t, body, "api-id"), "checked")
}
