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
	gitHubToken = "ghp_test"
	gitHubPulls = "/api/v3/repos/acme/app/pulls"
	// gitHubPullOpen is a pull request on the Enterprise API (any host other than github.com).
	gitHubPullOpen   = `{"number":7,"title":"T","state":"open","html_url":"http://gh/acme/app/pull/7","merged":false,"merged_at":null,"mergeable":true,"mergeable_state":"clean","head":{"ref":"pi/x","sha":"abc123"},"base":{"ref":"main","sha":"def"}}`
	gitHubPullMerged = `{"number":7,"title":"T","state":"closed","html_url":"http://gh/acme/app/pull/7","merged":true,"merged_at":"2026-01-01T00:00:00Z","mergeable":null,"mergeable_state":"unknown","head":{"ref":"pi/x","sha":"abc123"},"base":{"ref":"main","sha":"def"}}`
	gitHubListClosed = `{"number":5,"title":"Old","state":"closed","html_url":"http://gh/acme/app/pull/5","head":{"ref":"pi/x","sha":"s5"},"base":{"ref":"main","sha":"d"}}`
	gitHubListOpen   = `{"number":7,"title":"T","state":"open","html_url":"http://gh/acme/app/pull/7","head":{"ref":"pi/x","sha":"abc123"},"base":{"ref":"main","sha":"d"}}`
)

func newGitHubClient() *api.GitHubClient {
	return api.NewGitHubClient(api.NewClientManager())
}

func gitHubSettings(t *testing.T, provider *fakeProvider) models.CodeHostSettings {
	t.Helper()
	return settingsFor(t, models.GitHub, provider.URL(), gitHubToken)
}

// assertGitHubHeaders checks the headers every GitHub request must carry.
func assertGitHubHeaders(t *testing.T, seen recordedRequest) {
	t.Helper()
	require.Equal(t, "Bearer "+gitHubToken, seen.authorization)
	require.Equal(t, "iworkflow", seen.userAgent)
	require.Equal(t, "application/vnd.github+json", seen.accept)
	require.Equal(t, "2022-11-28", seen.apiVersion)
}

func TestGitHubCurrentUserSendsGitHubHeadersAndReturnsTheLogin(t *testing.T) {
	provider := newFakeProvider(t, scriptedRoute{target: "GET /api/v3/user", status: 200, body: `{"login":"pablo"}`})

	user, err := newGitHubClient().CurrentUser(context.Background(), gitHubSettings(t, provider))

	require.NoError(t, err)
	require.Equal(t, "pablo", user)
	assertGitHubHeaders(t, provider.requests()[0])
}

func TestGitHubBranchExistsMaps404ToFalse(t *testing.T) {
	provider := newFakeProvider(t, scriptedRoute{
		target: "GET /api/v3/repos/acme/app/git/ref/heads/pi/x",
		status: 200,
		body:   `{"ref":"refs/heads/pi/x"}`,
	})
	client := newGitHubClient()
	settings := gitHubSettings(t, provider)

	exists, err := client.BranchExists(context.Background(), settings, "acme/app", "pi/x")
	require.NoError(t, err)
	require.True(t, exists)
	missing, err := client.BranchExists(context.Background(), settings, "acme/app", "otra")
	require.NoError(t, err)
	require.False(t, missing)
	require.Len(t, provider.requests(), 2)
}

func TestGitHubFindLatestPrefersTheOpenPullAndRereadsIt(t *testing.T) {
	list := "[" + gitHubListClosed + "," + gitHubListOpen + "]"
	provider := newFakeProvider(t,
		scriptedRoute{
			target: "GET " + gitHubPulls + "?head=acme%3Api%2Fx&state=all&sort=created&direction=desc&per_page=30",
			status: 200,
			body:   list,
		},
		scriptedRoute{target: "GET /api/v3/repos/acme/app/pulls/7", status: 200, body: gitHubPullOpen},
	)

	found, err := newGitHubClient().FindLatestMergeRequest(context.Background(), gitHubSettings(t, provider), "acme/app", "pi/x")

	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, uint64(7), found.IID)
	require.Equal(t, "opened", found.State)
	require.Equal(t, "mergeable", found.MergeStatus)
	require.Equal(t, "http://gh/acme/app/pull/7", found.WebURL)
	require.Equal(t, "pi/x", found.SourceBranch)
	require.Equal(t, "main", found.TargetBranch)
	seen := provider.requests()
	require.Len(t, seen, 2)
	assertGitHubHeaders(t, seen[0])
	assertGitHubHeaders(t, seen[1])
}

