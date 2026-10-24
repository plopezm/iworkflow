//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package git

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// Folders inside the worktrees root that the adapter owns. The sweep leaves both alone when it finds
// them as leftovers; the trash is emptied by the sweep and by the background removals.
const (
	mergeDir = ".merge"
	trashDir = ".trash"
)

// Identity used only when the repository has none configured.
const (
	fallbackUserName  = "iworkflow"
	fallbackUserEmail = "iworkflow@localhost"
)

// WorktreeAdapter implements the worktree port with the git command line under one root folder.
type WorktreeAdapter struct {
	root string
	// merges is a one-slot semaphore: every merge writes to a base branch and they race for index.lock.
	// A channel (not a sync.Mutex) lets a waiting merge give up when its context ends.
	merges chan struct{}
	// cleanup owns the background purges of the trash; WaitForCleanup waits for them.
	cleanup sync.WaitGroup
}

// NewWorktreeAdapter builds the adapter over root. The root is made absolute so that containment checks
// compare like with like; it does not have to exist yet.
func NewWorktreeAdapter(root string) (*WorktreeAdapter, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve worktrees root: %w", err)
	}
	return &WorktreeAdapter{root: absolute, merges: make(chan struct{}, 1)}, nil
}

// Root returns the managed root; the composition root and the sweep use it.
func (a *WorktreeAdapter) Root() string {
	return a.root
}

// WorktreePath returns where the worktree of a task of that workspace lives.
func (a *WorktreeAdapter) WorktreePath(workspaceName string, dirName string) string {
	return filepath.Join(a.root, workspaceName, dirName)
}

// Exists reports whether the worktree folder exists on disk.
func (a *WorktreeAdapter) Exists(_ context.Context, path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ListBranches returns the local and remote branches of the repository, without task branches.
func (a *WorktreeAdapter) ListBranches(ctx context.Context, repo string) ([]models.BranchOption, error) {
	refs, err := git(ctx, repo, "for-each-ref", "--format=%(refname)", localRefsPrefix, remoteRefsPrefix)
	if err != nil {
		return nil, err
	}
	// Empty with a detached HEAD: then no branch is the current one.
	current, err := git(ctx, repo, "branch", "--show-current")
	if err != nil {
		return nil, err
	}
	return parseBranches(refs, current), nil
}

// Create checks out a new branch from baseBranch in path. The folder's parent is created when missing.
// Both the branch and the base are validated before git sees them: a value that starts with "-" would
// be read as an option.
func (a *WorktreeAdapter) Create(ctx context.Context, repo string, baseBranch string, branch string, path string) error {
	if err := a.checkPath(path); err != nil {
		return err
	}
	if !models.IsTaskBranch(branch) {
		return werrors.Git("la rama " + branch + " no es de una tarea")
	}
	if err := checkRefArgument(baseBranch); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return werrors.Git(fmt.Sprintf("no se pudo crear %s: %v", filepath.Dir(path), err))
	}
	_, err := git(ctx, repo, "worktree", "add", "--no-track", "-b", branch, path, baseBranch)
	return err
}

// CommitAll stages every change and commits it. The boolean is false when there was nothing to commit.
func (a *WorktreeAdapter) CommitAll(ctx context.Context, path string, message string) (bool, error) {
	if _, err := git(ctx, path, "add", "-A"); err != nil {
		return false, err
	}
	staged, err := runRaw(ctx, path, "diff", "--cached", "--quiet")
	if err != nil {
		return false, werrors.Git(err.Error())
	}
	if staged.success {
		return false, nil
	}
	commit, err := gitWithIdentity(ctx, path, "commit", "-q", "-m", message)
	if err != nil {
		return false, err
	}
	if !commit.success {
		return false, werrors.Git("commit fallido: " + commit.stderr)
	}
	return true, nil
}

// Diff returns the changes of branch against baseBranch: the commits of the branch and their diff.
func (a *WorktreeAdapter) Diff(ctx context.Context, repo string, baseBranch string, branch string) (models.TaskDiff, error) {
	return buildDiff(ctx, repo, baseBranch+"..."+branch, baseBranch+".."+branch)
}

// DiffRange returns the diff of a fixed range, even when the branch no longer exists.
func (a *WorktreeAdapter) DiffRange(ctx context.Context, repo string, diffRange models.DiffRange) (models.TaskDiff, error) {
	spec := diffRange.BaseSHA + ".." + diffRange.HeadSHA
	return buildDiff(ctx, repo, spec, spec)
}

// DiffFingerprint is the tips of both branches. base...branch depends on them and on nothing else, so
// the fingerprint changes exactly when the diff can change.
func (a *WorktreeAdapter) DiffFingerprint(ctx context.Context, repo string, baseBranch string, branch string) (string, error) {
	shas, err := git(ctx, repo, "rev-parse", baseBranch, branch)
	if err != nil {
		return "", err
	}
	return strings.Join(strings.Fields(shas), ":"), nil
}

// PinRange fixes the range of branch against baseBranch: the merge base, the tip, and a ref at the tip
// that keeps the commits after the branch is deleted. It must run before the merge, which moves the
// merge base to the tip.
func (a *WorktreeAdapter) PinRange(ctx context.Context, repo string, baseBranch string, branch string, pinRef string) (models.DiffRange, error) {
	baseSHA, err := git(ctx, repo, "merge-base", baseBranch, branch)
	if err != nil {
		return models.DiffRange{}, err
	}
	headSHA, err := git(ctx, repo, "rev-parse", branch)
	if err != nil {
		return models.DiffRange{}, err
	}
	if _, err := git(ctx, repo, "update-ref", pinRef, headSHA); err != nil {
		return models.DiffRange{}, err
	}
	return models.DiffRange{BaseSHA: baseSHA, HeadSHA: headSHA}, nil
}

