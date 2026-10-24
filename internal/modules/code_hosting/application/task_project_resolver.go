//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/code_hosting/domain/errors"
	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/domain/ports/outbound"
)

// ResolvedTask is a task with the connection of the detected provider and the project found in its
// origin remote.
type ResolvedTask struct {
	Settings models.CodeHostSettings
	// Client is the client of the detected provider.
	Client outbound.CodeHostClient
	Source models.TaskSource
	// Project is `namespace/project` (GitLab) or `owner/repo` (GitHub).
	Project string
}

// TaskProjectResolver joins the settings, the task and the origin remote to find the provider and the
// project of a task. It is shared by the use cases that talk to the provider.
type TaskProjectResolver struct {
	settings outbound.CodeHostSettingsRepository
	tasks    outbound.TaskSourceLookup
	remote   outbound.GitRemote
	clients  outbound.CodeHostClientFactory
}

// NewTaskProjectResolver builds the resolver.
func NewTaskProjectResolver(
	settings outbound.CodeHostSettingsRepository,
	tasks outbound.TaskSourceLookup,
	remote outbound.GitRemote,
	clients outbound.CodeHostClientFactory,
) *TaskProjectResolver {
	return &TaskProjectResolver{settings: settings, tasks: tasks, remote: remote, clients: clients}
}

// Resolve finds the provider from the host of `origin`, applies the settings of that provider and
// derives the project path.
func (r *TaskProjectResolver) Resolve(ctx context.Context, taskID string) (ResolvedTask, error) {
	source, err := r.tasks.Find(ctx, taskID)
	if err != nil {
		return ResolvedTask{}, err
	}
	if source == nil {
		return ResolvedTask{}, errors.TaskNotFound(taskID)
	}
	origin, err := r.remote.OriginURL(ctx, source.RepoPath)
	if err != nil {
		return ResolvedTask{}, err
	}
	remote, err := models.ParseRemoteURL(origin)
	if err != nil {
		return ResolvedTask{}, err
	}
	// The host of origin decides the provider, so the settings of that provider are the ones applied.
	configured, err := r.settings.List(ctx)
	if err != nil {
		return ResolvedTask{}, err
	}
	settings, err := models.SelectForHost(configured, remote.Host)
	if err != nil {
		return ResolvedTask{}, err
	}
	project, err := settings.ProjectPath(remote)
	if err != nil {
		return ResolvedTask{}, err
	}
	return ResolvedTask{
		Settings: settings,
		Client:   r.clients.ClientFor(settings.Provider()),
		Source:   *source,
		Project:  project,
	}, nil
}

// StatusOf completes a merge request with the latest pipeline and its jobs.
func (r *TaskProjectResolver) StatusOf(ctx context.Context, resolved ResolvedTask, mergeRequest models.MergeRequest) (models.MergeRequestStatus, error) {
	pipeline, err := resolved.Client.LatestPipeline(ctx, resolved.Settings, resolved.Project, mergeRequest.IID)
	if err != nil {
		return models.MergeRequestStatus{}, err
	}
	return models.MergeRequestStatus{
		Provider:     resolved.Settings.Provider(),
		ProjectPath:  resolved.Project,
		MergeRequest: mergeRequest,
		Pipeline:     pipeline,
	}, nil
}
