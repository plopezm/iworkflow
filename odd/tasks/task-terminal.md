# Feature: task-terminal

Button in the task detail right panel (Acciones tab) that opens a terminal (existing PTY/xterm slice) with the task's worktree as cwd.

Decision: the `terminal` slice resolves the task directory through its own outbound port (`TaskPathPort`) backed by a closure built in `app_state.rs` (same pattern as `PathResolver`); `tasks` presentation only links to a route `/tasks/:id/terminal` (new tab) owned by `terminal`, so no feature imports the other.

## Tasks
- [x] 1. Tasks domain/DTO: `TaskStatus::has_worktree()` (not Backlog/Merged/Discarded) + `can_open_terminal` in `TaskActionsDto` with tests
- [x] 2. Terminal backend: `TaskTarget`, `TaskPathPort`, `OpenTaskTerminal` use case (mockall tests), resolver adapter, `mocks.rs`, `TerminalContext`, `open_task_terminal` server fn
- [x] 3. Wiring: closure over `GetTask` in `app_state.rs`
- [x] 4. Presentation: route `/tasks/:id/terminal` + `NewTaskTerminalPage`/state in terminal slice; "Abrir terminal" link in the Acciones tab of the task detail page
- [x] 5. Checks: fmt, clippy ssr/hydrate, tests; update codegraph

## Evidence
- task 1: 36f1b7d
- task 2-3: 4768fe3
- task 4: 69d35cf
- task 5: fmt ok; clippy ssr+hydrate ok; cargo test --features ssr ok (883 unit); fixed pre-existing BlockingStart test impl (missing subagents). Not verified in browser.
