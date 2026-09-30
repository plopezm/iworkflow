# three-column-shell-layout

## Objective
Reorganize the UI into a 3-column app shell similar to the reference screenshot
`/tmp/pi-clipboard-777312ee-0542-40a7-881b-99f59c8b0c06.png` (read it first), **keeping all existing
functionality and routes** (`/`, `/workspaces`, `/tasks/:id`, kanban, modals, agent settings bar).

## Reference traits
- Full-viewport app (no page scroll; each column scrolls independently), flat bordered panels, compact
  mono-ish typography, cream/purple look (adapt to the existing Tailwind theme tokens; do not
  introduce a second theme system — a light restyle of tokens is fine if it stays coherent).
- Left sidebar: brand + "Actions" menu button, collapsible sections (WORKSPACES with count, TASKS with
  count) as rows with a title, subtitle and a `…` menu button; selected row highlighted.
- Center: main content (the routed page: kanban board, workspaces page, or task transcript + composer
  pinned at the bottom with status pill).
- Right: tab strip (e.g. Diff, Acciones, Estado/Info) + tab body; only on the task detail route.
- Collapse handles on the sidebar edges (left sidebar and right panel collapsible; state = signals).

## Scope / constraints
- `src/app.rs`, `src/core/components/**` (new generic layout components: `AppShell`, `SidebarSection`,
  `SidebarItem`, `TabBar`, `CollapsePanel`), `src/modules/*/presentation/**`, `style/tailwind.css`.
- The left sidebar needs workspaces + tasks data: `core` cannot import `modules/`. Put the sidebar
  composition in the feature that owns the data (e.g. `tasks/presentation/components/app_sidebar.rs`
  using existing server fns/resources) and pass it into the generic core shell via `Children`/props
  from `app.rs` (composition root). No new backend/DB work unless strictly required.
- Keep hydration rules: Resources read only inside `Transition`/`Suspense`; no signal writes from
  `Suspend` to outside (use `Effect`). Mobile (<768px): sidebars become overlays/stack, no horizontal
  overflow at 390px. Keep `--nav-h` consistent or remove it if the top nav disappears (the
  `AgentSettingsBar` must remain reachable — e.g. in the center header or sidebar footer).
- No business logic in components. Explicit `let` types, no `unwrap/expect`. English artifacts;
  existing Spanish UI copy stays.
- Checks: `cargo fmt --all -- --check`, `cargo clippy --features ssr --tests -- -D warnings`,
  `cargo clippy --features hydrate --target wasm32-unknown-unknown -- -D warnings`,
  `cargo test --features ssr`, `cargo leptos build`. Existing tests must keep passing; add state tests
  for any new non-trivial UI state (collapse/selection) where they add value.

## Tasks
- [x] T1 Generic shell + layout components in `core/components`, wire into `app.rs`, sidebar with
      workspaces and tasks, route highlighting, collapse state, responsive behavior.
- [x] T2 Task detail page: move to center (transcript + composer pinned) + right tab panel
      (Diff / Acciones / Estado) using the existing components (`DiffView`, `TaskActions`,
      `AgentSidebar`, `DialogPanel`).
- [x] T3 Board/workspaces pages fit the new center column (toolbar inside center header, lanes scroll
      within the column); visual polish to match the reference.

## Progress / evidence

### 2026-09-30 — T1, T2, T3 implemented
- T1: `core/components/shell.rs` (`AppShell`, `CollapsePanel`, `SidebarSection`, `SidebarItem`, `TabBar`,
  `DropdownMenu`); `app.rs` composes the shell with the sidebar (`tasks/presentation/components/app_sidebar.rs`
  + `state/app_sidebar_state.rs`, refreshed via SSE/navigation); `AgentSettingsBar` moved to the sidebar footer
  (vertical). Top nav and `--nav-h` removed. SSE subscription extracted to `state/task_events.rs` (shared by board and sidebar).
- T2: task detail = header + scrolling transcript + pinned status pill/composer; right `CollapsePanel` with tabs
  Diff / Acciones / Estado (`DiffView`, `TaskActions`, `AgentSidebar`, `DialogPanel` unchanged).
- T3: board toolbar in the center header, lanes scroll within the column, workspaces page in the new column;
  tokens restyled to a cream/purple light theme with mono typography (single theme, no dark variant).
  Fixed the "Nueva tarea" label class that Tailwind never generated (`sm:{}` via `format!`).
- Checks: `cargo fmt --all -- --check` ok; `cargo clippy --features ssr --tests -- -D warnings` ok;
  `cargo clippy --features hydrate --target wasm32-unknown-unknown -- -D warnings` ok;
  `cargo test --features ssr` 219 passed; `cargo leptos build` ok.
- Visual (Playwright + chromium, fake agent): desktop 1440x900 board/workspaces/task (all tabs, collapse handle)
  and 390px board/workspaces/task + mobile sidebar overlay: 0px horizontal overflow, no console errors
  (one 404 for a static asset, likely favicon).
- Not verified: `diesel`/DB untouched; `end2end/ui_smoke.cjs` is stale vs. current UI copy and was not run;
  sidebar workspace list refreshes on navigation/SSE, not immediately after registering on `/workspaces`.
