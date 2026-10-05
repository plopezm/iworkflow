# Feature: task-start-recovery

Medium-priority follow-ups from the tasks worktree audit (engram: tasks/worktree-lifecycle-audit).
Branch: `fix/worktree-cleanup-robustness`.

## Tasks

- [ ] 1. StartTask compensation
  - If `create()` fails and `revert_start_claim` also fails: log the revert error, try to mark the task `Failed` with the create error, and always return the original create error.
  - If `launch()` fails after the runtime started: stop the runtime best-effort (`is_running` -> `stop`, failures logged) before marking `Failed`.
  - Unit tests for both paths.
- [ ] 2. Missing worktree detection
  - `WorktreePort::exists(path)` (adapter: `tokio::fs::try_exists`), mock updated.
  - `ReconcileTasks`: interrupted task whose worktree is gone gets an explanatory error message.
  - `ResumeTask`: rejects with a clear error when the worktree is gone (discard remains possible).
  - Unit tests for reconcile and resume.

## Evidence
