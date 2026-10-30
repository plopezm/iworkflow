//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	codeerrors "iworkflow/internal/modules/code_hosting/domain/errors"
	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/domain/ports/outbound"
)

const (
	gitLabTokenHeader = "PRIVATE-TOKEN"
	gitLabJobsPerPage = "100"
)

// GitLabClient talks to the REST API v4 of GitLab.
type GitLabClient struct {
	clients *ClientManager
}

var _ outbound.CodeHostClient = (*GitLabClient)(nil)

// NewGitLabClient builds the client over the shared HTTP clients.
func NewGitLabClient(clients *ClientManager) *GitLabClient {
	return &GitLabClient{clients: clients}
}

// do sends one request to the API of the instance and returns the body of a successful answer.
func (c *GitLabClient) do(ctx context.Context, settings models.CodeHostSettings, method string, path string, body []byte) ([]byte, error) {
	request, err := newRequest(ctx, method, settings.APIURL()+path, map[string]string{gitLabTokenHeader: settings.Token()}, body)
	if err != nil {
		return nil, err
	}
	return send(c.clients.Client(settings.InsecureTLS), models.GitLab, request)
}

// gitLabMergeRequestJSON is the part of a GitLab merge request this module reads.
type gitLabMergeRequestJSON struct {
	IID                 uint64  `json:"iid"`
	Title               string  `json:"title"`
	State               string  `json:"state"`
	SourceBranch        string  `json:"source_branch"`
	TargetBranch        string  `json:"target_branch"`
	WebURL              string  `json:"web_url"`
	MergeStatus         *string `json:"merge_status"`
	DetailedMergeStatus *string `json:"detailed_merge_status"`
	HasConflicts        *bool   `json:"has_conflicts"`
}

func (j gitLabMergeRequestJSON) toDomain() models.MergeRequest {
	// The detailed status wins over the plain one when GitLab sends it.
	mergeStatus := ""
	switch {
	case j.DetailedMergeStatus != nil:
		mergeStatus = *j.DetailedMergeStatus
	case j.MergeStatus != nil:
		mergeStatus = *j.MergeStatus
	}
	return models.MergeRequest{
		IID:          j.IID,
		Title:        j.Title,
		State:        j.State,
		SourceBranch: j.SourceBranch,
		TargetBranch: j.TargetBranch,
		WebURL:       j.WebURL,
		MergeStatus:  mergeStatus,
		HasConflicts: j.HasConflicts != nil && *j.HasConflicts,
	}
}

type gitLabPipelineJSON struct {
	ID     uint64 `json:"id"`
	Status string `json:"status"`
	WebURL string `json:"web_url"`
}

type gitLabJobJSON struct {
	ID     uint64 `json:"id"`
	Name   string `json:"name"`
	Stage  string `json:"stage"`
	Status string `json:"status"`
	WebURL string `json:"web_url"`
}

type gitLabUserJSON struct {
	Username string `json:"username"`
}

// CurrentUser returns the username the token belongs to.
func (c *GitLabClient) CurrentUser(ctx context.Context, settings models.CodeHostSettings) (string, error) {
	body, err := c.do(ctx, settings, http.MethodGet, "/user", nil)
	if err != nil {
		return "", err
	}
	var user gitLabUserJSON
	if err := decodeJSON(body, &user); err != nil {
		return "", err
	}
	return user.Username, nil
}

