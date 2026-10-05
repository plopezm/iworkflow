# Feature: task-start-recovery

Medium-priority follow-ups from the tasks worktree audit (engram: tasks/worktree-lifecycle-audit).
Branch: `fix/worktree-cleanup-robustness`.

## Tasks

- [x] 1. StartTask compensation
  - If `create()` fails and `revert_start_claim` also fails: log the revert error, try to mark the task `Failed` with the create error, and always return the original create error.
  - If `launch()` fails after the runtime started: stop the runtime best-effort (`is_running` -> `stop`, failures logged) before marking `Failed`.
  - Unit tests for both paths.
- [x] 2. Missing worktree detection
  - `WorktreePort::exists(path)` (adapter: `tokio::fs::try_exists`), mock updated.
  - `ReconcileTasks`: interrupted task whose worktree is gone gets an explanatory error message.
  - `ResumeTask`: rejects with a clear error when the worktree is gone (discard remains possible).
  - Unit tests for reconcile and resume.

## Evidence

- Task 1: `a64dd94` — RED observed (create error masked; stop called 0 times); 213 tasks lib tests ok.
- Task 2: `425b379` — RED observed (reconcile message, resume proceeded); 216 tasks lib + 13 git adapter tests ok.
- Full verification at 425b379: fmt, clippy ssr + hydrate, `cargo test --features ssr` 516 passed. Not run: `cargo leptos test`, e2e.
