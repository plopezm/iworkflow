package models_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/domain/models"
)

func relatedProject(label string, path string, hasCodegraph bool) models.RelatedProject {
	return models.RelatedProject{
		Ref:          models.WorkspaceRef{ID: "id-" + label, Name: label, Path: path, BaseBranch: "main"},
		Label:        label,
		HasCodegraph: hasCodegraph,
	}
}

func TestRelatedProjectsPromptIsEmptyWithoutProjects(t *testing.T) {
	t.Parallel()
	assert.Empty(t, models.RelatedProjectsPrompt(nil, false))
	assert.Empty(t, models.RelatedProjectsPrompt([]models.RelatedProject{}, true))
}

func TestRelatedProjectsPromptListsEachProjectWithItsLabelAndPath(t *testing.T) {
	t.Parallel()
	section := models.RelatedProjectsPrompt([]models.RelatedProject{
		relatedProject("api", "/repos/api", false),
		relatedProject("web-app", "/repos/web", false),
	}, false)

	assert.Contains(t, section, "## Related projects")
	assert.Contains(t, section, "`@api` -> `/repos/api` (api)")
	assert.Contains(t, section, "`@web-app` -> `/repos/web` (web-app)")
	assert.Less(t, strings.Index(section, "@api"), strings.Index(section, "@web-app"), "the stored order is kept")
	assert.Contains(t, section, "MAY read")
	assert.Contains(t, section, "Do not modify files of related projects")
	assert.Contains(t, section, "The edit tools are denied there")
	assert.Contains(t, section, "shell commands are not sandboxed")
	assert.NotContains(t, section, "MUST NOT", "the prompt must not claim a guarantee it does not enforce")
	assert.Contains(t, section, "`@<label>:<relative/path>`")
}

func TestRelatedProjectsPromptOmitsEngramAndCodegraphWhenNotAvailable(t *testing.T) {
	t.Parallel()
	section := models.RelatedProjectsPrompt([]models.RelatedProject{relatedProject("api", "/repos/api", false)}, false)

	assert.NotContains(t, section, "codegraph")
	assert.NotContains(t, section, "mem_current_project")
	assert.NotContains(t, section, "mem_search")
}

func TestRelatedProjectsPromptMarksOnlyTheProjectsWithCodegraph(t *testing.T) {
	t.Parallel()
	section := models.RelatedProjectsPrompt([]models.RelatedProject{
		relatedProject("api", "/repos/api", true),
		relatedProject("web", "/repos/web", false),
	}, false)

	assert.Contains(t, section, "`@api` -> `/repos/api` (api) - codegraph index available")
	assert.NotContains(t, section, "`@web` -> `/repos/web` (web) - codegraph index available")
	assert.Contains(t, section, "`projectPath`")
	assert.NotContains(t, section, "mem_current_project")
}

func TestRelatedProjectsPromptExplainsEngramWhenAvailable(t *testing.T) {
	t.Parallel()
	section := models.RelatedProjectsPrompt([]models.RelatedProject{relatedProject("api", "/repos/api", false)}, true)

	assert.Contains(t, section, "`mem_current_project`")
	assert.Contains(t, section, "`cwd`")
	assert.Contains(t, section, "`project`")
	assert.Contains(t, section, "`mem_search`")
	assert.Contains(t, section, "`mem_context`")
	assert.Contains(t, section, "read-only")
	assert.NotContains(t, section, "codegraph")
}

func TestRelatedProjectsPromptMentionsTheCodeApiWithoutANewEndpoint(t *testing.T) {
	t.Parallel()
	section := models.RelatedProjectsPrompt([]models.RelatedProject{relatedProject("api", "/repos/api", false)}, false)

	assert.Contains(t, section, "API")
	assert.Contains(t, section, "read its sources")
	assert.Contains(t, section, "no new endpoint")
}

func TestAppendPromptSectionSeparatesTheSectionWithABlankLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		prompt  string
		section string
		want    string
	}{
		{name: "empty section leaves the prompt unchanged", prompt: "Fix the login", section: "", want: "Fix the login"},
		{name: "section follows a blank line", prompt: "Fix the login", section: "## Related projects", want: "Fix the login\n\n## Related projects"},
		{name: "empty prompt yields the section alone", prompt: "  ", section: "## Related projects", want: "## Related projects"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, models.AppendPromptSection(tt.prompt, tt.section))
		})
	}
}
