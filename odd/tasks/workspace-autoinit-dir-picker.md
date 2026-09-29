# workspace-autoinit-dir-picker

## Objective
1. Registering a workspace auto-creates the folder and initialises git when missing.
2. The workspace form gets a "Choose…" button that opens a server-backed directory picker.

## Scope / constraints
- `src/modules/workspaces` slice (+ `src/core/config.rs`, `src/app_state.rs`/`main.rs` wiring if needed). Follow AGENTS.md (explicit `let` types, no unwrap/expect outside tests, hexagonal layering, tests with each change, Spanish UI copy/errors like the rest of the slice).
- TDD: not enabled; functional checks per task. Runner: `cargo test --features ssr`.
- Route: delegated direct, one writer at a time. No SDD artifacts.
- Checks per task: `cargo fmt --all -- --check`, `cargo clippy --features ssr --tests -- -D warnings`, `cargo clippy --features hydrate --target wasm32-unknown-unknown -- -D warnings`, `cargo test --features ssr`, `cargo leptos build`.

## Design decisions
- Browsers cannot return absolute paths, so the picker lists directories server-side (app binds to 127.0.0.1).
- Auto-init is automatic (no toggle): missing dir -> `create_dir_all`; not a repo -> `git init -b <explicit base branch or "main">`; repo without commits -> empty initial commit built with plumbing (`git mktree` empty tree + `git commit-tree` + `git update-ref HEAD`) so the index is never committed. Fallback identity (`-c user.name=pi-workflow -c user.email=pi-workflow@localhost`) only when git has no identity configured.
- `GitRepositoryInfo` gains `has_commits`; `current_branch` reports the (possibly unborn) symbolic HEAD. Detached HEAD with commits still fails with `NoCurrentBranch`.

## Tasks
- [ ] T1 Auto-init on register (port + CLI adapter + use case + mocks + tests, incl. adapter tests on tempdirs)
- [ ] T2 Directory picker (port `DirectoryBrowserPort`, use case `BrowseDirectories`, fs adapter, server fn + DTO, picker state + generic component in Modal, "Choose…" button in form; tests for use case, adapter, DTO mapping)

## Progress / evidence

## Next step
T1.
