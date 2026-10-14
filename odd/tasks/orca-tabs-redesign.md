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
- [ ] T1 (S2,S3) Pure dock layout model in `core/dock/layout.rs` + unit tests — commit
- [ ] T2 (S2,S3) Dock context, host (groups, tab bars, splits, resize, DnD), tab hooks — commit
- [ ] T3 (S2,S4) Wire pages/sidebar/app.rs to tabs; remove Routes — commit
- [ ] T4 (S1) Visual redesign (tokens, shell, sidebar, tab strip, status bar) — commit
- [ ] T5 Verify: fmt, clippy ssr+hydrate, lib tests, leptos build — commit docs

## Log
L1: Rediseñar aplicación usando estilos como los de las fotos. Mejorar UX utilizando pestañas para evitar colapsar el espacio de ventanas. Me gusta la funcionalidad del programa iworkflow, pero el diseño me gusta más el de las fotos. Además, como no tenemos pestañas, no se puede configurar el layout. Cambia eso.
