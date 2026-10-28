//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/a-h/templ"
	"github.com/google/uuid"

	agentprofileserrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	agentprofilesoutbound "iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
	agentsettingsapp "iworkflow/internal/modules/agent_settings/application"
	agentsfileserrors "iworkflow/internal/modules/agents_files/domain/errors"
	agentsfilesmodels "iworkflow/internal/modules/agents_files/domain/models"
	agentsfilesoutbound "iworkflow/internal/modules/agents_files/domain/ports/outbound"
	agentsfilesdi "iworkflow/internal/modules/agents_files/infrastructure/di"
	codehostingmodels "iworkflow/internal/modules/code_hosting/domain/models"
	codehostinginbound "iworkflow/internal/modules/code_hosting/domain/ports/inbound"
	codehostingoutbound "iworkflow/internal/modules/code_hosting/domain/ports/outbound"
	codehostingcomponents "iworkflow/internal/modules/code_hosting/presentation/components"
	projecttreeapp "iworkflow/internal/modules/project_tree/application"
	projecttreemodels "iworkflow/internal/modules/project_tree/domain/models"
	projecttreeoutbound "iworkflow/internal/modules/project_tree/domain/ports/outbound"
	tasksapp "iworkflow/internal/modules/tasks/application"
	taskserrors "iworkflow/internal/modules/tasks/domain/errors"
	tasksmodels "iworkflow/internal/modules/tasks/domain/models"
	tasksoutbound "iworkflow/internal/modules/tasks/domain/ports/outbound"
	tasksinbound "iworkflow/internal/modules/tasks/infrastructure/adapters/inbound"
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

// taskWorkspaceLookup gives the tasks module the workspaces its tasks belong to. It is built over the workspaces
// use cases, which exist before the tasks module does.
type taskWorkspaceLookup struct {
	get  *workspacesapp.GetWorkspace
	list *workspacesapp.ListWorkspaces
}

var _ tasksoutbound.WorkspaceLookup = (*taskWorkspaceLookup)(nil)

// Find returns the workspace, or nil and no error when it does not exist.
func (l *taskWorkspaceLookup) Find(ctx context.Context, workspaceID string) (*tasksmodels.WorkspaceRef, error) {
	id, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, nil //nolint:nilnil // The port defines "unknown identifier" as nil and no error.
	}
	workspace, err := findWorkspace(ctx, l.get, id)
	if err != nil || workspace == nil {
		return nil, err
	}
	ref := taskWorkspaceRef(*workspace)
	return &ref, nil
}

// List returns every workspace.
func (l *taskWorkspaceLookup) List(ctx context.Context) ([]tasksmodels.WorkspaceRef, error) {
	workspaces, err := l.list.Execute(ctx)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	refs := make([]tasksmodels.WorkspaceRef, 0, len(workspaces))
	for _, workspace := range workspaces {
		refs = append(refs, taskWorkspaceRef(workspace))
	}
	return refs, nil
}

// taskWorkspaceRef maps a workspace to the view the tasks module uses.
func taskWorkspaceRef(workspace workspacesmodels.Workspace) tasksmodels.WorkspaceRef {
	return tasksmodels.WorkspaceRef{
		ID:         workspace.ID.String(),
		Name:       workspace.Name,
		Path:       workspace.Path,
		BaseBranch: workspace.BaseBranch,
	}
}

// workspaceTasksBridge gives the workspaces module the cascade that deletes the tasks of a workspace. The tasks
// module is built after the workspaces one, so the cascade is bound once it exists (see New).
type workspaceTasksBridge struct {
	deleteAll *tasksapp.DeleteWorkspaceTasks
}

var _ workspacesoutbound.WorkspaceTasks = (*workspaceTasksBridge)(nil)

// DeleteAll deletes the tasks of the workspace. A workspace with an active task is in use, as the Rust bridge maps it.
func (b *workspaceTasksBridge) DeleteAll(ctx context.Context, id workspacesmodels.WorkspaceID) error {
	err := b.deleteAll.DeleteAll(ctx, id.String())
	switch {
	case err == nil:
		return nil
	case errors.Is(err, taskserrors.WorkspaceHasActiveTasks()):
		return workspaceserrors.InUse()
	default:
		return workspaceserrors.Repository(err.Error())
	}
}

