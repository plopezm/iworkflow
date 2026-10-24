//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	agentprofileserrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	agentprofilesoutbound "iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
	agentsettingsapp "iworkflow/internal/modules/agent_settings/application"
	agentsfileserrors "iworkflow/internal/modules/agents_files/domain/errors"
	agentsfilesmodels "iworkflow/internal/modules/agents_files/domain/models"
	agentsfilesoutbound "iworkflow/internal/modules/agents_files/domain/ports/outbound"
	agentsfilesdi "iworkflow/internal/modules/agents_files/infrastructure/di"
	codehostingmodels "iworkflow/internal/modules/code_hosting/domain/models"
	codehostingoutbound "iworkflow/internal/modules/code_hosting/domain/ports/outbound"
	projecttreemodels "iworkflow/internal/modules/project_tree/domain/models"
	projecttreeoutbound "iworkflow/internal/modules/project_tree/domain/ports/outbound"
	terminalmodels "iworkflow/internal/modules/terminal/domain/models"
	terminalresolver "iworkflow/internal/modules/terminal/infrastructure/adapters/outbound/resolver"
	workspacesapp "iworkflow/internal/modules/workspaces/application"
	workspaceserrors "iworkflow/internal/modules/workspaces/domain/errors"
	workspacesmodels "iworkflow/internal/modules/workspaces/domain/models"
	workspacesoutbound "iworkflow/internal/modules/workspaces/domain/ports/outbound"
)

// projectLookup presents the workspaces as the projects an AGENTS.md can be installed in. The
// workspaces module needs the agents_files module and the other way round, so the lookup is built
// before both and bound to the workspaces use cases once they exist (see New).
type projectLookup struct {
	list *workspacesapp.ListWorkspaces
	get  *workspacesapp.GetWorkspace
}

var _ agentsfilesoutbound.ProjectLookup = (*projectLookup)(nil)

// List returns every workspace as a project.
func (p *projectLookup) List(ctx context.Context) ([]agentsfilesmodels.ProjectRef, error) {
	workspaces, err := p.list.Execute(ctx)
	if err != nil {
		return nil, fmt.Errorf("list workspaces as projects: %w", err)
	}
	projects := make([]agentsfilesmodels.ProjectRef, 0, len(workspaces))
	for _, workspace := range workspaces {
		projects = append(projects, projectOf(workspace))
	}
	return projects, nil
}

// Find returns the workspace with the given identifier as a project. An unknown identifier yields
// nil and no error, as the port requires.
func (p *projectLookup) Find(ctx context.Context, id string) (*agentsfilesmodels.ProjectRef, error) {
	workspaceID, err := workspacesmodels.ParseWorkspaceID(id)
	if err != nil {
		return nil, nil //nolint:nilnil // The port defines "unknown identifier" as nil and no error.
	}
	workspace, err := p.get.Execute(ctx, workspaceID)
	if errors.Is(err, workspaceserrors.NotFound("")) {
		return nil, nil //nolint:nilnil // The port defines "unknown identifier" as nil and no error.
	}
	if err != nil {
		return nil, fmt.Errorf("find workspace as project: %w", err)
	}
	project := projectOf(workspace)
	return &project, nil
}

// projectOf maps a workspace to the project view the agents_files module uses.
func projectOf(workspace workspacesmodels.Workspace) agentsfilesmodels.ProjectRef {
	return agentsfilesmodels.ProjectRef{
		ID:   workspace.ID.String(),
		Name: workspace.Name,
		Path: workspace.Path,
	}
}

// agentsFilesBridge gives the workspaces module the stored AGENTS.md files, as the outbound port
// defines them, through the agents_files module.
type agentsFilesBridge struct {
	module *agentsfilesdi.Module
}

var _ workspacesoutbound.AgentsFiles = (*agentsFilesBridge)(nil)

// ListOptions returns the stored AGENTS.md files a workspace can install.
func (b *agentsFilesBridge) ListOptions(ctx context.Context) ([]workspacesmodels.AgentsFileOption, error) {
	files, err := b.module.List.Execute(ctx)
	if err != nil {
		return nil, fmt.Errorf("list agents files: %w", err)
	}
	options := make([]workspacesmodels.AgentsFileOption, 0, len(files))
	for _, file := range files {
		options = append(options, workspacesmodels.AgentsFileOption{
			ID:          file.ID.String(),
			Description: file.Description,
		})
	}
	return options, nil
}

// Install writes the stored AGENTS.md into root. Errors are mapped to the workspaces error kinds.
func (b *agentsFilesBridge) Install(ctx context.Context, agentsFileID string, root string, overwrite bool) error {
	id, err := agentsfilesmodels.ParseAgentsFileID(agentsFileID)
	if err != nil {
		return workspaceserrors.AgentsFileNotFound(agentsFileID)
	}
	if _, err := b.module.Install.Execute(ctx, id, root, overwrite); err != nil {
		return mapAgentsFileError(err, agentsFileID, root)
	}
	return nil
}

