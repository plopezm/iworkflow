package git

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// Compile-time check: the adapter implements the whole worktree port once the sweep is here.
var _ outbound.Worktree = (*WorktreeAdapter)(nil)

// SweepOrphans deletes the leftovers of the normal cleanup: task folders under the root that are not
// listed in keep and that git does not know as worktrees. Live worktrees are never deleted, whether they
// are listed in keep or not, and neither are folders the adapter owns (hidden ones such as .trash).
//
// Keep is the set of folders of the tasks that are still open. A folder is protected when any form of its
// path (as given, cleaned, or with symbolic links resolved) matches an entry of keep: the comparison leans
// towards keeping. Symbolic links are never followed or deleted.
//
// The returned count is the number of folders deleted. A cancelled context stops the sweep at the next
// folder, so nothing is deleted after the cancellation.
func (a *WorktreeAdapter) SweepOrphans(ctx context.Context, keep []string) (int, error) {
	trash := filepath.Join(a.root, trashDir)
	if _, err := os.Stat(trash); err == nil {
		purgeTrash(trash)
	}
	kept := newFolderSet(keep)
	removed := 0
	for _, workspaceDir := range visibleSubdirs(a.root) {
		for _, candidate := range visibleSubdirs(workspaceDir) {
			if err := ctx.Err(); err != nil {
				return removed, fmt.Errorf("sweep cancelled: %w", err)
			}
			if kept.contains(candidate) || isLiveWorktreeDir(candidate) {
				continue
			}
			if a.ensureInsideRoot(candidate) != nil {
				continue
			}
			if err := os.RemoveAll(candidate); err != nil {
				slog.Warn("could not delete an orphaned folder", "path", candidate, "error", err)
				continue
			}
			slog.Info("orphaned folder deleted", "path", candidate)
			removed++
		}
	}
	return removed, nil
}

// folderSet answers whether a folder is among the protected ones. It keeps every form of each path.
type folderSet struct {
	forms map[string]struct{}
}

func newFolderSet(paths []string) folderSet {
	set := folderSet{forms: make(map[string]struct{}, 2*len(paths))}
	for _, path := range paths {
		if path == "" {
			continue
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			absolute = path
		}
		set.forms[filepath.Clean(absolute)] = struct{}{}
		set.forms[realPath(absolute)] = struct{}{}
	}
	return set
}

func (s folderSet) contains(path string) bool {
	if _, ok := s.forms[filepath.Clean(path)]; ok {
		return true
	}
	_, ok := s.forms[realPath(path)]
	return ok
}

// visibleSubdirs lists the real subfolders of dir: hidden ones and symbolic links are skipped. An
// unreadable or missing dir has none.
func visibleSubdirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var folders []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") || !entry.IsDir() {
			continue
		}
		folders = append(folders, filepath.Join(dir, entry.Name()))
	}
	return folders
}

// isLiveWorktreeDir reports whether the folder has a .git of its own. Without one it is only leftovers.
// Any error other than "does not exist" counts as live: when in doubt, the folder is kept.
func isLiveWorktreeDir(path string) bool {
	_, err := os.Lstat(filepath.Join(path, ".git"))
	return !errors.Is(err, os.ErrNotExist)
}