// taskTarget finds the worktree of a task, as task_target in src/app_state.rs does: a task has a target while it has
// a worktree. An unknown task yields nil and no error.
func taskTarget(ctx context.Context, get *tasksapp.GetTask, id uuid.UUID) (*terminalmodels.TaskTarget, error) {
	task, err := get.Execute(ctx, tasksmodels.TaskIDFromUUID(id))
	if errors.Is(err, taskserrors.NotFound("")) {
		return nil, nil //nolint:nilnil // The ports define "unknown identifier" as nil and no error.
	}
	if err != nil {
		return nil, fmt.Errorf("find task: %w", err)
	}
	if !task.Status.HasWorktree() || task.WorktreePath == "" {
		return nil, nil //nolint:nilnil // A task without worktree has no target.
	}
	workspaceID, err := uuid.Parse(task.WorkspaceID)
	if err != nil {
		return nil, nil //nolint:nilnil // A task with an invalid workspace has no target.
	}
	return &terminalmodels.TaskTarget{WorkspaceID: workspaceID, Name: task.Title, Path: task.WorktreePath}, nil
}

// taskRootLookup gives the project_tree module the worktree of a task. It mirrors task_root_resolver in src/app_state.rs.
type taskRootLookup struct {
	get *tasksapp.GetTask
}

var _ projecttreeoutbound.WorkspaceRootLookup = (*taskRootLookup)(nil)

// Find returns the worktree of the task as a root, or nil when the task has none.
func (l *taskRootLookup) Find(ctx context.Context, id uuid.UUID) (*projecttreemodels.WorkspaceRoot, error) {
	target, err := taskTarget(ctx, l.get, id)
	if err != nil || target == nil {
		return nil, err
	}
	return &projecttreemodels.WorkspaceRoot{Name: target.Name, Path: target.Path}, nil
}

// taskTargets gives the terminal module the working directory of a task. It mirrors the task lookup of the Rust
// terminal feature.
func taskTargets(get *tasksapp.GetTask) terminalresolver.TaskResolver {
	return func(ctx context.Context, id uuid.UUID) (*terminalmodels.TaskTarget, error) {
		return taskTarget(ctx, get, id)
	}
}

// taskSourceLookup gives the code_hosting module the repository, branch and prompt of a task. It mirrors
// CodeHostTaskSourceBridge in src/app_state.rs: a backlog task has no branch yet, so it has no source.
type taskSourceLookup struct {
	get        *tasksapp.GetTask
	workspaces *workspacesapp.GetWorkspace
}

var _ codehostingoutbound.TaskSourceLookup = (*taskSourceLookup)(nil)

// Find returns the source of the task, or nil and no error when the task does not exist or is still in backlog.
func (l *taskSourceLookup) Find(ctx context.Context, taskID string) (*codehostingmodels.TaskSource, error) {
	id, err := uuid.Parse(taskID)
	if err != nil {
		return nil, nil //nolint:nilnil // The port defines "unknown identifier" as nil and no error.
	}
	task, err := l.get.Execute(ctx, tasksmodels.TaskIDFromUUID(id))
	if errors.Is(err, taskserrors.NotFound("")) {
		return nil, nil //nolint:nilnil // The port defines "unknown identifier" as nil and no error.
	}
	if err != nil {
		return nil, fmt.Errorf("find task: %w", err)
	}
	if task.Status == tasksmodels.StatusBacklog {
		return nil, nil //nolint:nilnil // A backlog task has no branch yet.
	}
	workspaceID, err := uuid.Parse(task.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("task workspace: %w", err)
	}
	workspace, err := findWorkspace(ctx, l.workspaces, workspaceID)
	if err != nil || workspace == nil {
		return nil, err
	}
	return &codehostingmodels.TaskSource{
		RepoPath:    workspace.Path,
		Branch:      task.Branch,
		BaseBranch:  task.BaseBranch,
		Title:       task.Title,
		Description: task.Prompt,
	}, nil
}

// launchSettings gives the tasks module the launch options of an agent, resolved from the agent settings. It mirrors
// LaunchSettingsBridge in src/app_state.rs.
type launchSettings struct {
	prepareLaunch *agentsettingsapp.PrepareLaunch
}

var _ tasksoutbound.LaunchSettings = (*launchSettings)(nil)

