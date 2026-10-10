# project-tree

Goal: clicking a workspace in the left sidebar opens `/workspaces/:id`, a page that shows the project's directory
tree (lazy: one directory level fetched per expand). New vertical slice `src/modules/project_tree/`. It resolves the
workspace root through a port bridged in `app_state.rs` (same pattern as `terminal`), and confines all reads to
that root (no `..`, no symlink escape). `.git` is hidden; directories first, then files, case-insensitive order.

- [x] 1. Backend slice: domain (models, errors, ports), `ListProjectDirectory` use case, fs adapter with root confinement, `mocks.rs`, unit + integration tests
- [x] 2. Wiring: `ProjectTreeContext` (DI), bridge in `app_state.rs`, DTOs, `#[server] list_project_directory`, mapping tests
- [x] 3. UI: state (`project_tree_state.rs`), `ProjectTreePage` + `TreeNode` component, route `/workspaces/:id`, sidebar item links to it
- [x] 4. fmt, clippy ssr/hydrate, tests; commit
