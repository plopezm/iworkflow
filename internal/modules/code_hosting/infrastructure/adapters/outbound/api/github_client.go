//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	codeerrors "iworkflow/internal/modules/code_hosting/domain/errors"
	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/domain/ports/outbound"
)

const (
	gitHubAccept        = "application/vnd.github+json"
	gitHubAPIVersionKey = "X-GitHub-Api-Version"
	gitHubAPIVersion    = "2022-11-28"
	// gitHubUserAgent is required: GitHub rejects requests without one.
	gitHubUserAgent = "iworkflow"
	gitHubPullsPage = "30"
	gitHubJobsPage  = "100"
	// gitHubNoCommits is the message GitHub answers with when a pull request adds no commits.
	gitHubNoCommits = "No commits between"
)

// GitHubClient talks to the REST API v3 of GitHub (github.com or Enterprise).
type GitHubClient struct {
	clients *ClientManager
}

var _ outbound.CodeHostClient = (*GitHubClient)(nil)

// NewGitHubClient builds the client over the shared HTTP clients.
func NewGitHubClient(clients *ClientManager) *GitHubClient {
	return &GitHubClient{clients: clients}
}

// do sends one request to the API of the instance and returns the body of a successful answer.
func (c *GitHubClient) do(ctx context.Context, settings models.CodeHostSettings, method string, path string, body []byte) ([]byte, error) {
	headers := map[string]string{
		"Authorization":     "Bearer " + settings.Token(),
		"Accept":            gitHubAccept,
		gitHubAPIVersionKey: gitHubAPIVersion,
		"User-Agent":        gitHubUserAgent,
	}
	request, err := newRequest(ctx, method, settings.APIURL()+path, headers, body)
	if err != nil {
		return nil, err
	}
	return send(c.clients.Client(settings.InsecureTLS), models.GitHub, request)
}

// repoPath is `/repos/{owner}/{repo}`: each segment is encoded and the separating slash is kept.
func repoPath(project string) string {
	segments := strings.Split(project, "/")
	for index, segment := range segments {
		segments[index] = encodeComponent(segment)
	}
	return "/repos/" + strings.Join(segments, "/")
}

// refPath encodes a branch name segment by segment, so `pi/x` keeps its slash.
func refPath(branch string) string {
	segments := strings.Split(branch, "/")
	for index, segment := range segments {
		segments[index] = encodeComponent(segment)
	}
	return strings.Join(segments, "/")
}

// mapPullState maps `open` to `opened`; a closed pull request that was merged is `merged`.
func mapPullState(state string, merged bool) string {
	switch {
	case state == "open":
		return "opened"
	case merged:
		return "merged"
	default:
		return "closed"
	}
}

// mapMergeStatus translates `mergeable_state` to the neutral vocabulary.
func mapMergeStatus(mergeableState *string) string {
	if mergeableState == nil {
		return "checking"
	}
	switch *mergeableState {
	case "clean", "behind", "unstable", "has_hooks":
		return "mergeable"
	case "dirty":
		return "conflict"
	case "blocked":
		return "blocked_status"
	case "draft":
		return "not_approved"
	default:
		return "checking"
	}
}

// mapCIStatus translates the status and conclusion of a run or job to the neutral vocabulary.
func mapCIStatus(status string, conclusion *string) string {
	switch status {
	case "queued", "waiting", "pending", "requested":
		return "pending"
	case "in_progress":
		return "running"
	case "completed":
		if conclusion == nil {
			return "pending"
		}
		switch *conclusion {
		case "success":
			return "success"
		case "failure", "timed_out", "startup_failure":
			return "failed"
		case "cancelled":
			return "canceled"
		case "skipped", "neutral", "stale":
			return "skipped"
		case "action_required":
			return "manual"
		default:
			return "pending"
		}
	default:
		return "pending"
	}
}

type refJSON struct {
	Name string `json:"ref"`
	SHA  string `json:"sha"`
}

type pullJSON struct {
	Number         uint64  `json:"number"`
	Title          string  `json:"title"`
	State          string  `json:"state"`
	HTMLURL        string  `json:"html_url"`
	Merged         *bool   `json:"merged"`
	MergedAt       *string `json:"merged_at"`
	MergeableState *string `json:"mergeable_state"`
	Head           refJSON `json:"head"`
	Base           refJSON `json:"base"`
}

