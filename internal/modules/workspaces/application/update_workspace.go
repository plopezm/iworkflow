//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"fmt"

	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
)

// UpdateWorkspace edits the name and base branch of a workspace and optionally replaces its AGENTS.md.
// The workspace is saved first and the AGENTS.md is installed after, because the overwrite cannot be
// undone. If the installation fails, the original workspace is restored; if that restore fails too,
// the returned error reports both failures. The path cannot change: tasks and worktrees depend on it.
type UpdateWorkspace struct {
	repository  outbound.WorkspaceRepository
	git         outbound.GitRepository
	agentsFiles outbound.AgentsFiles
}

// NewUpdateWorkspace builds the use case.
func NewUpdateWorkspace(
	repository outbound.WorkspaceRepository,
	git outbound.GitRepository,
	agentsFiles outbound.AgentsFiles,
) *UpdateWorkspace {
	return &UpdateWorkspace{repository: repository, git: git, agentsFiles: agentsFiles}
}

// Execute applies the update and returns the updated workspace.
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

	baseBranch := current.BaseBranch
	if validated.BaseBranch != "" && validated.BaseBranch != current.BaseBranch {
		exists, err := u.git.BranchExists(ctx, current.Path, validated.BaseBranch)
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
	return updated, nil
}
