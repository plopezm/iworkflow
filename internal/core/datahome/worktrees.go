package datahome

import (
	"context"
	"fmt"
	"os"
	"slices"

	"iworkflow/internal/core/db"
	"iworkflow/internal/core/gitx"
)

// Worktree is a task worktree and the repository it belongs to.
type Worktree struct {
	Repo string
	Path string
}

// RepairReport counts what RepairWorktrees did.
type RepairReport struct {
	// Repaired is the number of worktrees handed to git.
	Repaired int
	// MissingRepos is the number of repositories that no longer exist; their worktrees cannot be reconnected.
	MissingRepos int
}

// WorktreesOf lists the worktree of every task that has one, with the repository it was created from.
func WorktreesOf(ctx context.Context, path string) ([]Worktree, error) {
	handle, err := db.Open(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("open the database: %w", err)
	}
	defer func() { _ = handle.Close() }() // read-only use: nothing to recover from a close error
	rows, err := handle.QueryContext(ctx, `SELECT w.path, t.worktree_path FROM tasks t
		JOIN workspaces w ON w.id = t.workspace_id WHERE t.worktree_path <> '' ORDER BY t.worktree_path`)
	if err != nil {
		return nil, fmt.Errorf("list the worktrees: %w", err)
	}
	defer func() { _ = rows.Close() }() // the rows are read to the end below
	var worktrees []Worktree
	for rows.Next() {
		var item Worktree
		if err := rows.Scan(&item.Repo, &item.Path); err != nil {
			return nil, fmt.Errorf("read a worktree: %w", err)
		}
		worktrees = append(worktrees, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list the worktrees: %w", err)
	}
	return worktrees, nil
}

// RepairWorktrees tells each repository where its worktrees live now (git worktree repair). Without
// it git keeps the old location, and the next "git worktree prune" - which iworkflow runs whenever a
// task is removed - would forget the moved worktrees. Paths that do not exist (tasks already merged or
// discarded) are skipped; a missing repository is counted, not an error.
func RepairWorktrees(ctx context.Context, worktrees []Worktree) (RepairReport, error) {
	byRepo := map[string][]string{}
	var repos []string
	for _, item := range worktrees {
		if _, known := byRepo[item.Repo]; !known {
			repos = append(repos, item.Repo)
		}
		byRepo[item.Repo] = append(byRepo[item.Repo], item.Path)
	}
	slices.Sort(repos)
	var report RepairReport
	for _, repo := range repos {
		if info, err := os.Stat(repo); err != nil || !info.IsDir() {
			report.MissingRepos++
			continue
		}
		var existing []string
		for _, path := range byRepo[repo] {
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				existing = append(existing, path)
			}
		}
		if len(existing) == 0 {
			continue
		}
		if _, err := gitx.Run(ctx, repo, append([]string{"worktree", "repair"}, existing...)...); err != nil {
			return report, fmt.Errorf("repair the worktrees of %s: %w", repo, err)
		}
		report.Repaired += len(existing)
	}
	return report, nil
}
