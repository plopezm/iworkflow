package git

import (
	"context"
	"strings"
	"unicode/utf8"

	"iworkflow/internal/modules/tasks/domain/models"
)

// maxPatchBytes is the largest patch returned to the client. Longer patches are cut and marked.
const maxPatchBytes = 1024 * 1024

// buildDiff runs `git diff --stat spec`, `git diff spec` and `git log --oneline logRange`.
func buildDiff(ctx context.Context, repo string, spec string, logRange string) (models.TaskDiff, error) {
	stat, err := git(ctx, repo, "diff", "--stat", spec)
	if err != nil {
		return models.TaskDiff{}, err
	}
	patch, err := git(ctx, repo, "diff", spec)
	if err != nil {
		return models.TaskDiff{}, err
	}
	log, err := git(ctx, repo, "log", "--oneline", logRange)
	if err != nil {
		return models.TaskDiff{}, err
	}
	return truncateDiff(stat, patch, splitLines(log)), nil
}

// truncateDiff cuts the patch at maxPatchBytes, on a character boundary, and records whether it cut.
func truncateDiff(stat string, patch string, commits []string) models.TaskDiff {
	truncated := len(patch) > maxPatchBytes
	if truncated {
		cut := maxPatchBytes
		for cut > 0 && !utf8.RuneStart(patch[cut]) {
			cut--
		}
		patch = patch[:cut]
	}
	return models.TaskDiff{Stat: stat, Patch: patch, Truncated: truncated, Commits: commits}
}

// DiffLive is the diff of a worktree against the merge base with baseBranch: the commits of the branch,
// the uncommitted changes and the new untracked files. It reads the worktree and does not touch the
// index the agent uses.
func (a *WorktreeAdapter) DiffLive(ctx context.Context, worktree string, baseBranch string) (models.TaskDiff, error) {
	mergeBase, err := git(ctx, worktree, "merge-base", baseBranch, "HEAD")
	if err != nil {
		return models.TaskDiff{}, err
	}
	stat, err := git(ctx, worktree, "diff", "--stat", mergeBase)
	if err != nil {
		return models.TaskDiff{}, err
	}
	patch, err := git(ctx, worktree, "diff", mergeBase)
	if err != nil {
		return models.TaskDiff{}, err
	}
	untracked, err := git(ctx, worktree, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return models.TaskDiff{}, err
	}
	for _, file := range strings.Split(untracked, "\x00") {
		if file == "" {
			continue
		}
		// git() trims its output: the line break that separates the sections is restored here.
		if stat != "" {
			stat += "\n"
		}
		if patch != "" {
			patch += "\n"
		}
		filePatch, err := untrackedPatch(ctx, worktree, file, nil)
		if err != nil {
			return models.TaskDiff{}, err
		}
		fileStat, err := untrackedPatch(ctx, worktree, file, []string{"--stat"})
		if err != nil {
			return models.TaskDiff{}, err
		}
		patch += filePatch
		stat += fileStat
	}
	log, err := git(ctx, worktree, "log", "--oneline", mergeBase+"..HEAD")
	if err != nil {
		return models.TaskDiff{}, err
	}
	return truncateDiff(stat, patch, splitLines(log)), nil
}

// untrackedPatch is the diff of a file that is not tracked, as if it were added. `git diff --no-index`
// exits with 1 when the files differ, so its exit status is not an error: only its output is used.
func untrackedPatch(ctx context.Context, worktree string, file string, extra []string) (string, error) {
	args := append([]string{"diff", "--no-index"}, extra...)
	args = append(args, "--", "/dev/null", file)
	out, err := runRaw(ctx, worktree, args...)
	if err != nil {
		return "", err
	}
	return out.stdout, nil
}
