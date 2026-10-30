package code_hosting_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	codeerrors "iworkflow/internal/modules/code_hosting/domain/errors"
	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/infrastructure/adapters/outbound/api"
)

const (
	gitLabToken = "glpat-test"
	gitLabMR    = `{"iid":7,"title":"T","state":"opened","source_branch":"pi/x","target_branch":"main","web_url":"http://gl/mr/7","merge_status":"can_be_merged","detailed_merge_status":"mergeable","has_conflicts":false}`
)

func newGitLabClient() *api.GitLabClient {
	return api.NewGitLabClient(api.NewClientManager())
}

func gitLabSettings(t *testing.T, provider *fakeProvider) models.CodeHostSettings {
	t.Helper()
	return settingsFor(t, models.GitLab, provider.URL(), gitLabToken)
}

func TestGitLabCurrentUserSendsTheTokenAndReturnsTheUsername(t *testing.T) {
	provider := newFakeProvider(t, scriptedRoute{target: "GET /api/v4/user", status: 200, body: `{"username":"pablo"}`})

	user, err := newGitLabClient().CurrentUser(context.Background(), gitLabSettings(t, provider))

	require.NoError(t, err)
	require.Equal(t, "pablo", user)
	require.Equal(t, gitLabToken, provider.requests()[0].privateToken)
}

func TestGitLabBranchExistsMaps404ToFalse(t *testing.T) {
	provider := newFakeProvider(t, scriptedRoute{
		target: "GET /api/v4/projects/g%2Fp/repository/branches/pi%2Fx",
		status: 200,
		body:   `{"name":"pi/x"}`,
	})
	client := newGitLabClient()
	settings := gitLabSettings(t, provider)

	exists, err := client.BranchExists(context.Background(), settings, "g/p", "pi/x")
	require.NoError(t, err)
	require.True(t, exists)
	missing, err := client.BranchExists(context.Background(), settings, "g/p", "otra")
	require.NoError(t, err)
	require.False(t, missing)
}

func TestGitLabFindLatestMergeRequestReturnsTheFirstOrNil(t *testing.T) {
	provider := newFakeProvider(t,
		scriptedRoute{
			target: "GET /api/v4/projects/g%2Fp/merge_requests?source_branch=pi%2Fx&order_by=created_at&sort=desc",
			status: 200,
			body:   "[" + gitLabMR + "]",
		},
		scriptedRoute{
			target: "GET /api/v4/projects/g%2Fp/merge_requests?source_branch=vacia&order_by=created_at&sort=desc",
			status: 200,
			body:   "[]",
		},
	)
	client := newGitLabClient()
	settings := gitLabSettings(t, provider)

	found, err := client.FindLatestMergeRequest(context.Background(), settings, "g/p", "pi/x")
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, uint64(7), found.IID)
	require.Equal(t, "mergeable", found.MergeStatus)
	require.True(t, found.IsOpen())
	missing, err := client.FindLatestMergeRequest(context.Background(), settings, "g/p", "vacia")
	require.NoError(t, err)
	require.Nil(t, missing)
}

func TestGitLabFindLatestPrefersTheOpenRequest(t *testing.T) {
	list := `[{"iid":5,"title":"Old","state":"closed","source_branch":"pi/x","target_branch":"main","web_url":"u5"},` + gitLabMR + `]`
	provider := newFakeProvider(t, scriptedRoute{
		target: "GET /api/v4/projects/g%2Fp/merge_requests?source_branch=pi%2Fx&order_by=created_at&sort=desc",
		status: 200,
		body:   list,
	})

	found, err := newGitLabClient().FindLatestMergeRequest(context.Background(), gitLabSettings(t, provider), "g/p", "pi/x")

	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, uint64(7), found.IID)
}

func TestGitLabCreateMergeRequestPostsTheExpectedBody(t *testing.T) {
	provider := newFakeProvider(t, scriptedRoute{target: "POST /api/v4/projects/g%2Fp/merge_requests", status: 201, body: gitLabMR})

	created, err := newGitLabClient().CreateMergeRequest(context.Background(), gitLabSettings(t, provider), "g/p", models.NewMergeRequest{
		SourceBranch: "pi/x",
		TargetBranch: "main",
		Title:        "T",
		Description:  "D",
	})

	require.NoError(t, err)
	require.Equal(t, "http://gl/mr/7", created.WebURL)
	var body map[string]string
	require.NoError(t, json.Unmarshal([]byte(provider.requests()[0].body), &body))
	require.Equal(t, map[string]string{"source_branch": "pi/x", "target_branch": "main", "title": "T", "description": "D"}, body)
}

