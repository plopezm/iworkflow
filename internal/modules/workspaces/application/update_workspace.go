//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
)

// UpdateWorkspace edits a workspace: its name, base branch, AGENTS.md, default agent, color and, when it is safe,
// its path (see checkPathChange). The workspace is saved first and the AGENTS.md is installed after, because the
// overwrite cannot be undone. If the installation fails, the original workspace is restored; if that restore fails
// too, the returned error reports both failures. The worktrees are repaired last, once the new path is in place.
type UpdateWorkspace struct {
	repository  outbound.WorkspaceRepository
	git         outbound.GitRepository
	agentsFiles outbound.AgentsFiles
	runtimes    outbound.AgentRuntimes
	tasks       outbound.WorkspaceTasks
}

// NewUpdateWorkspace builds the use case.
func NewUpdateWorkspace(
	repository outbound.WorkspaceRepository,
	git outbound.GitRepository,
	agentsFiles outbound.AgentsFiles,
	runtimes outbound.AgentRuntimes,
	tasks outbound.WorkspaceTasks,
) *UpdateWorkspace {
	return &UpdateWorkspace{repository: repository, git: git, agentsFiles: agentsFiles, runtimes: runtimes, tasks: tasks}
}

// PathEditBlocker reports whether the path of the workspace is locked because it has unfinished tasks. It only
// reads, so the edit screen can disable the path field. The reason is werrors.PathBlockedByTasks().
func (u *UpdateWorkspace) PathEditBlocker(ctx context.Context, id models.WorkspaceID) (bool, error) {
	current, err := u.repository.FindByID(ctx, id)
	if err != nil {
		return false, err
	}
	if current == nil {
		return false, werrors.NotFound(id.String())
	}
	return u.tasks.HasBlocking(ctx, id)
}

// Execute applies the update and returns the updated workspace. A changed path is checked before anything is
// saved; when the workspace still has worktrees after the save, they are repaired, and a repair failure is
// returned together with the updated workspace, which stays saved.
func (u *UpdateWorkspace) Execute(ctx context.Context, id models.WorkspaceID, input models.WorkspaceUpdate) (models.Workspace, error) {
	validated, err := input.Validated()
	if err != nil {
		return models.Workspace{}, err
	}
	current, err := u.repository.FindByID(ctx, id)
	if err != nil {
		return models.Workspace{}, err
	}
	if current == nil {
		return models.Workspace{}, werrors.NotFound(id.String())
	}

	pathChanged := validated.Path != "" && validated.Path != current.Path
	root := current.Path
	if pathChanged {
		if err := u.checkPathChange(ctx, *current, validated.Path, validated.ConfirmSameProject); err != nil {
			return models.Workspace{}, err
		}
		root = validated.Path
	}

	// An absent default agent keeps the saved one without checking it against the installed agents, so
	// editing a workspace never drops it; a present one is validated like a registration.
	defaultRuntime := current.DefaultRuntime
	if validated.DefaultRuntime != nil {
		resolved, resolveErr := resolveDefaultRuntime(ctx, u.runtimes, *validated.DefaultRuntime)
		if resolveErr != nil {
			return models.Workspace{}, resolveErr
		}
		defaultRuntime = resolved
	}

	baseBranch := current.BaseBranch
	if validated.BaseBranch != "" && validated.BaseBranch != current.BaseBranch {
		exists, err := u.git.BranchExists(ctx, root, validated.BaseBranch)
		if err != nil {
			return models.Workspace{}, err
		}
		if !exists {
			return models.Workspace{}, werrors.BranchNotFound(validated.BaseBranch)
		}
		baseBranch = validated.BaseBranch
	}

	original := *current
	updated := *current
	updated.Name = validated.Name
	updated.BaseBranch = baseBranch
	updated.DefaultRuntime = defaultRuntime
	if pathChanged {
		updated.Path = validated.Path
	}
	if validated.Color != nil {
		updated.Color = optionalColor(*validated.Color)
	}
	if err := u.repository.Update(ctx, updated); err != nil {
		return models.Workspace{}, err
	}

	if validated.AgentsFileID != "" {
		if installErr := u.agentsFiles.Install(ctx, validated.AgentsFileID, updated.Path, true); installErr != nil {
			if revertErr := u.repository.Update(ctx, original); revertErr != nil {
				return models.Workspace{}, werrors.Repository(fmt.Sprintf(
					"%s; además falló la restauración del workspace: %s", installErr.Error(), revertErr.Error()))
			}
			return models.Workspace{}, installErr
		}
	}
	if pathChanged {
		if repairErr := u.repairWorktrees(ctx, updated.Path); repairErr != nil {
			return updated, repairErr
		}
	}
	return updated, nil
}

