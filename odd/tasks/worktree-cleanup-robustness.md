# Feature: worktree-cleanup-robustness

Fix git worktree lifecycle defects found in the tasks audit (engram: tasks/worktree-lifecycle-audit).
Branch: `fix/worktree-cleanup-robustness`.

## Tasks

- [x] 1. Merge robustness in `GitCliWorktreeAdapter`
  - `merge_in`: always abort a pending merge (MERGE_HEAD present) even if the conflicted-files diff fails; check abort success and return `TaskError::Git` naming the path if it fails; do not run abort when no MERGE_HEAD exists.
  - Temp merge worktree: unique path per merge; a failing `worktree remove` after the merge must not mask the merge outcome (warn + fallback `remove_dir_all` + `worktree prune`).
  - Integration tests (temp repo): conflict leaves base clean (no MERGE_HEAD) on both paths; stale `.merge` entry does not block merging; outcome preserved.
- [x] 2. Clean orphan worktrees/branches on workspace deletion
  - `DeleteWorkspaceTasks`: before deleting, best-effort `WorktreePort::remove` for closed tasks (Merged/Discarded) that have a branch, logging failures; active-task guard unchanged.
  - Unit tests with mocks: cleanup called for closed tasks with branch, skipped for Backlog, failure does not block deletion.
  - `MergeTask` test: `remove()` failure -> `Merged` with cleanup error recorded.

## Evidence

- Task 1: `a28ab89` — RED observed for stale merge dir test; 12/12 git adapter integration tests, 208 lib tasks tests; fmt + clippy ssr ok.
- Task 2: `73dafbb` — RED observed (3/4 delete_workspace_tasks tests); 211 lib tasks tests, repo + git integration tests, fmt, clippy ssr + hydrate ok.
- Not covered: abort-failure and temp-cleanup-failure paths (hard to provoke deterministically).
