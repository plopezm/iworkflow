# orca-tabs-redesign
Redesign the UI in the style of the Orca reference screenshots (flat dark panels, compact sidebar, tab strips)
and replace route-per-screen navigation with a tabbed, splittable workspace so windows no longer stack up.

## Specs
- S1: "Rediseñar aplicación usando estilos como los de las fotos." Flat, near-black, compact look: sidebar with
  nav rows + project/task rows, tab strips with `+` and `…`, thin borders, small radius, sans UI font.
- S2: "Mejorar UX utilizando pestañas para evitar colapsar el espacio de ventanas." Opening a task, terminal,
  board, workspace tree, etc. opens a tab in a tab group; re-opening an already open path activates its tab.
- S3: "como no tenemos pestañas, no se puede configurar el layout. Cambia eso." The workspace area is a tree of
  resizable tab groups: split right/down, move tabs between groups (drag and drop + menu), close, and the
  layout persists across reloads (localStorage).
- S4: "Me gusta la funcionalidad del programa iworkflow" — all existing routes/pages keep working, now as tabs.

## Tasks
- [x] T1 (S2,S3) Pure dock layout model in `core/dock/layout.rs` + unit tests — commit
- [x] T2 (S2,S3) Dock context, host (groups, tab bars, splits, resize, DnD), tab hooks — commit
- [x] T3 (S2,S4) Wire pages/sidebar/app.rs to tabs; remove Routes — commit
- [x] T4 (S1) Visual redesign (tokens, shell, sidebar, tab strip, status bar) — commit
- [x] T5 Verify: fmt, clippy ssr+hydrate, lib tests, leptos build — commit docs

## Evidence
- `cargo fmt --check`, `clippy --features ssr --tests`, `clippy --features hydrate --target wasm32` with `-D warnings`: ok.
- `cargo test --features ssr --lib`: 957 passed (incl. 14 dock layout tests, route parsing tests).
- `end2end/run_smoke.sh` (real Chromium): dock_smoke (tabs, dedupe, split, resize, drag, persistence), terminal keys/reconnect and ui_smoke (task flow, diff, merge) all pass.
- Not verified: integration tests under `tests/`, mobile (<768px) layout, light-theme visual pass beyond one screenshot.
- Known leftovers: dev-mode hydrate warnings from pre-existing pages (resource read outside Suspense).

## Log
L1: Rediseñar aplicación usando estilos como los de las fotos. Mejorar UX utilizando pestañas para evitar colapsar el espacio de ventanas. Me gusta la funcionalidad del programa iworkflow, pero el diseño me gusta más el de las fotos. Además, como no tenemos pestañas, no se puede configurar el layout. Cambia eso.
