# Feature: edit-workspace

As a user I can edit a workspace (name, base branch, replace its AGENTS.md) from a new edit screen.
Branch: `pi/editar-workspaces-1061bad3-92ef-4ce9-99b7-55480c6bd4b2`.
Path is NOT editable (tasks/worktrees depend on it). Replacing AGENTS.md overwrites the file in the repo root with a saved one.

## Tasks

- [x] 1. Backend: `UpdateWorkspace` use case + persistence
  - Domain: `WorkspaceUpdate {name, base_branch: Option<String>, agents_file_id: Option<String>}` with `validated()`; port `WorkspaceRepositoryPort::update` (NotFound if missing); `AgentsFilesPort::install` gets `overwrite: bool` (bridge in app_state passes it through; RegisterWorkspace passes false).
  - Use case: load workspace (NotFound), validate branch exists via `GitRepositoryPort::branch_exists` when changed, install AGENTS.md with overwrite before saving, persist.
  - Diesel adapter `update` (AsChangeset) + integration test; mocks.rs updated; unit tests.
- [x] 2. Inbound: DTO `UpdateWorkspaceDto`, `#[server] update_workspace` + `get_workspace`, mapping tests; context wiring.
- [x] 3. UI: route `/workspaces/:id/edit`, `WorkspaceEditPage` + `WorkspaceEditState`, "Editar" button in `WorkspaceTable`.
- [x] 4. Checks: fmt, clippy ssr+hydrate, tests OK; headless smoke test pending.
- [x] 5. Fix review findings (compensating order in UpdateWorkspace, surface agents options error, drop duplicated normalization)

## Evidence
- Tasks 1-3: implemented by worker; 83 workspaces lib tests, 6 repo integration tests, fmt, clippy ssr + hydrate ok.
- Task 4: independent verify: fmt, clippy ssr, tests ok; headless smoke (register, update name/branch/AGENTS.md replace, bad branch -> error, blank branch keeps, GET /workspaces/:id/edit 200) ok.
- Not covered: browser UI interaction (form submit, "Editar" button render), unknown-id edit page message. Cargo.lock 0.6.2->0.7.0 drift left out of the commit.
- Task 5: review-fix (compensating order in UpdateWorkspace, surfaced agents options error, dropped duplicated normalization): RED observed (4 failed use-case tests); independent verify: fmt, clippy ssr+hydrate, full ssr suite (609 lib, 0 failed), headless smoke incl. failure compensation (bad agents id -> 500, original name kept) ok.
- Not covered: partial overwrite of AGENTS.md if install fails midway (only the DB row is restored), browser UI.
