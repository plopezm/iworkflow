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
- [ ] 4. UI
  - State resource refetched on `SubagentsChanged`; `SubagentsModal` component (list + detail) reusing core modal; button in `TaskActions`.
- [ ] 5. Full verification
  - fmt, clippy ssr + hydrate, `cargo test --features ssr`, `cargo leptos build`; codegraph refreshed; ARCHITECTURE.md note.

## Evidence
