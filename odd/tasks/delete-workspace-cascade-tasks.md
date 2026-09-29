# delete-workspace-cascade-tasks

## Objective
Deleting a workspace (`DeleteWorkspace`) also deletes its tasks (and their events), but only when
none of them is active. A task is inactive when its status is `Backlog`, `Merged` or `Discarded`
(no live agent, no worktree/branch left). If any task is active, nothing is deleted and the
use case returns `WorkspaceError::InUse`.

## Scope / constraints
- `src/modules/tasks`, `src/modules/workspaces`, `src/app_state.rs`. Follow AGENTS.md.
- Features must not import each other: the workspaces slice gets its own outbound port, bridged in `app_state.rs`.
- Checks: `cargo fmt --all -- --check`, `cargo clippy --features ssr --tests -- -D warnings`,
  `cargo clippy --features hydrate --target wasm32-unknown-unknown -- -D warnings`, `cargo test --features ssr`.

## Design decisions
- Keep FK `tasks.workspace_id ON DELETE RESTRICT` as a race safety net (no migration).
- tasks: `TaskStatus::is_inactive()` (Backlog|Merged|Discarded); repository
  `delete_inactive_by_workspace(workspace_id) -> Result<Option<Vec<TaskId>>, TaskError>` runs in ONE
  transaction: if any task of the workspace is active returns `Ok(None)` without deleting; otherwise
  deletes all of them and returns their ids. Use case `DeleteWorkspaceTasks` returns
  `TaskError::WorkspaceHasActiveTasks` on `None` and calls `lifecycle.notify_removed` per id.
- workspaces: port `WorkspaceTasksPort::delete_all(WorkspaceId) -> Result<(), WorkspaceError>`;
  `DeleteWorkspace` calls it before `repository.delete`. Bridge maps
  `WorkspaceHasActiveTasks` -> `WorkspaceError::InUse`, others -> `Repository`.
- `WorkspaceError::InUse` message becomes "el workspace tiene tareas activas".

## Tasks
- [x] T1 tasks slice: status predicate, repo method + adapter integration test, use case + unit tests, mocks, context wiring
- [x] T2 workspaces slice: port, mocks, DeleteWorkspace + unit tests, bridge in app_state

## Progress / evidence

- T1: added `TaskStatus::is_inactive()`, `TaskError::WorkspaceHasActiveTasks`,
  `TaskRepositoryPort::delete_inactive_by_workspace` (single-transaction adapter
  implementation + mock), `DeleteWorkspaceTasks` use case (unit tests for success and
  active-task rejection), and wired it into `TasksContext`. Adapter integration tests
  added in `tests/modules/tasks/task_repository_adapter_test.rs`: deletes all inactive
  tasks + their events, returns `None`/deletes nothing when one task is active, other
  workspaces untouched.
- T2: added `WorkspaceTasksPort` (+ mock), updated `DeleteWorkspace` to call
  `tasks.delete_all` before `repository.delete` (unit tests for both orderings),
  changed `WorkspaceError::InUse` message to "el workspace tiene tareas activas" (and
  the matching server_fns test), updated `WorkspaceRepositoryPort::delete` doc comment
  to reference the FK as a race safety net. `app_state.rs`: built `GetWorkspace`/
  `ListWorkspaces` directly from the workspace repository (before `WorkspacesContext`)
  to construct the `WorkspaceLookupBridge` for `TasksContext`, then built
  `WorkspaceTasksBridge` from `tasks.delete_workspace_tasks` to pass into
  `WorkspacesContext::new`, avoiding a construction cycle.
- Commands run (all green): `cargo fmt --all -- --check`,
  `cargo clippy --features ssr --tests -- -D warnings`,
  `cargo clippy --features hydrate --target wasm32-unknown-unknown -- -D warnings`,
  `cargo test --features ssr` (212 tests passed, 0 failed).
- Not verified: no UI/manual exercise of the delete-workspace flow (server-only change;
  no frontend behavior touched).
