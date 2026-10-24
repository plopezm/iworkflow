//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package git

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// Merge integrates branch into baseBranch with --no-ff. Merges are serialized. When the base branch is
// checked out somewhere, the merge happens there, and only if that checkout is clean. Otherwise the merge
// runs in a temporary worktree that is removed afterwards. On conflict the base branch is left as it was.
func (a *WorktreeAdapter) Merge(ctx context.Context, repo string, baseBranch string, branch string, message string) (models.MergeOutcome, error) {
	if err := a.lockMerges(ctx); err != nil {
		return models.MergeOutcome{}, err
	}
	defer a.unlockMerges()

	target, err := a.mergeTarget(ctx, repo, baseBranch)
	if err != nil {
		return models.MergeOutcome{}, err
	}
	porcelain, err := git(ctx, repo, "worktree", "list", "--porcelain")
	if err != nil {
		return models.MergeOutcome{}, err
	}
	if checkout, ok := worktreeWithBranch(porcelain, target); ok {
		dirty, err := git(ctx, checkout, "status", "--porcelain", "--untracked-files=no")
		if err != nil {
			return models.MergeOutcome{}, err
		}
		if dirty != "" {
			return models.MergeOutcome{}, werrors.MergeBlocked(fmt.Sprintf("la rama %s tiene cambios sin commitear en %s", target, checkout))
		}
		return a.mergeIn(ctx, checkout, branch, message)
	}
	return a.mergeInTemporaryWorktree(ctx, repo, target, branch, message)
}

// mergeInTemporaryWorktree checks out target in a temporary worktree, merges there and removes it. The
// cleanup never changes the outcome of the merge.
func (a *WorktreeAdapter) mergeInTemporaryWorktree(ctx context.Context, repo string, target string, branch string, message string) (models.MergeOutcome, error) {
	unique := models.Slugify(branch) + "-" + uuid.NewString()
	temp := filepath.Join(a.root, mergeDir, unique)
	if err := os.MkdirAll(filepath.Dir(temp), 0o755); err != nil {
		return models.MergeOutcome{}, werrors.Git(fmt.Sprintf("no se pudo crear %s: %v", filepath.Dir(temp), err))
	}
	// Drops the registrations of temporary worktrees left by earlier merges.
	if _, err := git(ctx, repo, "worktree", "prune"); err != nil {
		slog.Warn("could not prune before the merge", "error", err)
	}
	if _, err := git(ctx, repo, "worktree", "add", temp, target); err != nil {
		return models.MergeOutcome{}, err
	}
	outcome, mergeErr := a.mergeIn(ctx, temp, branch, message)
	if _, err := git(ctx, repo, "worktree", "remove", "--force", temp); err != nil {
		slog.Warn("could not remove the temporary merge worktree; removing the folder", "path", temp, "error", err)
		if err := os.RemoveAll(temp); err != nil {
			slog.Warn("could not delete the temporary merge folder", "path", temp, "error", err)
		}
		if _, err := git(ctx, repo, "worktree", "prune"); err != nil {
			slog.Warn("could not prune after the merge", "error", err)
		}
	}
	return outcome, mergeErr
}

// mergeTarget returns the local branch to merge into. A base that exists only as a remote-tracking branch
// gets a local namesake that follows it, created when missing. Nothing is pushed.
func (a *WorktreeAdapter) mergeTarget(ctx context.Context, repo string, baseBranch string) (string, error) {
	local, err := refExists(ctx, repo, localRefsPrefix+baseBranch)
	if err != nil || local {
		return baseBranch, err
	}
	remote, err := refExists(ctx, repo, remoteRefsPrefix+baseBranch)
	if err != nil || !remote {
		return baseBranch, err
	}
	remotes, err := git(ctx, repo, "remote")
	if err != nil {
		return "", err
	}
	name, found := localNameOfRemote(baseBranch, remotes)
	if !found {
		return baseBranch, nil
	}
	exists, err := refExists(ctx, repo, localRefsPrefix+name)
	if err != nil {
		return "", err
	}
	if !exists {
		if _, err := git(ctx, repo, "branch", "--track", name, baseBranch); err != nil {
			return "", err
		}
	}
	return name, nil
}

// mergeIn runs the merge in target, which has the base branch checked out. On failure the outcome is a
// conflict when git reports unmerged files, and an error otherwise.
func (a *WorktreeAdapter) mergeIn(ctx context.Context, target string, branch string, message string) (models.MergeOutcome, error) {
	result, err := gitWithIdentity(ctx, target, "merge", "--no-ff", "--no-edit", "-m", message, branch)
	if err != nil {
		return models.MergeOutcome{}, err
	}
	if result.success {
		return models.MergedOutcome(), nil
	}
	// Without MERGE_HEAD there is no merge in progress (git refused before starting).
	pending, err := runRaw(ctx, target, "rev-parse", "-q", "--verify", "MERGE_HEAD")
	hasPending := err == nil && pending.success
	// The conflicted files are read before the abort, which clears them.
	conflicted, conflictErr := git(ctx, target, "diff", "--name-only", "--diff-filter=U")
	if hasPending {
		aborted, err := runRaw(ctx, target, "merge", "--abort")
		if err != nil {
			return models.MergeOutcome{}, werrors.Git(err.Error())
		}
		if !aborted.success {
			return models.MergeOutcome{}, werrors.Git(fmt.Sprintf(
				"no se pudo abortar el merge en %s; queda un merge pendiente y hay que ejecutar `git merge --abort` manualmente: %s",
				target, aborted.stderr))
		}
	}
	if conflictErr != nil {
		return models.MergeOutcome{}, conflictErr
	}
	files := splitLines(conflicted)
	if len(files) == 0 {
		return models.MergeOutcome{}, werrors.Git("merge fallido: " + result.stderr)
	}
	return models.ConflictOutcome(files), nil
}

// lockMerges takes the merge slot, or gives up when ctx ends first.
func (a *WorktreeAdapter) lockMerges(ctx context.Context) error {
	select {
	case a.merges <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("waiting for another merge: %w", ctx.Err())
	}
}

func (a *WorktreeAdapter) unlockMerges() {
	<-a.merges
}
