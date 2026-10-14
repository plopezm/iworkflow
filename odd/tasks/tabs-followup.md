# tabs-followup
Follow-up of orca-tabs-redesign: remaining non-tab navigation, task actions menu and project file tree in Orca style.

## Specs
- S1: "No todo funciona con pestañas, abrir terminal en task por ejemplo." Every in-app link/open action (task "Abrir terminal", links inside forms, etc.) opens/activates a dock tab; none opens a browser window or reloads.
- S2: "Dale una vuelta al diseño del menú de acciones de las tasks" Task actions become a compact vertical action list (icon, label, danger separated), not a wrapped row of big buttons.
- S3: "visor de ficheros de los proyectos no me gusta porque me gustaría que se viera más como el estilo que te he pasado en un tree" Project viewer is a full-height file tree like the Orca explorer: header with project name + refresh, chevron + file/folder icons, dense rows.

## Tasks
- [x] T1 (S1) Replace remaining raw anchors with TabLink — commit
- [x] T2 (S2) Task actions list redesign — commit
- [x] T3 (S3) Shared tree row styles, project tree page, diff tree restyle, refresh — commit
- [x] T4 Verify fmt/clippy/tests/e2e smoke — commit

## Evidence
- Found/fixed: task "Abrir terminal" was `<a target=_blank>`; task modal link was a raw `<a>`; sidebar workspace list stayed stale after register/edit/delete (now `core/refresh.rs` DataRefresh bumped by the workspace actions).
- fmt, clippy ssr+hydrate (-D warnings): ok. `cargo test --features ssr --lib`: all pass.
- e2e `run_smoke.sh`: all steps pass incl. new "abrir terminal de la tarea abre una pestaña (sin ventana nueva)"; final console warnings are pre-existing (resource reads outside Suspense).
- Not verified: integration tests in `tests/`, mobile layout.

## Log
L1: No todo funciona con pestañas, abrir terminal en task por ejemplo. Dale una vuelta al diseño del menú de acciones de las tasks y visor de ficheros de los proyectos no me gusta porque me gustaría que se viera más como el estilo que te he pasado en un tree
