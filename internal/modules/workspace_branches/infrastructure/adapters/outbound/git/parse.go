package git

import (
	"strings"

	"iworkflow/internal/modules/workspace_branches/domain/models"
)

const (
	localRefPrefix  = "refs/heads/"
	remoteRefPrefix = "refs/remotes/"
	worktreeBranch  = "branch refs/heads/"
)

// parseBranches turns the ref list into branches. refs holds `%(refname)` lines, remotes the remote names,
// current the checked-out branch ("" when HEAD is detached) and checkedOut the local branches of any worktree.
// A remote-tracking ref whose remote is not listed is skipped. The result is sorted by models.SortBranches.
func parseBranches(refs string, remotes []string, current string, checkedOut map[string]bool) []models.Branch {
	var branches []models.Branch
	for _, line := range strings.Split(refs, "\n") {
		ref := strings.TrimSpace(line)
		if name, ok := strings.CutPrefix(ref, localRefPrefix); ok && name != "" {
			branches = append(branches, models.Branch{
				Name:       name,
				Current:    name == current,
				CheckedOut: name == current || checkedOut[name],
			})
			continue
		}
		if name, ok := strings.CutPrefix(ref, remoteRefPrefix); ok {
			remote, branch, found := models.SplitRemoteRef(name, remotes)
			if found {
				branches = append(branches, models.Branch{Name: branch, Remote: true, RemoteName: remote})
			}
		}
	}
	models.SortBranches(branches)
	return branches
}

// checkedOutBranches returns the local branches that some worktree has checked out, from `git worktree list --porcelain`.
func checkedOutBranches(porcelain string) map[string]bool {
	checked := map[string]bool{}
	for _, line := range strings.Split(porcelain, "\n") {
		if name, ok := strings.CutPrefix(strings.TrimSpace(line), worktreeBranch); ok && name != "" {
			checked[name] = true
		}
	}
	return checked
}