func (p pullJSON) toDomain() models.MergeRequest {
	merged := (p.Merged != nil && *p.Merged) || p.MergedAt != nil
	return models.MergeRequest{
		IID:          p.Number,
		Title:        p.Title,
		State:        mapPullState(p.State, merged),
		SourceBranch: p.Head.Name,
		TargetBranch: p.Base.Name,
		WebURL:       p.HTMLURL,
		MergeStatus:  mapMergeStatus(p.MergeableState),
		HasConflicts: p.MergeableState != nil && *p.MergeableState == "dirty",
	}
}

type gitHubUserJSON struct {
	Login string `json:"login"`
}

type runJSON struct {
	ID         uint64  `json:"id"`
	Name       *string `json:"name"`
	Status     *string `json:"status"`
	Conclusion *string `json:"conclusion"`
	HTMLURL    string  `json:"html_url"`
}

type runsJSON struct {
	WorkflowRuns []runJSON `json:"workflow_runs"`
}

type gitHubJobJSON struct {
	ID         uint64  `json:"id"`
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	Conclusion *string `json:"conclusion"`
	HTMLURL    *string `json:"html_url"`
}

type jobsJSON struct {
	Jobs []gitHubJobJSON `json:"jobs"`
}

// CurrentUser returns the login the token belongs to.
func (c *GitHubClient) CurrentUser(ctx context.Context, settings models.CodeHostSettings) (string, error) {
	body, err := c.do(ctx, settings, http.MethodGet, "/user", nil)
	if err != nil {
		return "", err
	}
	var user gitHubUserJSON
	if err := decodeJSON(body, &user); err != nil {
		return "", err
	}
	return user.Login, nil
}