// Remove deletes the worktree of a task and its branch. The folder is never deleted unless it is inside
// the root (symbolic links resolved), and the root itself is never a candidate. An empty branch name
// (a task that never got one) skips the branch step; a name that is not a task branch is refused, so a
// corrupt row can never delete a user branch.
func (a *WorktreeAdapter) Remove(ctx context.Context, repo string, path string, branch string) error {
	if branch != "" && !models.IsTaskBranch(branch) {
		return werrors.Git("la rama " + branch + " no es de una tarea; no se borra")
	}
	if a.Exists(ctx, path) && !a.deferRemoval(ctx, repo, path) {
		if err := a.removeNow(ctx, repo, path); err != nil {
			return err
		}
	}
	if _, err := git(ctx, repo, "worktree", "prune"); err != nil {
		slog.Warn("could not run git worktree prune", "error", err)
	}
	if branch == "" {
		return nil
	}
	exists, err := refExists(ctx, repo, localRefsPrefix+branch)
	if err != nil {
		return err
	}
	if exists {
		if _, err := git(ctx, repo, "branch", "-D", branch); err != nil {
			return err
		}
	}
	return nil
}

// deferRemoval moves the worktree into the trash, unregisters it and purges the trash in the background,
// so a heavy target folder does not hold the request. It returns false when the worktree cannot be moved
// that way (not registered, locked, outside the root, or the rename fails); the caller then removes it
// synchronously.
func (a *WorktreeAdapter) deferRemoval(ctx context.Context, repo string, path string) bool {
	porcelain, err := git(ctx, repo, "worktree", "list", "--porcelain")
	if err != nil {
		return false
	}
	if isDeadWorktree(porcelain, path) || isLockedWorktree(porcelain, path) {
		return false
	}
	if a.ensureInsideRoot(path) != nil {
		return false
	}
	trash := filepath.Join(a.root, trashDir)
	parked := filepath.Join(trash, uuid.NewString())
	if err := os.MkdirAll(trash, 0o755); err != nil {
		return false
	}
	if err := os.Rename(path, parked); err != nil {
		slog.Warn("could not park the worktree in the trash; it is removed synchronously", "path", path, "error", err)
		return false
	}
	// The gitdir now points to a folder that does not exist: prune drops the registration.
	if _, err := git(ctx, repo, "worktree", "prune"); err != nil {
		slog.Warn("could not prune after parking the worktree", "error", err)
	}
	a.cleanup.Add(1)
	go func() {
		defer a.cleanup.Done()
		purgeTrash(trash)
	}()
	return true
}

// removeNow removes the worktree with git. When git no longer recognizes the folder as a worktree, the
// folder itself is removed, after the containment check. Any other failure is returned.
func (a *WorktreeAdapter) removeNow(ctx context.Context, repo string, path string) error {
	_, err := git(ctx, repo, "worktree", "remove", "--force", path)
	if err == nil {
		return nil
	}
	porcelain, listErr := git(ctx, repo, "worktree", "list", "--porcelain")
	if listErr != nil || !isDeadWorktree(porcelain, path) {
		return err
	}
	slog.Warn("folder is no longer a valid worktree; removing the folder", "path", path, "error", err)
	if err := a.ensureInsideRoot(path); err != nil {
		return err
	}
	if err := os.RemoveAll(path); err != nil {
		return werrors.Git(fmt.Sprintf("no se pudo borrar %s: %v", path, err))
	}
	return nil
}

// ensureInsideRoot requires path to be strictly inside the root once symbolic links are resolved: not
// empty, not the root itself, and not a link that leads elsewhere. A path that does not exist fails too.
func (a *WorktreeAdapter) ensureInsideRoot(path string) error {
	unsafe := werrors.UnsafeWorktreePath(path)
	if path == "" {
		return unsafe
	}
	root, err := filepath.EvalSymlinks(a.root)
	if err != nil {
		return unsafe
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return unsafe
	}
	if !isStrictlyInside(root, resolved) {
		return unsafe
	}
	return nil
}

// checkPath validates a path before it is used to create a worktree: absolute, and inside the root by
// its lexical form (the folder does not exist yet, so it cannot be resolved).
func (a *WorktreeAdapter) checkPath(path string) error {
	if !filepath.IsAbs(path) || !isStrictlyInside(a.root, filepath.Clean(path)) {
		return werrors.UnsafeWorktreePath(path)
	}
	return nil
}

// isStrictlyInside reports whether path is below root, not root itself and not outside it.
func isStrictlyInside(root string, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// checkRefArgument rejects a ref name that git would read as an option, or that is empty.
func checkRefArgument(ref string) error {
	if ref == "" || strings.HasPrefix(ref, "-") || strings.ContainsRune(ref, 0) {
		return werrors.Git("nombre de rama no válido: " + ref)
	}
	return nil
}

// purgeTrash deletes everything in the trash. Errors are logged: nobody waits for the result.
func purgeTrash(trash string) {
	entries, err := os.ReadDir(trash)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("could not read the trash", "trash", trash, "error", err)
		}
		return
	}
	for _, entry := range entries {
		entryPath := filepath.Join(trash, entry.Name())
		if err := os.RemoveAll(entryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("could not delete a trashed worktree", "path", entryPath, "error", err)
		}
	}
}

// WaitForCleanup waits for the background purges started so far.
func (a *WorktreeAdapter) WaitForCleanup(_ context.Context) {
	a.cleanup.Wait()
}
