package application_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/code_hosting/application"
	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/domain/ports/outbound"
)

const (
	taskID  = "task-1"
	project = "grupo/proyecto"
	token   = "tok"
)

// fakeSettingsRepository is an in-memory CodeHostSettingsRepository that records the saves.
type fakeSettingsRepository struct {
	stored  []models.CodeHostSettings
	getErr  error
	listErr error
	saveErr error
	saved   []models.CodeHostSettings
}

func (r *fakeSettingsRepository) Get(_ context.Context, provider models.HostingProvider) (*models.CodeHostSettings, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	for _, settings := range r.stored {
		if settings.Provider() == provider {
			found := settings
			return &found, nil
		}
	}
	return nil, nil
}

func (r *fakeSettingsRepository) List(_ context.Context) ([]models.CodeHostSettings, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return append([]models.CodeHostSettings(nil), r.stored...), nil
}

func (r *fakeSettingsRepository) Save(_ context.Context, settings models.CodeHostSettings) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.saved = append(r.saved, settings)
	return nil
}

// fakeTaskSourceLookup returns a fixed task source; nil means the task does not exist.
type fakeTaskSourceLookup struct {
	source *models.TaskSource
	err    error
}

func (f *fakeTaskSourceLookup) Find(_ context.Context, _ string) (*models.TaskSource, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.source, nil
}

// publishCall is one push recorded by fakeGitRemote.
type publishCall struct {
	repo    string
	branch  string
	project string
	token   string
}

// fakeGitRemote records every git operation. events, when set, collects the operation names in order.
type fakeGitRemote struct {
	origin      string
	originErr   error
	ensureAdded bool
	ensureErr   error
	publishErr  error
	ensureCalls [][2]string
	publishes   []publishCall
	events      *[]string
}

func (r *fakeGitRemote) record(event string) {
	if r.events != nil {
		*r.events = append(*r.events, event)
	}
}

func (r *fakeGitRemote) OriginURL(_ context.Context, _ string) (string, error) {
	if r.originErr != nil {
		return "", r.originErr
	}
	return r.origin, nil
}

func (r *fakeGitRemote) PublishBranch(_ context.Context, repo string, branch string, settings models.CodeHostSettings, project string) error {
	r.record("publish")
	r.publishes = append(r.publishes, publishCall{repo: repo, branch: branch, project: project, token: settings.Token()})
	return r.publishErr
}

func (r *fakeGitRemote) EnsureCommitsAhead(_ context.Context, _ string, branch string, base string) (bool, error) {
	r.ensureCalls = append(r.ensureCalls, [2]string{branch, base})
	if r.ensureErr != nil {
		return false, r.ensureErr
	}
	return r.ensureAdded, nil
}

// fakeClient is a CodeHostClient with canned answers. Branches missing from branches do not exist.
type fakeClient struct {
	user        string
	userErr     error
	userCalls   int
	branches    map[string]bool
	findResult  *models.MergeRequest
	findErr     error
	created     []models.NewMergeRequest
	createdMR   models.MergeRequest
	createErr   error
	merged      []uint64
	mergedMR    models.MergeRequest
	mergeErr    error
	pipeline    *models.Pipeline
	pipelineErr error
	pipelines   int
	events      *[]string
}

func (c *fakeClient) CurrentUser(_ context.Context, _ models.CodeHostSettings) (string, error) {
	c.userCalls++
	return c.user, c.userErr
}

func (c *fakeClient) BranchExists(_ context.Context, _ models.CodeHostSettings, _ string, branch string) (bool, error) {
	return c.branches[branch], nil
}

func (c *fakeClient) FindLatestMergeRequest(_ context.Context, _ models.CodeHostSettings, _ string, _ string) (*models.MergeRequest, error) {
	if c.findErr != nil {
		return nil, c.findErr
	}
	return c.findResult, nil
}

func (c *fakeClient) CreateMergeRequest(_ context.Context, _ models.CodeHostSettings, _ string, request models.NewMergeRequest) (models.MergeRequest, error) {
	c.created = append(c.created, request)
	return c.createdMR, c.createErr
}

func (c *fakeClient) GetMergeRequest(_ context.Context, _ models.CodeHostSettings, _ string, _ uint64) (models.MergeRequest, error) {
	return c.mergedMR, nil
}

func (c *fakeClient) LatestPipeline(_ context.Context, _ models.CodeHostSettings, _ string, _ uint64) (*models.Pipeline, error) {
	c.pipelines++
	if c.pipelineErr != nil {
		return nil, c.pipelineErr
	}
	return c.pipeline, nil
}

func (c *fakeClient) MergeMergeRequest(_ context.Context, _ models.CodeHostSettings, _ string, iid uint64) (models.MergeRequest, error) {
	if c.events != nil {
		*c.events = append(*c.events, "merge")
	}
	c.merged = append(c.merged, iid)
	return c.mergedMR, c.mergeErr
}

// fakeClientFactory hands out the client of each provider, or the fallback when there is none.
type fakeClientFactory struct {
	byProvider map[models.HostingProvider]outbound.CodeHostClient
	fallback   outbound.CodeHostClient
}

func (f *fakeClientFactory) ClientFor(provider models.HostingProvider) outbound.CodeHostClient {
	if client, ok := f.byProvider[provider]; ok {
		return client
	}
	return f.fallback
}

// fixture wires the fakes into the resolver. The defaults describe a GitLab task that is ready to
// be proposed: origin points at the configured GitLab, and both branches are published.
type fixture struct {
	settings *fakeSettingsRepository
	tasks    *fakeTaskSourceLookup
	remote   *fakeGitRemote
	client   *fakeClient
	factory  *fakeClientFactory
	events   *[]string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	events := []string{}
	client := &fakeClient{
		user:      "pablo",
		branches:  map[string]bool{"main": true, "pi/feature": true},
		events:    &events,
		createdMR: mergeRequest(9, "opened"),
		mergedMR:  mergeRequest(7, "merged"),
	}
	return &fixture{
		settings: &fakeSettingsRepository{stored: []models.CodeHostSettings{gitlabSettings(t)}},
		tasks:    &fakeTaskSourceLookup{source: taskSource()},
		remote:   &fakeGitRemote{origin: "git@git.corp:" + project + ".git", events: &events},
		client:   client,
		factory:  &fakeClientFactory{fallback: client},
		events:   &events,
	}
}

func (f *fixture) resolver() *application.TaskProjectResolver {
	return application.NewTaskProjectResolver(f.settings, f.tasks, f.remote, f.factory)
}

func gitlabSettings(t *testing.T) models.CodeHostSettings {
	t.Helper()
	settings, err := models.NewCodeHostSettings(models.GitLab, "https://git.corp", token, false)
	require.NoError(t, err)
	return settings
}

func githubSettings(t *testing.T) models.CodeHostSettings {
	t.Helper()
	settings, err := models.NewCodeHostSettings(models.GitHub, "https://github.com", "ghp", false)
	require.NoError(t, err)
	return settings
}

func taskSource() *models.TaskSource {
	return &models.TaskSource{
		RepoPath:    "/repo",
		Branch:      "pi/feature",
		BaseBranch:  "main",
		Title:       "Mi tarea",
		Description: "Prompt de la tarea",
	}
}

func mergeRequest(iid uint64, state string) models.MergeRequest {
	return models.MergeRequest{
		IID:          iid,
		Title:        "Mi tarea",
		State:        state,
		SourceBranch: "pi/feature",
		TargetBranch: "main",
		WebURL:       fmt.Sprintf("https://git.corp/%s/-/merge_requests/%d", project, iid),
		MergeStatus:  "mergeable",
	}
}
