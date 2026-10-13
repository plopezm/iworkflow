# Feature: agent-runtime-abstraction

Decouple the tasks module from pi/gentle-shell so other agents (not necessarily RPC/stdio) can be added.
Branch: `pi/actualmente-iworkflow-soporta-pi-y-gentl-82f5ddd7-3aa7-4ff7-89a6-8bed8186bb27` (current).

Decisions:
- `AgentRuntimePort` is the transport-neutral contract (no process/RPC notions). Process supervision + JSONL stay inside the pi adapter.
- Capabilities are declared per runtime; unsupported commands are REJECTED with an explicit `TaskError` (no silent degradation).
- Runtime selected by a typed `RuntimeId` (not `String`); an `AgentRuntimeRouter` implements the port and delegates per task.
- pi and gentle-shell = one `PiRpcRuntimeAdapter` registered twice with different binaries.
- Out of scope: gentle-pi profiles/pin, settings UI redesign (UI only adapts to `RuntimeId`).

## Tasks

- [x] 1. Domain contract + capability checks
  - `RuntimeId` value object, `AgentCapabilities` (steer, follow_up, dialogs, model_selection, thinking_levels, resume), `TaskError::Unsupported`.
  - Port: neutral docs (`resume` semantics owned by port), `capabilities(task_id)`; `LaunchOptions.runtime: Option<RuntimeId>`.
  - `SendMessage`/`AnswerDialog`/resume/start use cases reject unsupported commands/options; update `mocks.rs`; unit tests (RED first).
- [x] 2. Pi adapter extraction
  - Move `GentleShellRuntimeAdapter` + `rpc_protocol.rs` into `runtime/pi/` as `PiRpcRuntimeAdapter` (single binary per instance, no `program_for`); declares full capabilities.
  - Update integration tests under `tests/modules/tasks/`.
- [x] 3. Router, registry and DI
  - `AgentRuntimeRouter` (task -> adapter map recorded at `start`, default runtime), registered in `tasks_context.rs`; config passes binaries per runtime id.
  - Model catalog: per-runtime registry replacing the `match` in `AgentCliModelCatalogAdapter`; runtime without catalog -> empty list.
  - Router unit tests + DI wiring test.
- [x] 4. Full verification
  - fmt, clippy ssr + hydrate, `cargo test --features ssr`, `cargo leptos build`; codegraph refreshed.
- [x] 5. Fix native review findings (lineage review-7096ec7a931d786d, all non-blocking)
  - SendMessage: check `is_running` before capabilities so a stopped task yields `AgentNotRunning`, not `Unsupported`.
  - Router: reject `start` when the recorded adapter for that task is still running; `stop` must not prune an entry re-created by a concurrent `start` (generation check).
  - Enforce capabilities at launch: port gets `runtime_capabilities(Option<&RuntimeId>)`; ResumeTask rejects with `Unsupported` if `resume` is missing; StartTask drops model/thinking the runtime does not support.
  - Test `build_router` wiring (ids registered, default, unknown id rejected); test in `app_state.rs` that the local runtime id consts equal `AgentRuntime::as_str()`.
  - Fix the task doc: DI wiring test claim (task 3) and wording consistency.

## Evidence

- Task 1: `bcfccb2` — RED observed (10 compile errors on new capability tests); 397 lib tasks tests + 6 integration binaries, fmt, clippy ssr + hydrate ok. Resume capability check deferred to task 3 (needs runtime known before start).
- Task 2: `0ec6d6a` — pure refactor, no RED applicable; 400 lib tasks tests + 6 integration binaries (pi_rpc_runtime_adapter_test 9), fmt, clippy ssr/hydrate/tests ok. TemporaryRuntimeSelector to be replaced in task 3.
- Task 3: `84fe5c5` — router mutation-style RED (3 tests failed against a broken router; not genuine test-first); 406 tasks + 50 agent_settings lib tests, integration binaries ok, fmt, clippy ssr/hydrate/tests ok. TasksContext::new/AppState::new now return Result. Resume capability pre-check (capabilities_for) not implemented.
- Task 4: verified at `84fe5c5`; docs fix `bff0bf5`. fmt, clippy ssr+hydrate, full cargo test --features ssr (21 result lines, 0 failures, 828 lib), cargo leptos build ok; no forbidden imports in tasks domain/application. Not verified: real pi/gentle-shell binaries end to end; codegraph not refreshed.
- Task 5: `95f337f` — RED observed (12 failing new tests); 844 lib tests + 20 integration binaries, fmt, clippy ssr/tests/hydrate ok. No process-spawn test for build_router (login shell always spawns). Real pi/gentle-shell binaries still not exercised end to end.
