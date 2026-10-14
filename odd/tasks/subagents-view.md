# Feature: subagents-view

List subagents and inspect what they did/do (equivalent to pi's `/gentle:agents`), from the task screen's actions, going through the agent runtime abstraction.
Branch: `pi/visualizar-subagentes-d1e5a1cf-707a-4ba3-8adf-1a3ad4a5103b` (current).

Decisions:
- Source: gentle-pi already pushes `gentle-agents.activity/v1` (widget `gentle-agents`, one JSON line: summary + `tasks[{summary, thread.items}]`), see `~/.pi/agent/npm/node_modules/gentle-pi/docs/gentle-agents-activity.md`. We already decode only the counters.
- Abstraction: transport-neutral `SubagentTask` domain model; new `AgentRuntimePort::subagents(task_id)` + capability `subagents`; router delegates to the recorded adapter; runtimes without the capability yield an empty list. Pi-specific JSON stays in `rpc_protocol.rs` / pi adapter.
- Snapshot is held in memory by the pi adapter (latest per task, kept after the agent exits until the next `start`). Not persisted in DB (out of scope: survives neither server restart nor task deletion).
- Live refresh: existing `SubagentsChanged` events trigger a refetch; no new event type.
- UI: button in the task actions opens a modal with the subagent list and a per-subagent detail (prompt, status, steps, thread items).

## Tasks

- [x] 1. Domain + port + use case
  - `SubagentTask`/`SubagentStatus`/`SubagentThreadItem` models, `AgentCapabilities.subagents`, `AgentRuntimePort::subagents`, mocks, router delegation, `GetTaskSubagents` use case + DI. Tests RED first.
- [x] 2. Pi adapter
  - Decode full `gentle-agents.activity/v1` snapshot in `rpc_protocol.rs`; adapter stores latest per task, reset on `start`, kept after exit; declares `subagents`.
- [x] 3. Inbound adapter
  - `SubagentTaskDto`, `get_task_subagents` server fn, DTO/error mapping tests.
- [x] 4. UI
  - State resource refetched on `SubagentsChanged`; `SubagentsPanel` component (inside core `Modal`) (list + detail) reusing core modal; button in `TaskActions`.
- [x] 5. Full verification
  - fmt, clippy ssr + hydrate, `cargo test --features ssr`, `cargo leptos build`; codegraph refreshed; ARCHITECTURE.md note.

## Evidence
- Task 4 (UI): `Transcript.subagents_version/subagents_active` bumped on `SubagentsChanged`; `TaskDetailState.subagents` resource (fetched only with the modal open, refetched on version/status change) + `selected_subagent` with `resolve_selection` fallback; `SubagentsPanel` (plain props) in core `Modal`; "Subagentes" button + active badge in `TaskActions`.
  - `cargo fmt --all -- --check`: ok
  - `cargo clippy --features ssr --all-targets -- -D warnings`: ok
  - `cargo clippy --features hydrate --target wasm32-unknown-unknown -- -D warnings`: ok
  - `cargo test --features ssr --lib modules::tasks`: 446 passed, 0 failed (new tests: transcript version bump, status label/variant, display name, selection fallback)
  - Not verified: visual rendering in a browser (no wasm/e2e test).
- Task 5: verifier run at `decf748`: fmt, clippy ssr + hydrate, 869 lib tests, 19 integration binaries (154 tests) each run separately, `cargo leptos build`: all ok; no forbidden imports in tasks domain/application. Codegraph refreshed. Not verified: browser rendering, real pi end to end, tasks 1-3 evidence not recorded by the worker (commits 4c7bde0, 21b56aa, 86ef083).