// Prepare returns the launch options for the folder. A failed read or an unknown runtime is an error.
func (l *launchSettings) Prepare(ctx context.Context, cwd string) (tasksmodels.LaunchOptions, error) {
	preferences, err := l.prepareLaunch.Execute(ctx, cwd)
	if err != nil {
		return tasksmodels.LaunchOptions{}, fmt.Errorf("prepare launch: %w", err)
	}
	options := tasksmodels.LaunchOptions{
		Limits:         tasksmodels.ResourceLimits{MemoryMB: preferences.MemoryLimitMB, CPUPercent: preferences.CPULimitPercent},
		LimitsDisabled: preferences.LimitsDisabled,
	}
	if preferences.Runtime != nil {
		id, err := tasksmodels.ParseRuntimeID(string(*preferences.Runtime))
		if err != nil {
			return tasksmodels.LaunchOptions{}, fmt.Errorf("prepare launch runtime: %w", err)
		}
		options.Runtime = &id
	}
	if preferences.Model != nil {
		options.Model = &tasksmodels.LaunchModel{Provider: preferences.Model.Provider, Model: preferences.Model.Model}
	}
	if preferences.Thinking != nil {
		thinking := string(*preferences.Thinking)
		options.Thinking = &thinking
	}
	return options, nil
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

// remoteMergeBridge adapts the code_hosting remote merge to the port the tasks module consumes. The two modules name
// the outcome differently, so the composition root maps it, as the Rust CodeHostRemoteMergeBridge does.
type remoteMergeBridge struct {
	inner codehostinginbound.RemoteMergePort
}

var _ tasksoutbound.RemoteMerge = (*remoteMergeBridge)(nil)

// MergeOpenRequest merges the open merge request of the task and maps the outcome to the tasks names.
func (b *remoteMergeBridge) MergeOpenRequest(ctx context.Context, taskID string) (tasksoutbound.RemoteMergeOutcome, error) {
	outcome, err := b.inner.MergeOpenRequest(ctx, taskID)
	if err != nil {
		return 0, err
	}
	switch outcome {
	case codehostinginbound.RemoteMergeMerged:
		return tasksoutbound.RemoteMerged, nil
	case codehostinginbound.RemoteMergeNoOpenRequest:
		return tasksoutbound.RemoteNoOpenRequest, nil
	default:
		return 0, fmt.Errorf("unknown remote merge outcome %d", outcome)
	}
}

// taskFileSearch gives the composer of the tasks module the file search of the project tree for the @ completion.
// The project tree is built after the tasks module, so the search is bound once it exists (see New).
type taskFileSearch struct {
	search *projecttreeapp.SearchProjectFiles
}

// Files lists the files of the worktree of the task whose path matches the query. It finds nothing while unbound.
func (b *taskFileSearch) Files(ctx context.Context, id tasksmodels.TaskID, query string) ([]string, error) {
	if b.search == nil {
		return nil, nil
	}
	return b.search.Execute(ctx, projecttreemodels.TaskScope(id.UUID()), query)
}

type projectTreePanels interface {
	RenderTree(ctx context.Context, rawScope string) templ.Component
	TreePanel(ctx context.Context, rawScope string) templ.Component
	ChangesPanel(ctx context.Context, rawScope string) templ.Component
	RenderFile(ctx context.Context, rawScope string, file string) templ.Component
}

// taskPanels gives the Files and Changes tabs of a task the project views of the project tree, and the PR/MR tab
// the merge request panel of code hosting.
type taskPanels struct {
	inner projectTreePanels
}

var _ tasksinbound.ProjectPanels = (*taskPanels)(nil)

// RenderTree is the project root page of a scope.
func (b *taskPanels) RenderTree(ctx context.Context, rawScope string) templ.Component {
	return b.inner.RenderTree(ctx, rawScope)
}

// TreePanel is the tree of a scope without its page.
func (b *taskPanels) TreePanel(ctx context.Context, rawScope string) templ.Component {
	return b.inner.TreePanel(ctx, rawScope)
}

// ChangesPanel is the changes panel of a scope.
func (b *taskPanels) ChangesPanel(ctx context.Context, rawScope string) templ.Component {
	return b.inner.ChangesPanel(ctx, rawScope)
}

// RenderFile is the editor of one file of a scope.
func (b *taskPanels) RenderFile(ctx context.Context, rawScope string, file string) templ.Component {
	return b.inner.RenderFile(ctx, rawScope, file)
}

// MergeRequestPanel is the merge/pull request panel of a task.
func (b *taskPanels) MergeRequestPanel(_ context.Context, taskID string) templ.Component {
	return codehostingcomponents.TaskMergeRequestPanel(codehostingcomponents.TaskMergeRequestPanelProps{
		TaskID: taskID,
	})
}
