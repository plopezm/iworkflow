//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"errors"
	"fmt"

	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
)

// defaultBranch names the branch of a repository initialized without an explicit one.
const defaultBranch = "main"

// RegisterWorkspace registers a folder with git as a workspace, normalizing its path to the root of
// the repository. When the folder is missing or is not a repository, it is created and initialized,
// but only after the caller confirms it (Initialize). When the repository has no commits, an empty
// initial commit is created so worktrees can be made.
type RegisterWorkspace struct {
	repository  outbound.WorkspaceRepository
	git         outbound.GitRepository
	agentsFiles outbound.AgentsFiles
	clock       Clock
	newID       IDGenerator
}

// NewRegisterWorkspace builds the use case.
func NewRegisterWorkspace(
	repository outbound.WorkspaceRepository,
	git outbound.GitRepository,
	agentsFiles outbound.AgentsFiles,
	clock Clock,
	newID IDGenerator,
) *RegisterWorkspace {
	return &RegisterWorkspace{
		repository:  repository,
		git:         git,
		agentsFiles: agentsFiles,
		clock:       clock,
		newID:       newID,
	}
}

// Execute registers the workspace and returns it, or fails without saving anything.
func (u *RegisterWorkspace) Execute(ctx context.Context, input models.NewWorkspace) (models.Workspace, error) {
	validated, err := input.Validated()
	if err != nil {
		return models.Workspace{}, err
	}

	info, err := u.git.Inspect(ctx, validated.Path)
	if err != nil {
		reason, recoverable := initializationReasonFor(err)
		if !recoverable {
			return models.Workspace{}, err
		}
		if !validated.Initialize {
			return models.Workspace{}, werrors.InitializationRequired(validated.Path, reason)
		}
		info, err = u.initAndInspect(ctx, validated)
		if err != nil {
			return models.Workspace{}, err
		}
	}

	existing, err := u.repository.FindByPath(ctx, info.Root)
	if err != nil {
		return models.Workspace{}, err
	}
	if existing != nil {
		return models.Workspace{}, werrors.AlreadyRegistered(info.Root)
	}

	if !info.HasCommits {
		if !validated.Initialize {
			return models.Workspace{}, werrors.InitializationRequired(info.Root, werrors.ReasonNoCommits)
		}
		if err := u.git.CreateInitialCommit(ctx, info.Root); err != nil {
			return models.Workspace{}, err
		}
	}

	baseBranch, err := u.resolveBaseBranch(ctx, validated.BaseBranch, info)
	if err != nil {
		return models.Workspace{}, err
	}

	if validated.AgentsFileID != "" {
		if err := u.agentsFiles.Install(ctx, validated.AgentsFileID, info.Root, false); err != nil {
			return models.Workspace{}, err
		}
	}

	id, err := u.newID()
	if err != nil {
		return models.Workspace{}, fmt.Errorf("generate workspace id: %w", err)
	}
	workspace := models.Workspace{
		ID:         id,
		Name:       validated.Name,
		Path:       info.Root,
		BaseBranch: baseBranch,
		CreatedAt:  u.clock().UTC(),
	}
	if err := u.repository.Save(ctx, workspace); err != nil {
		return models.Workspace{}, err
	}
	return workspace, nil
}

// resolveBaseBranch checks an explicit branch, or falls back to the current one of the repository.
func (u *RegisterWorkspace) resolveBaseBranch(ctx context.Context, requested string, info models.GitRepositoryInfo) (string, error) {
	if requested == "" {
		if info.CurrentBranch == "" {
			return "", werrors.NoCurrentBranch()
		}
		return info.CurrentBranch, nil
	}
	exists, err := u.git.BranchExists(ctx, info.Root, requested)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", werrors.BranchNotFound(requested)
	}
	return requested, nil
}

// initAndInspect initializes the folder on the requested branch (or main) and inspects it again.
func (u *RegisterWorkspace) initAndInspect(ctx context.Context, validated models.NewWorkspace) (models.GitRepositoryInfo, error) {
	initialBranch := validated.BaseBranch
	if initialBranch == "" {
		initialBranch = defaultBranch
	}
	if err := u.git.InitRepository(ctx, validated.Path, initialBranch); err != nil {
		return models.GitRepositoryInfo{}, err
	}
	return u.git.Inspect(ctx, validated.Path)
}

// initializationReasonFor maps the two "folder is not ready" errors to the reason the caller must
// confirm. Any other error is not recoverable by initializing.
func initializationReasonFor(err error) (werrors.InitializationReason, bool) {
	switch {
	case errors.Is(err, werrors.PathNotFound("")):
		return werrors.ReasonMissingFolder, true
	case errors.Is(err, werrors.NotAGitRepository("")):
		return werrors.ReasonNotAGitRepository, true
	default:
		return 0, false
	}
}
