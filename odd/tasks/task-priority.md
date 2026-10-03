# task-priority

## Objective
Assign a priority (Low/Normal/High/Urgent) to tasks on create/edit. Board lane order: oldest -> newest (created_at asc) unless priority puts a task above (priority desc first).

## Scope / constraints
- Follow AGENTS.md (explicit `let` types, no unwrap/expect outside tests, hexagonal layering, tests with each change, mocks.rs updated).
- Checks: `cargo fmt --all -- --check`, `cargo clippy --features ssr --tests -- -D warnings`, `cargo clippy --features hydrate --target wasm32-unknown-unknown -- -D warnings`, `cargo test --features ssr`, `diesel migration redo`.
- Route: one delegated writer.

## Tasks
- [x] T1 Domain+persistence: TaskPriority enum, Task.priority, NewTask/update, migration (up/down, schema.rs), row mapping, repository, use cases, tests
- [x] T2 DTO/server fns + presentation: priority select in create modal and edit page, badge on card, board ordering (priority desc, created_at asc), tests

## Progress / evidence
- T1: commit 95b48d6; writer reported fmt, clippy ssr+hydrate, 329 tests, migration redo ok.
- T2: commit c1d59cf; writer reported fmt, clippy ssr+hydrate, tests, cargo leptos build ok; parent re-ran fmt + tests ok. Not verified in a browser.
