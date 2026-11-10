package pages_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/jira/presentation/pages"
	"iworkflow/internal/modules/jira/presentation/state"
)

func renderConvert(t *testing.T, workspaces []dto.WorkspaceDTO, related []string) string {
	t.Helper()
	view := state.ConvertIssueView{Preparation: dto.ConversionPreparationDTO{
		Issue: dto.IssueDTO{Key: "A-1"}, Workspaces: workspaces, RelatedProjectIDs: related,
	}}
	var out bytes.Buffer
	require.NoError(t, pages.JiraConvertPage(view).Render(context.Background(), &out))
	return out.String()
}

func TestConvertPagePromptCompletesFilesAndOffersRelatedProjects(t *testing.T) {
	html := renderConvert(t, []dto.WorkspaceDTO{{ID: "w1", Name: "one", Path: "/a"}, {ID: "w2", Name: "two", Path: "/b"}}, []string{"w2"})

	assert.Contains(t, html, `data-composer`)
	assert.Contains(t, html, `data-files-path="/board/tasks/composer/files"`)
	assert.Regexp(t, `<input[^>]*value="w2"[^>]*checked`, html, "a chosen related project stays checked")
	assert.NotRegexp(t, `<input[^>]*value="w1"[^>]*checked`, html, "an unchosen project is not checked")
	assert.Contains(t, html, `data-workspace-id="w1"`)
}

func TestConvertPageHidesRelatedProjectsWithASingleWorkspace(t *testing.T) {
	html := renderConvert(t, []dto.WorkspaceDTO{{ID: "w1", Name: "one"}}, nil)

	assert.NotContains(t, html, `related_project_id`)
	assert.Contains(t, html, `data-composer`)
}