// BranchExists maps a 404 answer to false; any other failure is reported.
func (c *GitLabClient) BranchExists(ctx context.Context, settings models.CodeHostSettings, project string, branch string) (bool, error) {
	path := fmt.Sprintf("/projects/%s/repository/branches/%s", encodeComponent(project), encodeComponent(branch))
	if _, err := c.do(ctx, settings, http.MethodGet, path, nil); err != nil {
		if codeerrors.IsAPIStatus(err, http.StatusNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// FindLatestMergeRequest prefers the newest open request and falls back to the newest of any state.
func (c *GitLabClient) FindLatestMergeRequest(ctx context.Context, settings models.CodeHostSettings, project string, sourceBranch string) (*models.MergeRequest, error) {
	path := withQuery(
		fmt.Sprintf("/projects/%s/merge_requests", encodeComponent(project)),
		[2]string{"source_branch", sourceBranch},
		[2]string{"order_by", "created_at"},
		[2]string{"sort", "desc"},
	)
	body, err := c.do(ctx, settings, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var list []gitLabMergeRequestJSON
	if err := decodeJSON(body, &list); err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	index := 0
	for position, candidate := range list {
		if candidate.State == "opened" {
			index = position
			break
		}
	}
	found := list[index].toDomain()
	return &found, nil
}

// CreateMergeRequest opens a merge request from the source to the target branch.
func (c *GitLabClient) CreateMergeRequest(ctx context.Context, settings models.CodeHostSettings, project string, request models.NewMergeRequest) (models.MergeRequest, error) {
	payload, err := json.Marshal(map[string]string{
		"source_branch": request.SourceBranch,
		"target_branch": request.TargetBranch,
		"title":         request.Title,
		"description":   request.Description,
	})
	if err != nil {
		return models.MergeRequest{}, codeerrors.Repository(err.Error())
	}
	body, err := c.do(ctx, settings, http.MethodPost, fmt.Sprintf("/projects/%s/merge_requests", encodeComponent(project)), payload)
	if err != nil {
		return models.MergeRequest{}, err
	}
	return decodeMergeRequest(body)
}

// GetMergeRequest reads one merge request by its iid.
func (c *GitLabClient) GetMergeRequest(ctx context.Context, settings models.CodeHostSettings, project string, iid uint64) (models.MergeRequest, error) {
	body, err := c.do(ctx, settings, http.MethodGet, fmt.Sprintf("/projects/%s/merge_requests/%d", encodeComponent(project), iid), nil)
	if err != nil {
		return models.MergeRequest{}, err
	}
	return decodeMergeRequest(body)
}

// LatestPipeline returns the newest pipeline of the merge request with its jobs, or nil.
func (c *GitLabClient) LatestPipeline(ctx context.Context, settings models.CodeHostSettings, project string, iid uint64) (*models.Pipeline, error) {
	encoded := encodeComponent(project)
	body, err := c.do(ctx, settings, http.MethodGet, fmt.Sprintf("/projects/%s/merge_requests/%d/pipelines", encoded, iid), nil)
	if err != nil {
		return nil, err
	}
	var pipelines []gitLabPipelineJSON
	if err := decodeJSON(body, &pipelines); err != nil {
		return nil, err
	}
	// GitLab lists them from the newest to the oldest.
	if len(pipelines) == 0 {
		return nil, nil
	}
	latest := pipelines[0]
	jobsPath := withQuery(
		fmt.Sprintf("/projects/%s/pipelines/%d/jobs", encoded, latest.ID),
		[2]string{"per_page", gitLabJobsPerPage},
	)
	jobsBody, err := c.do(ctx, settings, http.MethodGet, jobsPath, nil)
	if err != nil {
		return nil, err
	}
	var jobs []gitLabJobJSON
	if err := decodeJSON(jobsBody, &jobs); err != nil {
		return nil, err
	}
	pipeline := models.Pipeline{ID: latest.ID, Status: latest.Status, WebURL: latest.WebURL, Jobs: make([]models.PipelineJob, 0, len(jobs))}
	for _, job := range jobs {
		pipeline.Jobs = append(pipeline.Jobs, models.PipelineJob{
			ID:     job.ID,
			Name:   job.Name,
			Stage:  job.Stage,
			Status: job.Status,
			WebURL: job.WebURL,
		})
	}
	return &pipeline, nil
}

// MergeMergeRequest merges the request and returns its state after the merge.
func (c *GitLabClient) MergeMergeRequest(ctx context.Context, settings models.CodeHostSettings, project string, iid uint64) (models.MergeRequest, error) {
	body, err := c.do(ctx, settings, http.MethodPut, fmt.Sprintf("/projects/%s/merge_requests/%d/merge", encodeComponent(project), iid), nil)
	if err != nil {
		return models.MergeRequest{}, err
	}
	return decodeMergeRequest(body)
}

// decodeMergeRequest converts a merge request answer into the domain model.
func decodeMergeRequest(body []byte) (models.MergeRequest, error) {
	var merged gitLabMergeRequestJSON
	if err := decodeJSON(body, &merged); err != nil {
		return models.MergeRequest{}, err
	}
	return merged.toDomain(), nil
}