// BranchExists maps a 404 answer to false; any other failure is reported.
func (c *GitHubClient) BranchExists(ctx context.Context, settings models.CodeHostSettings, project string, branch string) (bool, error) {
	path := fmt.Sprintf("%s/git/ref/heads/%s", repoPath(project), refPath(branch))
	if _, err := c.do(ctx, settings, http.MethodGet, path, nil); err != nil {
		if codeerrors.IsAPIStatus(err, http.StatusNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// FindLatestMergeRequest prefers the newest open pull request and falls back to the newest of any
// state. The list lacks the mergeability fields, so the chosen pull request is read again.
func (c *GitHubClient) FindLatestMergeRequest(ctx context.Context, settings models.CodeHostSettings, project string, sourceBranch string) (*models.MergeRequest, error) {
	owner, _, _ := strings.Cut(project, "/")
	path := withQuery(
		repoPath(project)+"/pulls",
		[2]string{"head", owner + ":" + sourceBranch},
		[2]string{"state", "all"},
		[2]string{"sort", "created"},
		[2]string{"direction", "desc"},
		[2]string{"per_page", gitHubPullsPage},
	)
	body, err := c.do(ctx, settings, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var list []pullJSON
	if err := decodeJSON(body, &list); err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	index := 0
	for position, candidate := range list {
		if candidate.State == "open" {
			index = position
			break
		}
	}
	pull, err := c.pull(ctx, settings, project, list[index].Number)
	if err != nil {
		return nil, err
	}
	merged := pull.toDomain()
	return &merged, nil
}

// CreateMergeRequest opens a pull request. A request with no new commits fails with
// NoCommitsToPropose, which tells the user what to do instead of showing the raw provider text.
func (c *GitHubClient) CreateMergeRequest(ctx context.Context, settings models.CodeHostSettings, project string, request models.NewMergeRequest) (models.MergeRequest, error) {
	payload, err := json.Marshal(map[string]string{
		"title": request.Title,
		"head":  request.SourceBranch,
		"base":  request.TargetBranch,
		"body":  request.Description,
	})
	if err != nil {
		return models.MergeRequest{}, codeerrors.Repository(err.Error())
	}
	body, err := c.do(ctx, settings, http.MethodPost, repoPath(project)+"/pulls", payload)
	if err != nil {
		return models.MergeRequest{}, noCommitsError(err, request.SourceBranch, request.TargetBranch)
	}
	var pull pullJSON
	if err := decodeJSON(body, &pull); err != nil {
		return models.MergeRequest{}, err
	}
	return pull.toDomain(), nil
}

// GetMergeRequest reads one pull request by its number.
func (c *GitHubClient) GetMergeRequest(ctx context.Context, settings models.CodeHostSettings, project string, iid uint64) (models.MergeRequest, error) {
	pull, err := c.pull(ctx, settings, project, iid)
	if err != nil {
		return models.MergeRequest{}, err
	}
	return pull.toDomain(), nil
}

// LatestPipeline reads the newest workflow run of the head commit of the pull request, with its jobs.
func (c *GitHubClient) LatestPipeline(ctx context.Context, settings models.CodeHostSettings, project string, iid uint64) (*models.Pipeline, error) {
	pull, err := c.pull(ctx, settings, project, iid)
	if err != nil {
		return nil, err
	}
	repo := repoPath(project)
	runsPath := withQuery(repo+"/actions/runs",
		[2]string{"head_sha", pull.Head.SHA},
		[2]string{"per_page", "1"},
	)
	runsBody, err := c.do(ctx, settings, http.MethodGet, runsPath, nil)
	if err != nil {
		return nil, err
	}
	var runs runsJSON
	if err := decodeJSON(runsBody, &runs); err != nil {
		return nil, err
	}
	// GitHub lists them from the newest to the oldest.
	if len(runs.WorkflowRuns) == 0 {
		return nil, nil
	}
	run := runs.WorkflowRuns[0]
	jobsPath := withQuery(fmt.Sprintf("%s/actions/runs/%d/jobs", repo, run.ID),
		[2]string{"per_page", gitHubJobsPage},
	)
	jobsBody, err := c.do(ctx, settings, http.MethodGet, jobsPath, nil)
	if err != nil {
		return nil, err
	}
	var jobs jobsJSON
	if err := decodeJSON(jobsBody, &jobs); err != nil {
		return nil, err
	}
	stage := ""
	if run.Name != nil {
		stage = *run.Name
	}
	pipeline := models.Pipeline{
		ID:     run.ID,
		Status: mapCIStatus(stringOrEmpty(run.Status), run.Conclusion),
		WebURL: run.HTMLURL,
		Jobs:   make([]models.PipelineJob, 0, len(jobs.Jobs)),
	}
	for _, job := range jobs.Jobs {
		webURL := run.HTMLURL
		if job.HTMLURL != nil {
			webURL = *job.HTMLURL
		}
		pipeline.Jobs = append(pipeline.Jobs, models.PipelineJob{
			ID:     job.ID,
			Name:   job.Name,
			Stage:  stage,
			Status: mapCIStatus(job.Status, job.Conclusion),
			WebURL: webURL,
		})
	}
	return &pipeline, nil
}

// MergeMergeRequest merges the pull request. The merge answer is not a pull request, so the state
// is read again to return the final one.
func (c *GitHubClient) MergeMergeRequest(ctx context.Context, settings models.CodeHostSettings, project string, iid uint64) (models.MergeRequest, error) {
	path := fmt.Sprintf("%s/pulls/%d/merge", repoPath(project), iid)
	if _, err := c.do(ctx, settings, http.MethodPut, path, []byte("{}")); err != nil {
		return models.MergeRequest{}, err
	}
	return c.GetMergeRequest(ctx, settings, project, iid)
}

// pull reads one pull request.
func (c *GitHubClient) pull(ctx context.Context, settings models.CodeHostSettings, project string, number uint64) (pullJSON, error) {
	body, err := c.do(ctx, settings, http.MethodGet, fmt.Sprintf("%s/pulls/%d", repoPath(project), number), nil)
	if err != nil {
		return pullJSON{}, err
	}
	var pull pullJSON
	if err := decodeJSON(body, &pull); err != nil {
		return pullJSON{}, err
	}
	return pull, nil
}

// noCommitsError turns GitHub's "No commits between" 422 into NoCommitsToPropose and keeps any other
// error as it is.
func noCommitsError(err error, branch string, base string) error {
	var apiErr codeerrors.Error
	if errors.As(err, &apiErr) && apiErr.Status() == http.StatusUnprocessableEntity && strings.Contains(apiErr.Message(), gitHubNoCommits) {
		return codeerrors.NoCommitsToPropose(branch, base)
	}
	return err
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