// mapAgentsFileError translates an agents_files failure into the workspaces error the caller expects.
func mapAgentsFileError(err error, agentsFileID string, root string) error {
	switch {
	case errors.Is(err, agentsfileserrors.NotFound("")):
		return workspaceserrors.AgentsFileNotFound(agentsFileID)
	case errors.Is(err, agentsfileserrors.TargetExists("")):
		return workspaceserrors.AgentsFileTargetExists(root)
	default:
		return workspaceserrors.AgentsFile(err.Error())
	}
}

// findWorkspace returns the workspace with the given identifier, or nil and no error when it does not
// exist. The Rust resolvers (workspace_root_resolver and the terminal workspace lookup) treat a
// missing workspace the same way.
func findWorkspace(ctx context.Context, get *workspacesapp.GetWorkspace, id uuid.UUID) (*workspacesmodels.Workspace, error) {
	workspace, err := get.Execute(ctx, workspacesmodels.WorkspaceIDFromUUID(id))
	if errors.Is(err, workspaceserrors.NotFound("")) {
		return nil, nil //nolint:nilnil // The ports define "unknown identifier" as nil and no error.
	}
	if err != nil {
		return nil, fmt.Errorf("find workspace: %w", err)
	}
	return &workspace, nil
}

// workspaceRoots gives the project_tree module the root directory of a workspace. It mirrors
// workspace_root_resolver in src/app_state.rs.
type workspaceRoots struct {
	get *workspacesapp.GetWorkspace
}

var _ projecttreeoutbound.WorkspaceRootLookup = (*workspaceRoots)(nil)

// Find returns the root of the workspace, or nil when it does not exist.
func (w *workspaceRoots) Find(ctx context.Context, id uuid.UUID) (*projecttreemodels.WorkspaceRoot, error) {
	workspace, err := findWorkspace(ctx, w.get, id)
	if err != nil || workspace == nil {
		return nil, err
	}
	return &projecttreemodels.WorkspaceRoot{Name: workspace.Name, Path: workspace.Path}, nil
}

// workspaceTargets gives the terminal module the working directory of a workspace. It mirrors the
// workspace lookup of the Rust terminal feature.
func workspaceTargets(get *workspacesapp.GetWorkspace) terminalresolver.WorkspaceResolver {
	return func(ctx context.Context, id uuid.UUID) (*terminalmodels.WorkspaceTarget, error) {
		workspace, err := findWorkspace(ctx, get, id)
		if err != nil || workspace == nil {
			return nil, err
		}
		return &terminalmodels.WorkspaceTarget{Name: workspace.Name, Path: workspace.Path}, nil
	}
}

// TODO(tasks): the tasks module is not migrated yet, so no task resolves. The three placeholders below
// stand for the task lookups of the Rust version (task_root_resolver, task_target and
// CodeHostTaskSourceBridge in src/app_state.rs). Replace them when the tasks module lands.

// taskRootsPending is the project_tree lookup of task worktrees. Until tasks are migrated it finds nothing.
type taskRootsPending struct{}

var _ projecttreeoutbound.WorkspaceRootLookup = taskRootsPending{}

// Find returns nil: no task is migrated yet.
func (taskRootsPending) Find(context.Context, uuid.UUID) (*projecttreemodels.WorkspaceRoot, error) {
	return nil, nil //nolint:nilnil // TODO(tasks): no task is migrated yet.
}

// taskTargetsPending is the terminal lookup of task worktrees. Until tasks are migrated it finds nothing.
func taskTargetsPending(context.Context, uuid.UUID) (*terminalmodels.TaskTarget, error) {
	return nil, nil //nolint:nilnil // TODO(tasks): no task is migrated yet.
}

// taskSourcesPending is the code_hosting lookup of the repository, branch and prompt of a task. Until
// tasks are migrated it finds nothing, so the GitLab and GitHub merge requests of tasks stay unavailable.
type taskSourcesPending struct{}

var _ codehostingoutbound.TaskSourceLookup = taskSourcesPending{}

// Find returns nil: no task is migrated yet.
func (taskSourcesPending) Find(context.Context, string) (*codehostingmodels.TaskSource, error) {
	return nil, nil //nolint:nilnil // TODO(tasks): no task is migrated yet.
}

// modelOptions lists the orchestrator models of the agent settings catalog for the profile routing
// editors. It mirrors ModelOptionsBridge in src/app_state.rs: the identifier and the label are
// provider/model.
type modelOptions struct {
	listModels *agentsettingsapp.ListModels
}

var _ agentprofilesoutbound.ModelOptionsPort = (*modelOptions)(nil)

// List returns the models of the catalog. A failed read is an agent_profiles IO error.
func (m *modelOptions) List(ctx context.Context) ([]agentprofilesoutbound.ModelOption, error) {
	refs, err := m.listModels.Execute(ctx)
	if err != nil {
		return nil, agentprofileserrors.IO(err.Error())
	}
	options := make([]agentprofilesoutbound.ModelOption, 0, len(refs))
	for _, ref := range refs {
		id := ref.Provider + "/" + ref.Model
		options = append(options, agentprofilesoutbound.ModelOption{ID: id, Label: id})
	}
	return options, nil
}
