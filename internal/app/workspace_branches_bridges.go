package app

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	tasksapp "iworkflow/internal/modules/tasks/application"
	brancheserrors "iworkflow/internal/modules/workspace_branches/domain/errors"
	branchesoutbound "iworkflow/internal/modules/workspace_branches/domain/ports/outbound"
	workspacesapp "iworkflow/internal/modules/workspaces/application"
)

// branchWorkspaces gives the workspace_branches module the path and base branch of a workspace, through the
// workspaces use cases. An unknown or malformed identifier wraps ErrWorkspaceNotFound.
type branchWorkspaces struct {
	get *workspacesapp.GetWorkspace
}

var _ branchesoutbound.WorkspaceResolver = (*branchWorkspaces)(nil)

// Resolve returns the workspace the identifier names.
func (w *branchWorkspaces) Resolve(ctx context.Context, id string) (branchesoutbound.WorkspaceTarget, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return branchesoutbound.WorkspaceTarget{}, fmt.Errorf("workspace %q: %w", id, brancheserrors.ErrWorkspaceNotFound)
	}
	workspace, err := findWorkspace(ctx, w.get, parsed)
	if err != nil {
		return branchesoutbound.WorkspaceTarget{}, err
	}
	if workspace == nil {
		return branchesoutbound.WorkspaceTarget{}, fmt.Errorf("workspace %q: %w", id, brancheserrors.ErrWorkspaceNotFound)
	}
	return branchesoutbound.WorkspaceTarget{
		ID:         workspace.ID.String(),
		Name:       workspace.Name,
		Path:       workspace.Path,
		BaseBranch: workspace.BaseBranch,
	}, nil
}

// branchTaskBranches gives the workspace_branches module the branches of the live tasks of a workspace, through
// the tasks use cases. A task without a branch (still in backlog) protects nothing.
type branchTaskBranches struct {
	list *tasksapp.ListTasks
}

var _ branchesoutbound.ProtectedBranches = (*branchTaskBranches)(nil)

// InUse returns the branch names of the unfinished tasks of the workspace.
func (b *branchTaskBranches) InUse(ctx context.Context, workspaceID string) ([]string, error) {
	tasks, err := b.list.Execute(ctx, &workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list live tasks: %w", err)
	}
	names := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task.Branch != "" && !task.Status.IsClosed() {
			names = append(names, task.Branch)
		}
	}
	return names, nil
}