func TestGitHubFindLatestFallsBackToTheMostRecentAndIsNilWhenEmpty(t *testing.T) {
	provider := newFakeProvider(t,
		scriptedRoute{
			target: "GET " + gitHubPulls + "?head=acme%3Acerrada&state=all&sort=created&direction=desc&per_page=30",
			status: 200,
			body:   "[" + gitHubListClosed + "]",
		},
		scriptedRoute{
			target: "GET " + gitHubPulls + "?head=acme%3Avacia&state=all&sort=created&direction=desc&per_page=30",
			status: 200,
			body:   "[]",
		},
		scriptedRoute{target: "GET /api/v3/repos/acme/app/pulls/5", status: 200, body: gitHubPullMerged},
	)
	client := newGitHubClient()
	settings := gitHubSettings(t, provider)

	found, err := client.FindLatestMergeRequest(context.Background(), settings, "acme/app", "cerrada")
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, "merged", found.State)
	missing, err := client.FindLatestMergeRequest(context.Background(), settings, "acme/app", "vacia")
	require.NoError(t, err)
	require.Nil(t, missing)
}

func gitHubNewPull() models.NewMergeRequest {
	return models.NewMergeRequest{SourceBranch: "pi/x", TargetBranch: "main", Title: "T", Description: "D"}
}

func TestGitHubCreateMergeRequestPostsTheExpectedBody(t *testing.T) {
	provider := newFakeProvider(t, scriptedRoute{target: "POST " + gitHubPulls, status: 201, body: gitHubPullOpen})

	created, err := newGitHubClient().CreateMergeRequest(context.Background(), gitHubSettings(t, provider), "acme/app", gitHubNewPull())

	require.NoError(t, err)
	require.Equal(t, uint64(7), created.IID)
	seen := provider.requests()[0]
	assertGitHubHeaders(t, seen)
	var body map[string]string
	require.NoError(t, json.Unmarshal([]byte(seen.body), &body))
	require.Equal(t, map[string]string{"title": "T", "head": "pi/x", "base": "main", "body": "D"}, body)
}

func TestGitHubCreateWithoutNewCommitsIsAClearError(t *testing.T) {
	provider := newFakeProvider(t, scriptedRoute{
		target: "POST " + gitHubPulls,
		status: 422,
		body:   `{"message":"Validation Failed","errors":[{"resource":"PullRequest","code":"custom","message":"No commits between main and pi/x"}]}`,
	})

	_, err := newGitHubClient().CreateMergeRequest(context.Background(), gitHubSettings(t, provider), "acme/app", gitHubNewPull())

	require.Equal(t, codeerrors.NoCommitsToPropose("pi/x", "main"), err)
}

func TestGitHubGetMergeRequestMapsAMergedPull(t *testing.T) {
	provider := newFakeProvider(t, scriptedRoute{target: "GET /api/v3/repos/acme/app/pulls/7", status: 200, body: gitHubPullMerged})

	mergeRequest, err := newGitHubClient().GetMergeRequest(context.Background(), gitHubSettings(t, provider), "acme/app", 7)

	require.NoError(t, err)
	require.Equal(t, "merged", mergeRequest.State)
	require.False(t, mergeRequest.IsOpen())
}

func TestGitHubMergePutsThenRereadsThePull(t *testing.T) {
	provider := newFakeProvider(t,
		scriptedRoute{
			target: "PUT /api/v3/repos/acme/app/pulls/7/merge",
			status: 200,
			body:   `{"merged":true,"sha":"abc","message":"Pull Request successfully merged"}`,
		},
		scriptedRoute{target: "GET /api/v3/repos/acme/app/pulls/7", status: 200, body: gitHubPullMerged},
	)

	mergeRequest, err := newGitHubClient().MergeMergeRequest(context.Background(), gitHubSettings(t, provider), "acme/app", 7)

	require.NoError(t, err)
	require.Equal(t, "merged", mergeRequest.State)
	seen := provider.requests()
	require.Equal(t, "PUT", seen[0].method)
	require.Equal(t, "{}", seen[0].body)
	require.Equal(t, "GET", seen[1].method)
}

