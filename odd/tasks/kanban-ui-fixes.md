# kanban-ui-fixes

## Objective
Fix all findings of the read-only UI verification of the kanban redesign (2026-09-29).

## Scope / constraints
- Presentation + create-task flow only. Follow AGENTS.md (explicit `let` types, no unwrap/expect outside tests, hexagonal layering, tests with each change).
- TDD: not enabled (no explicit config); functional checks per task. Runner: `cargo test --features ssr`.
- Route: delegated direct, one writer at a time. No SDD artifacts.
- Checks per task: `cargo fmt --all -- --check`, `cargo clippy --features ssr --tests -- -D warnings`, `cargo clippy --features hydrate --target wasm32-unknown-unknown -- -D warnings`, `cargo test --features ssr`.

## Tasks
- [x] T0 Baseline commit of verified kanban/Tailwind + backend race-fix work (branch `fix/kanban-ui-review`)
- [x] T1 (major) CreateTask returns CreatedTask{task,start_error}; server fn/DTO/board close modal + toast start error; no duplicate on retry
- [x] T2 (major) Task detail page: static shell; Composer/Transcript outside Suspend; no focus loss on status change
- [x] T3 Modal focuses first form field, not close button
- [x] T4 Detail page handles `removed` (message / navigate to board)
- [x] T5 Board filter "Necesita atención" consistent with counters (needs_attention)
- [x] T6 TranscriptView keyed `<For>` (keeps `<details>` open)
- [x] T7 Detail SSE `lagged` handler: defer reopen, reopen on CLOSED
- [x] T8 Minor batch: DTO pending_dialogs uses non-expired count + pluralisation; surface workspace load error; toast auto-dismiss; confirm on workspace "Quitar"; abort double-click guard; board keyed rendering / skip first reset refetch

## Progress / evidence
- T0: commit 87f8957 (baseline; fmt/clippy/test green before commit).
- T1: commit e9ae820; writer reported fmt, clippy ssr+hydrate, tests green; parent spot-check `cargo test --features ssr` ok. Route: delegated writer (2+ files). Not verified in browser.

- T2 ce417e5, T4 48d59ba, T6 ae666e2, T7 73da901: writer reported fmt, clippy ssr+hydrate, tests, cargo leptos build ok; parent spot-check clippy hydrate. Browser behaviour unverified. Limitation T6: a tool row receiving its output rebuilds (its details collapse).

- T3 2a7c9b8, T5 e393cc2, T8 c0b6241 bc2f58a 326c838 52be79b c7575e1 56a6d9e: writer reported fmt, clippy ssr+hydrate, tests (144 lib), cargo leptos build ok; parent spot-check fmt, clippy ssr, tests ok.
- Skipped: keyed lanes/cards in T8f (only first-reset refetch skip done; needs Memo outside Suspend); `with_untracked` params capture in task_detail_page (unverified, no in-app link between detail pages; cheap fix: key page on id). Nothing verified in a browser.

## Next step
Manual browser check; decide on keyed board rendering and params capture; push/PR is the user's call.