func TestGitLabLatestPipelineReturnsTheNewestWithItsJobs(t *testing.T) {
	provider := newFakeProvider(t,
		scriptedRoute{
			target: "GET /api/v4/projects/g%2Fp/merge_requests/7/pipelines",
			status: 200,
			body:   `[{"id":22,"status":"running","web_url":"http://gl/p/22"},{"id":21,"status":"failed","web_url":"http://gl/p/21"}]`,
		},
		scriptedRoute{
			target: "GET /api/v4/projects/g%2Fp/pipelines/22/jobs?per_page=100",
			status: 200,
			body:   `[{"id":1,"name":"build","stage":"build","status":"success","web_url":"http://gl/j/1"}]`,
		},
	)

	pipeline, err := newGitLabClient().LatestPipeline(context.Background(), gitLabSettings(t, provider), "g/p", 7)

	require.NoError(t, err)
	require.NotNil(t, pipeline)
	require.Equal(t, uint64(22), pipeline.ID)
	require.Equal(t, "running", pipeline.Status)
	require.Len(t, pipeline.Jobs, 1)
	require.Equal(t, "build", pipeline.Jobs[0].Name)
}

func TestGitLabLatestPipelineIsNilWithoutPipelines(t *testing.T) {
	provider := newFakeProvider(t, scriptedRoute{target: "GET /api/v4/projects/g%2Fp/merge_requests/7/pipelines", status: 200, body: "[]"})

	pipeline, err := newGitLabClient().LatestPipeline(context.Background(), gitLabSettings(t, provider), "g/p", 7)

	require.NoError(t, err)
	require.Nil(t, pipeline)
}

func TestGitLabMergePutsAndSurfacesTheProviderMessage(t *testing.T) {
	provider := newFakeProvider(t,
		scriptedRoute{target: "PUT /api/v4/projects/g%2Fp/merge_requests/7/merge", status: 200, body: gitLabMR},
		scriptedRoute{target: "PUT /api/v4/projects/g%2Fp/merge_requests/8/merge", status: 405, body: `{"message":"405 Method Not Allowed"}`},
	)
	client := newGitLabClient()
	settings := gitLabSettings(t, provider)

	_, err := client.MergeMergeRequest(context.Background(), settings, "g/p", 7)
	require.NoError(t, err)
	_, err = client.MergeMergeRequest(context.Background(), settings, "g/p", 8)

	require.Equal(t, codeerrors.API(models.GitLab, 405, "405 Method Not Allowed"), err)
	require.Len(t, provider.requests(), 2)
}

func TestGitLabUnauthorizedReportsTheErrorField(t *testing.T) {
	provider := newFakeProvider(t, scriptedRoute{target: "GET /api/v4/user", status: 401, body: `{"message":"401 Unauthorized"}`})

	_, err := newGitLabClient().CurrentUser(context.Background(), gitLabSettings(t, provider))

	require.ErrorIs(t, err, codeerrors.API(models.GitLab, 401, ""))
	require.Equal(t, codeerrors.API(models.GitLab, 401, "401 Unauthorized"), err)
}

func TestGitLabNetworkFailureDoesNotLeakTheToken(t *testing.T) {
	settings := settingsFor(t, models.GitLab, "http://127.0.0.1:1", gitLabToken)

	_, err := newGitLabClient().CurrentUser(context.Background(), settings)

	require.ErrorIs(t, err, codeerrors.Network(""))
	require.False(t, strings.Contains(err.Error(), gitLabToken))
}

func TestGitLabRedirectToAnotherHostNeverReceivesTheToken(t *testing.T) {
	target := newFakeProvider(t, scriptedRoute{target: "GET /api/v4/user", status: 200, body: `{"username":"intruso"}`})
	redirecting := newRedirectingServer(t, target.URL()+"/api/v4/user")
	settings := settingsFor(t, models.GitLab, redirecting, "glpat-secret")

	_, err := newGitLabClient().CurrentUser(context.Background(), settings)

	require.Equal(t, codeerrors.Redirected(models.GitLab, 302), err)
	for _, seen := range target.requests() {
		require.Empty(t, seen.privateToken)
	}
}
