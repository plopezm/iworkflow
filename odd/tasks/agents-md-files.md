# agents-md-files

## Objective
A new section to manage reusable `AGENTS.md` files. Each file has a description. Files are uploaded, stored on disk (never in SQLite), and can be installed into a **new** project (at workspace registration) or added to an **existing** workspace.

## Scope / constraints
- New slice `src/modules/agents_files` + small integration in `src/modules/workspaces` (new-project flow) and `src/app_state.rs` / `src/core/config.rs` wiring. Follow AGENTS.md (explicit `let` types, no unwrap/expect outside tests/main, hexagonal layering, tests with each change, `mocks.rs` per slice, Spanish UI copy/errors like the rest of the app).
- TDD: not enabled; functional checks per task. Runner: `cargo test --features ssr`.
- Route: delegated direct, one writer at a time. No SDD artifacts. Do not commit.
- Checks per task: `cargo fmt --all -- --check`, `cargo clippy --features ssr --tests -- -D warnings`, `cargo clippy --features hydrate --target wasm32-unknown-unknown -- -D warnings`, `cargo test --features ssr`, and `cargo leptos build` for UI tasks.

## Design decisions
- Storage dir: `AppConfig.agents_files_dir` = env `AGENTS_FILES_DIR` or `<parent dir of DATABASE_URL>/agents-files` (i.e. `~/.pi-dashboard/agents-files`). Stored file name is `<uuid>.md` (never the user-supplied name: no path traversal, no collisions).
- DB table `agents_files(id TEXT PK, description TEXT NOT NULL, path TEXT NOT NULL UNIQUE, created_at TIMESTAMP NOT NULL)`: only the reference (id), the path and the description. Content lives only on disk.
- Validation (domain): description trimmed, 1..=200 chars; content non-empty, UTF-8, <= 512 KiB.
- Upload is a real multipart upload: `#[server(input = MultipartFormData)]` fields `description` + `file`. Client dispatches `FormData` through a local `Action`.
- Installing writes `<project root>/AGENTS.md` and then commits ONLY that path (`git add -- AGENTS.md` + `git commit --only`), because tasks run in worktrees created from the base branch and an uncommitted file would not be visible to agents. Fallback identity `-c user.name=pi-workflow -c user.email=pi-workflow@localhost` only when git has none configured (same as workspace initial commit). A commit failure does not fail the install: outcome `committed: false`.
- Existing `AGENTS.md` at the target: `AgentsFileError::TargetExists` unless `overwrite`. Existing-workspace flow reports it as typed outcome `TargetExists` so the UI can ask for confirmation and resend with `overwrite: true`. New-project flow never overwrites (error).
- Cross-feature access goes through each slice's own outbound port, bridged in `app_state.rs`:
  - `agents_files::ProjectLookupPort` (list/find workspaces) <- workspaces use cases.
  - `workspaces::AgentsFilesPort` (list options / install into a root) <- agents_files use cases.
- New-project flow: `NewWorkspace.agents_file_id: Option<String>`; `RegisterWorkspace` installs it after the workspace has a base branch/commit and before saving; failure aborts the registration.

## Tasks
- [x] T1 Backend slice `agents_files`: domain (models, errors, outbound ports), use cases (Upload, List, Delete, InstallAgentsFile, ApplyAgentsFileToProject, ListProjects), migration + schema.rs, Diesel repository adapter, fs storage adapter, git-aware installer adapter, DTOs + server fns (incl. multipart upload), DI context, config, `app_state` wiring, `mocks.rs`, unit + integration tests, `[[test]]` entries in Cargo.toml.
- [x] T2 New-project flow: workspaces `AgentsFilesPort`, `ListAgentsFileOptions`, `NewWorkspace.agents_file_id`, `RegisterWorkspace` install step, DTO/server fn, bridge, form select + state, tests.
- [x] T3 UI `agents_files` presentation: page `/agents-files`, state, upload form, list with per-file "add to project" (select workspace, overwrite confirmation, committed/not-committed feedback) and delete; route + nav entry; `cargo leptos build`; headless browser check.

## Progress / evidence
- T1 done (not committed). Slice `src/modules/agents_files` (domain, use cases, Diesel/fs/git adapters, DTOs, multipart server fns, DI, mocks), migration `2026-09-30-180000-0000_create_agents_files` (`diesel migration run` + `redo` ok; schema.rs diff = new table + allow_tables only), `AppConfig.agents_files_dir`, `AppState.agents_files` + `ProjectLookupBridge`, `multipart` feature of leptos, `multer` (optional, ssr) to type/test the streaming cap, 3 `[[test]]` entries.
- Checks: `cargo fmt --all -- --check` ok; `cargo clippy --features ssr --tests -- -D warnings` ok; `cargo clippy --features hydrate --target wasm32-unknown-unknown -- -D warnings` ok; `cargo test --features ssr --no-fail-fast` all green (259 unit + integration incl. repository 4, storage 3, installer 6).
- Not verified: `cargo leptos build`, real browser multipart upload (page is T3).
- T2 done (not committed). workspaces: `AgentsFilesPort` + `AgentsFileOption`, error variants, `NewWorkspace.agents_file_id`, `RegisterWorkspace` installs after base branch/initial commit and before save (error aborts), `ListAgentsFileOptions`, DTO + `list_agents_file_options` server fn, context wiring, `AgentsFilesBridge` in app_state (agents_files context is built before workspaces, no cycle), form select (shown only when options exist) + state. Unit tests + mocks (`MockAgentsFiles`); no new integration test.
- Checks: fmt ok; clippy ssr --tests ok; clippy hydrate ok; `cargo test --features ssr --no-fail-fast` green (272 unit).
- Not verified: `cargo leptos build`, browser check of the form select (hydration).

- T3 done (not committed). Presentation for `agents_files`: `AgentsFilesState` (Resources `agents_files`/`projects`, local `Action<FormData,..>` multipart upload, ServerActions delete/apply, `ApplyFeedback` memo derived from the last apply, overwrite confirm resending last input with `overwrite: true`, dismiss), components `AgentsFileUploadForm`/`AgentsFileList`/`AgentsFileItem`, page `AgentsFilesPage`, route `/agents-files` + "AGENTS.md" menu item, core `ConfirmPanel` (copy of the workspaces one, so the slice does not import another feature) and optional `name` prop on core `TextField` (multipart field name). No Cargo changes (`leptos::web_sys`/`wasm_bindgen` re-exports).
- Checks: fmt ok; clippy ssr --tests ok; clippy hydrate ok; `cargo test --features ssr --no-fail-fast` green (277 unit + integration); `cargo leptos build` ok.
- Browser (Playwright/Chromium headless, temp DB/dirs under /tmp, port 3457, cleaned up): nav entry -> /agents-files; upload .md with description -> listed, form reset; disk has only `<uuid>.md`, DB row = id/description/path/created_at; apply to a registered git workspace -> AGENTS.md present + commit "docs: add AGENTS.md", clean status; second apply -> overwrite prompt, Cancelar dismisses, Sobrescribir resends (identical content => git has nothing to commit => "Añadido (no se pudo crear el commit; queda sin commitear)"); new-project registration with the AGENTS.md select (+ init confirm) -> AGENTS.md committed on top of the initial commit; delete (native confirm) removes row and file; 0 console errors/warnings/HTTP>=400; 390px no horizontal overflow.
- Not verified: apply with an unreadable/missing target dir error path in the UI, and dark-mode visuals of the new page.

## Next step
None (all tasks done; pending review/commit).
