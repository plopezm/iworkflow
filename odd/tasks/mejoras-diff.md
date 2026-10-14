# Feature: mejoras-diff

Human-friendly diff on the task detail page: one card per changed file with before/after (split view with a unified toggle), a tree of the changed files, and per-file editing (save = write in the task worktree + commit on the branch, so the diff refreshes).
Branch: `pi/mejoras-diff-a638328d-33a3-4426-af6f-8f6d0c9b8a56`.

Decisions (user): split with unified toggle; editor per file with save+commit; tree shows only modified files with A/M/D status and +/- counters.
Non-goals: editing files untouched by the task, merged/closed tasks (read-only), syntax highlighting, conflict resolution while the agent is writing.

## Tasks

- [x] 1. Pure patch model (presentation/state): parse a unified patch into `FileDiff` (path, old path, status A/M/D/R, +/- counts, hunks with split rows and old/new line numbers, binary flag) and build a folder tree of the changed files (folders aggregate counts). Unit tests.
- [x] 2. Backend file read/save: outbound `TaskFilePort` (read/write inside the task worktree, relative-path validation against traversal/absolute/.git, size and binary guards) + fs adapter; use cases `GetTaskFile` / `SaveTaskFile` (only open tasks, write then `WorktreePort::commit_all`); DI in tasks context; `#[server]` fns + DTOs; mocks. Tests: use cases (mockall), adapter integration (tempfile), DTO/error mapping.
- [x] 3. UI: rebuild `DiffView` with file tree sidebar (click scrolls to file), per-file cards with split/unified toggle, collapse, inline editor (load, save, cancel, error), diff refetch after save; disable edit for merged/closed tasks and deleted files; Tailwind styles; keep PDF export working.
- [ ] 4. Verification: fmt, clippy ssr + hydrate, `cargo test --features ssr`, update codegraph.

## Evidence
- Task 1: commit a7328b9 — `cargo test --features ssr diff_files` 13 passed; fmt, clippy ssr + hydrate clean.
- Task 2: see commit message/HEAD — `cargo test --features ssr tasks` 485 passed; fmt, clippy ssr + hydrate clean.
- Task 3: `cargo test --features ssr --lib tasks` 503 passed; fmt, clippy ssr + hydrate (wasm) clean. `cargo leptos build` and visual smoke check not run.