// checkPathChange applies the rules of a new path, in order, and stops at the first one that fails. Nothing is
// saved by this method, so a failure leaves the stored workspace unchanged.
//
//  1. The path is absolute (checked by WorkspaceUpdate.Validated before this method runs).
//  2. The workspace has no unfinished task: a task that is not merged or discarded, backlog included.
//  3. The new folder exists and is the root of a git repository, not a subfolder of one.
//  4. The new repository shares a root commit with the old one, so the path does not point at another project. When
//     the old folder is gone this cannot be checked, and the person must confirm the same project (S4b).
//  5. No other workspace uses the new path.
func (u *UpdateWorkspace) checkPathChange(ctx context.Context, current models.Workspace, newPath string, confirmSameProject bool) error {
	blocked, err := u.tasks.HasBlocking(ctx, current.ID)
	if err != nil {
		return err
	}
	if blocked {
		return werrors.PathBlockedByTasks()
	}

	info, err := u.git.Inspect(ctx, newPath)
	if err != nil {
		return err
	}
	if info.Root != newPath {
		return werrors.PathNotRepositoryRoot(newPath)
	}

	if err := u.checkSameProject(ctx, current.Path, newPath, confirmSameProject); err != nil {
		return err
	}

	existing, err := u.repository.FindByPath(ctx, newPath)
	if err != nil {
		return err
	}
	if existing != nil && existing.ID != current.ID {
		return werrors.AlreadyRegistered(newPath)
	}
	return nil
}

// checkSameProject requires the old and the new repository to share a root commit (S4). When the old folder no
// longer exists, which is the usual reason to edit the path (the folder was moved), its root commits cannot be
// read: the workspace does not record one, so nothing can be compared. Then the new path is accepted only when the
// person confirmed that it is the same project (S4b); otherwise it is rejected and nothing is saved. The flag is
// ignored when the old folder exists.
func (u *UpdateWorkspace) checkSameProject(ctx context.Context, oldPath string, newPath string, confirmed bool) error {
	oldRoots, err := u.git.RootCommits(ctx, oldPath)
	if errors.Is(err, werrors.PathNotFound("")) {
		if !confirmed {
			return werrors.SameProjectUnconfirmed()
		}
		return nil
	}
	if err != nil {
		return err
	}
	newRoots, err := u.git.RootCommits(ctx, newPath)
	if err != nil {
		return err
	}
	for _, commit := range oldRoots {
		if slices.Contains(newRoots, commit) {
			return nil
		}
	}
	return werrors.PathNotSameProject(newPath)
}

// repairWorktrees runs `git worktree repair` when the repository still has linked worktrees (S5). It is a
// safety net: a workspace with unfinished tasks cannot change its path, so usually there is nothing to repair.
func (u *UpdateWorkspace) repairWorktrees(ctx context.Context, root string) error {
	linked, err := u.git.HasLinkedWorktrees(ctx, root)
	if err != nil {
		return werrors.WorktreeRepair(err.Error())
	}
	if !linked {
		return nil
	}
	if err := u.git.RepairWorktrees(ctx, root); err != nil {
		return werrors.WorktreeRepair(err.Error())
	}
	return nil
}