func TestGitHubMergeRefusalSurfacesTheProviderMessage(t *testing.T) {
	provider := newFakeProvider(t, scriptedRoute{
		target: "PUT /api/v3/repos/acme/app/pulls/8/merge",
		status: 405,
		body:   `{"message":"Pull Request is not mergeable"}`,
	})

	_, err := newGitHubClient().MergeMergeRequest(context.Background(), gitHubSettings(t, provider), "acme/app", 8)

	require.Equal(t, codeerrors.API(models.GitHub, 405, "Pull Request is not mergeable"), err)
}

func TestGitHubLatestPipelineMapsTheRunAndItsJobs(t *testing.T) {
	provider := newFakeProvider(t,
		scriptedRoute{target: "GET /api/v3/repos/acme/app/pulls/7", status: 200, body: gitHubPullOpen},
		scriptedRoute{
			target: "GET /api/v3/repos/acme/app/actions/runs?head_sha=abc123&per_page=1",
			status: 200,
			body:   `{"total_count":1,"workflow_runs":[{"id":22,"name":"CI","status":"completed","conclusion":"failure","html_url":"http://gh/runs/22"}]}`,
		},
		scriptedRoute{
			target: "GET /api/v3/repos/acme/app/actions/runs/22/jobs?per_page=100",
			status: 200,
			body:   `{"total_count":2,"jobs":[{"id":1,"name":"build","status":"completed","conclusion":"success","html_url":"http://gh/j/1"},{"id":2,"name":"test","status":"in_progress","conclusion":null,"html_url":"http://gh/j/2"}]}`,
		},
	)

	pipeline, err := newGitHubClient().LatestPipeline(context.Background(), gitHubSettings(t, provider), "acme/app", 7)

	require.NoError(t, err)
	require.NotNil(t, pipeline)
	require.Equal(t, uint64(22), pipeline.ID)
	require.Equal(t, "failed", pipeline.Status)
	require.Equal(t, "http://gh/runs/22", pipeline.WebURL)
	require.Len(t, pipeline.Jobs, 2)
	require.Equal(t, "CI", pipeline.Jobs[0].Stage)
	require.Equal(t, "success", pipeline.Jobs[0].Status)
	require.Equal(t, "running", pipeline.Jobs[1].Status)
}

func TestGitHubLatestPipelineIsNilWithoutRuns(t *testing.T) {
	provider := newFakeProvider(t,
		scriptedRoute{target: "GET /api/v3/repos/acme/app/pulls/7", status: 200, body: gitHubPullOpen},
		scriptedRoute{
			target: "GET /api/v3/repos/acme/app/actions/runs?head_sha=abc123&per_page=1",
			status: 200,
			body:   `{"total_count":0,"workflow_runs":[]}`,
		},
	)

	pipeline, err := newGitHubClient().LatestPipeline(context.Background(), gitHubSettings(t, provider), "acme/app", 7)

	require.NoError(t, err)
	require.Nil(t, pipeline)
	require.Len(t, provider.requests(), 2)
}

func TestGitHubUnauthorizedReportsTheGitHubError(t *testing.T) {
	provider := newFakeProvider(t, scriptedRoute{target: "GET /api/v3/user", status: 401, body: `{"message":"Bad credentials"}`})

	_, err := newGitHubClient().CurrentUser(context.Background(), gitHubSettings(t, provider))

	require.Equal(t, codeerrors.API(models.GitHub, 401, "Bad credentials"), err)
}

func TestGitHubNetworkFailureDoesNotLeakTheToken(t *testing.T) {
	settings := settingsFor(t, models.GitHub, "http://127.0.0.1:1", gitHubToken)

	_, err := newGitHubClient().CurrentUser(context.Background(), settings)

	require.ErrorIs(t, err, codeerrors.Network(""))
	require.False(t, strings.Contains(err.Error(), gitHubToken))
}

func TestGitHubRedirectToAnotherHostNeverReceivesTheToken(t *testing.T) {
	target := newFakeProvider(t, scriptedRoute{target: "GET /api/v3/user", status: 200, body: `{"login":"intruso"}`})
	redirecting := newRedirectingServer(t, target.URL()+"/api/v3/user")
	settings := settingsFor(t, models.GitHub, redirecting, "ghp_secret")

	_, err := newGitHubClient().CurrentUser(context.Background(), settings)

	require.Equal(t, codeerrors.Redirected(models.GitHub, 302), err)
	for _, seen := range target.requests() {
		require.Empty(t, seen.authorization)
	}
}
