package git

import (
	"path/filepath"
	"sort"
	"strings"

	"iworkflow/internal/modules/tasks/domain/models"
)

const (
	localRefsPrefix  = "refs/heads/"
	remoteRefsPrefix = "refs/remotes/"
)

// worktreeEntry is one block of `git worktree list --porcelain`.
type worktreeEntry struct {
	path     string
	prunable bool
	locked   bool
	branch   string
}

// parseWorktrees reads the porcelain listing: blocks separated by blank lines, each starting with
// "worktree <path>". Paths are returned as git printed them.
func parseWorktrees(porcelain string) []worktreeEntry {
	var entries []worktreeEntry
	for _, line := range strings.Split(porcelain, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			entries = append(entries, worktreeEntry{path: strings.TrimPrefix(line, "worktree ")})
		case len(entries) == 0:
			continue
		case strings.HasPrefix(line, "prunable"):
			entries[len(entries)-1].prunable = true
		case strings.HasPrefix(line, "locked"):
			entries[len(entries)-1].locked = true
		case strings.HasPrefix(line, "branch refs/heads/"):
			entries[len(entries)-1].branch = strings.TrimPrefix(line, "branch refs/heads/")
		}
	}
	return entries
}

// findWorktree returns the listed entry for path. Both the path as given and its real location are
// compared, because git may print either form.
func findWorktree(entries []worktreeEntry, path string) (worktreeEntry, bool) {
	real := realPath(path)
	for _, entry := range entries {
		if sameFolder(entry.path, path) || sameFolder(entry.path, real) {
			return entry, true
		}
	}
	return worktreeEntry{}, false
}

// isDeadWorktree reports whether git cannot remove path as a worktree: it is not listed, or it is
// listed as prunable (its gitdir no longer exists).
func isDeadWorktree(porcelain string, path string) bool {
	entry, listed := findWorktree(parseWorktrees(porcelain), path)
	return !listed || entry.prunable
}

// isLockedWorktree reports whether path is listed as locked.
func isLockedWorktree(porcelain string, path string) bool {
	entry, listed := findWorktree(parseWorktrees(porcelain), path)
	return listed && entry.locked
}

// worktreeWithBranch returns the checkout of branch, when a worktree has it checked out.
func worktreeWithBranch(porcelain string, branch string) (string, bool) {
	for _, entry := range parseWorktrees(porcelain) {
		if entry.branch == branch {
			return entry.path, true
		}
	}
	return "", false
}

// parseBranches turns `git for-each-ref --format=%(refname)` output into the branch options: local
// branches first, then remote ones, each group sorted by name. Task branches and "<remote>/HEAD" are
// left out. current is the checked-out branch, empty when HEAD is detached.
func parseBranches(refs string, current string) []models.BranchOption {
	var local, remote []models.BranchOption
	for _, reference := range strings.Split(refs, "\n") {
		reference = strings.TrimSpace(reference)
		if name, ok := strings.CutPrefix(reference, localRefsPrefix); ok {
			if !models.IsTaskBranch(name) {
				local = append(local, models.BranchOption{Name: name, Remote: false, Current: name == current})
			}
			continue
		}
		if name, ok := strings.CutPrefix(reference, remoteRefsPrefix); ok && !strings.HasSuffix(name, "/HEAD") {
			remote = append(remote, models.BranchOption{Name: name, Remote: true})
		}
	}
	byName := func(options []models.BranchOption) {
		sort.Slice(options, func(i, j int) bool { return options[i].Name < options[j].Name })
	}
	byName(local)
	byName(remote)
	return append(local, remote...)
}

// localNameOfRemote returns the local name of a remote-tracking branch: "origin/develop" with remote
// "origin" gives "develop". The boolean is false when no listed remote matches.
func localNameOfRemote(base string, remotes string) (string, bool) {
	for _, remote := range strings.Split(remotes, "\n") {
		remote = strings.TrimSpace(remote)
		if remote == "" {
			continue
		}
		if rest, ok := strings.CutPrefix(base, remote+"/"); ok && rest != "" {
			return rest, true
		}
	}
	return "", false
}

// realPath resolves symbolic links when the path exists, and returns the cleaned path otherwise.
func realPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(path)
}

// sameFolder reports whether two paths name the same location, comparing cleaned and resolved forms.
func sameFolder(a string, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return filepath.Clean(a) == filepath.Clean(b) || realPath(a) == realPath(b)
}
