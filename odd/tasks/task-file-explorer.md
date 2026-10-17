# task-file-explorer
Task panel: drop the "Estado" tab and add "Explorar archivos" (Revisión) that opens the same file explorer/editor
as a workspace, rooted at the task worktree, with git-colored entries and VS Code-like change bars in the editor gutter.

## Specs
- S1: "Estado no me aporta nada, quitar." The task side panel no longer has the "Estado" tab.
- S2: "En "Acciones" , en la sección de "Revisión" quiero una opción que sea explorar archivos." New action row
  "Explorar archivos" inside "Revisión" (shown whenever the task has a worktree).
- S3: "La funcionalidad tiene que ser la misma que cuando pulso un Workspace y se abre un explorador de archivos y editor."
  Opens a dock tab with the same tree + file editor, rooted at the task worktree (`/tasks/:id/files[/path]`).
- S4: "que se coloreen los ficheros modificados/añadidos (usando git)" Tree entries (and ancestor folders) are colored
  by git status (modified / added / untracked / deleted / conflicted), in workspaces and tasks.
- S5: "cuando edito que se marquen a la izquierda los cambios (como con vscode)" The editor gutter marks added,
  modified and deleted lines against git HEAD, live while editing.

## Tasks
- [x] T1 (S3) `ProjectScope` (workspace | task) + task root resolver + routes/pages reuse
- [x] T2 (S4,S5) Git outbound port + adapter, use cases, DTOs, server fns (status, base content)
- [x] T3 (S4) Tree coloring UI
- [x] T4 (S5) Pure line diff + gutter markers in CodeEditor
- [x] T5 (S1,S2) Task panel: remove Estado, add Explorar archivos
- [x] T6 Verify: fmt, clippy ssr+hydrate, lib tests

## Log
L1: Mira las fotos. Cuando se está ejecutando una tarea hay un panel a la derecha. En el panel verás 3 opciones, "Acciones", "Estado" y "Gitlab". Estado no me aporta nada, quitar. En "Acciones" , en la sección de "Revisión" quiro una opción que sea explorar archivos. La funcionalidad tiene que ser la misma que cuando pulso un Workspace y se abre un explorador de archivos y editor. Quiero además que se coloreen los ficheros modificados/añadidos (usando git) y cuando edito que se marquen a la izquierda los cambios (como con vscode). La primera foto muestre el menú y la segunda el explorador del que hablo.

## Evidence
- `cargo fmt --check`, `clippy --features ssr --tests`, `clippy --features hydrate --target wasm32` with `-D warnings`: ok.
- `cargo test --features ssr --lib`: 1299 passed. `--test git_cli_status_adapter_test`: 4 passed (real git repos).
- Not verified: browser rendering (tree colors, gutter bar alignment), other integration tests.
