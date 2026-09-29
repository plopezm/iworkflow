# agent-thinking-level-selector

## Objective
Add a third global selector, right of the model selector in `AgentSettingsBar`, to choose the
reasoning effort (pi thinking level). It is persisted with the other agent settings and applied to
**newly started tasks** only (same rules as model: resumed tasks keep their session settings).

## Scope / constraints
- `src/modules/agent_settings`, `src/modules/tasks`, `src/app_state.rs`, `src/core/db/schema.rs`,
  `migrations/`, `style/tailwind.css`, `tests/modules/**`. Follow AGENTS.md strictly.
- Checks: `cargo fmt --all -- --check`, `cargo clippy --features ssr --tests -- -D warnings`,
  `cargo clippy --features hydrate --target wasm32-unknown-unknown -- -D warnings`,
  `cargo test --features ssr`, `cargo leptos build`.

## Verified facts
- pi CLI `--thinking <level>` with levels `off, minimal, low, medium, high, xhigh, max`. Works
  without `--model` (verified: RPC `get_state` reports `"thinkingLevel":"high"`), and pi clamps it to
  the model's capabilities, so no per-model validation is needed on our side.

## Design decisions
- Domain (agent_settings): enum `ThinkingLevel { Off, Minimal, Low, Medium, High, XHigh, Max }` with
  `as_str()`/`parse()` (pi's lowercase ids) and `ALL` list; `AgentSettings.thinking: Option<ThinkingLevel>`
  (None = pi default). Invalid stored/incoming value -> domain error (no panic).
- Migration `add_agent_settings_thinking`: `ALTER TABLE agent_settings ADD COLUMN thinking TEXT NULL`
  (down.sql drops the column — SQLite >= 3.35 supports DROP COLUMN; verify `diesel migration redo`
  against a temp DATABASE_URL). Regenerate schema.rs. Row model + adapter mapping.
- `PrepareLaunch` returns the model and the thinking level (e.g. a small `LaunchPreferences` struct)
  instead of only `Option<ModelRef>`.
- tasks: `LaunchOptions.thinking: Option<String>` (tasks owns its type; plain pi id); bridge in
  app_state maps it; `GentleShellRuntimeAdapter` appends `--thinking <level>` when set, independent of
  model. `ResumeTask` unchanged (default options).
- DTO `AgentSettingsDto.thinking: Option<String>` + mapping tests; `update_agent_settings` validates it.
  Serve the level list from the domain (server fn or static DTO list) — no duplicated hardcoding in UI
  beyond display labels.
- UI: third `OptionSelect` pill with a gauge/brain icon, placed immediately right of the model pill;
  "Por defecto" + Spanish labels (Off -> "Sin razonamiento", Minimal -> "Mínimo", Low -> "Bajo",
  Medium -> "Medio", High -> "Alto", XHigh -> "Muy alto", Max -> "Máximo"). Mobile second row becomes
  3 columns (`grid-cols-3`) with no horizontal overflow at 390px; nav height var `--nav-h` must stay
  correct. Keep hydration rules (Resources only inside Transition/Suspense; no signal writes from
  Suspend to outside).

## Tasks
- [x] T1 full vertical: domain + migration + adapter + use cases + DTO/server fns + tasks launch args + UI, with tests.

## Progress / evidence

### T1 (2026-09-29)
- `agent_settings` domain: `ThinkingLevel` enum (`domain/models/thinking_level.rs`) with
  `as_str()`/`parse()` (pi's lowercase ids) and `ThinkingLevel::ALL`; `AgentSettings.thinking:
  Option<ThinkingLevel>`; new error variant `AgentSettingsError::InvalidThinkingLevel(String)`.
  New `domain/models/launch_preferences.rs::LaunchPreferences { model, thinking }`;
  `PrepareLaunch::execute` now returns `Result<LaunchPreferences, AgentSettingsError>` instead of
  `Result<Option<ModelRef>, _>`. `UpdateAgentSettings` needed no change to its validation logic
  (per the verified fact that pi clamps thinking to the model, `thinking` flows through untouched,
  same as `profile`/`model` already do — no new catalog to check against).
- Migration `2026-09-29-210000-0000_add_agent_settings_thinking`: `ALTER TABLE agent_settings ADD
  COLUMN thinking TEXT NULL` (down: `DROP COLUMN`, SQLite >= 3.35). `diesel migration run` +
  `diesel migration redo` verified against a temp `DATABASE_URL=/tmp/pi-workflow-t1-thinking-db`
  (removed afterwards; the real `~/.pi-dashboard` db was never touched). `AgentSettingsRow` gained
  `thinking: Option<String>` mapped via `ThinkingLevel::as_str()`/`parse()` in both `From`/`TryFrom`
  directions.
- **Gotcha re-encountered from the previous slice:** `diesel migration run`/`redo` auto-regenerate
  `src/core/db/schema.rs` (per `diesel.toml`'s `print_schema.file`) against whatever `DATABASE_URL`
  is active at the time, and diesel's SQLite introspection still misreports the `id INTEGER PRIMARY
  KEY` column as `Nullable<Integer>`. Running the migration commands against the temp DB silently
  rewrote the checked-in `schema.rs` with that wrong type (breaking `cargo clippy --tests`, which
  actually type-checks the Diesel query against the struct); caught it via `cargo clippy --features
  ssr --tests` and corrected `id -> Integer` by hand afterward. `diesel print-schema` output otherwise
  matched the file exactly (confirmed with `diff`).
- `AgentSettingsDto.thinking: Option<String>` (mapping tests: full roundtrip incl. thinking, empty
  string -> `None`, invalid id -> `AgentSettingsError::InvalidThinkingLevel`). New server fn
  `list_thinking_levels() -> Vec<String>` serving `ThinkingLevel::ALL` ids directly from the domain
  (no repository/context needed, no duplicated hardcoding beyond the UI's Spanish labels).
- `tasks`: `LaunchOptions.thinking: Option<String>` (tasks' own type, plain pi id, independent of
  `model`). `GentleShellRuntimeAdapter` gained a `thinking_args` helper appending `--thinking
  <level>` right after the `--provider`/`--model` args, when set — independent of whether `model`
  is set. `StartTask`/`ResumeTask` needed no code change (they already thread `LaunchOptions` through
  `PrepareLaunch`/`LaunchSettingsPort` unchanged); only their existing tests' literals needed the new
  field.
- `src/app_state.rs`'s `LaunchSettingsBridge::prepare` now consumes `LaunchPreferences` (model +
  thinking) instead of a bare `Option<ModelRef>`, mapping `ThinkingLevel::as_str().to_string()` into
  `LaunchOptions.thinking`.
- UI: third `OptionSelect` pill (gauge icon) in `AgentSettingsBar`, placed immediately right of the
  model pill (order: model, thinking, profile), each with its own `select-pill-divider`. New
  `AgentSettingsState.thinking_levels: Resource<Result<Vec<String>, ServerFnError>>` (fetched once,
  like `models`) and `select_thinking(current, value)` (mirrors `select_model`/`select_profile`,
  preserving the other two fields). `select_model`/`select_profile` updated to also preserve
  `current.thinking` when dispatching. Spanish labels via `thinking_option_label` (`off` ->
  "Sin razonamiento", ..., `max` -> "Máximo", unknown id falls back to itself). Mobile grid changed
  from `grid-cols-2` to `grid-cols-3` (three equal columns); `md:flex` layout unchanged.
- Only edited within the allowed surfaces: `agent_settings/**`, `tasks/domain/models/launch_options.rs`,
  `tasks/infrastructure/adapters/outbound/runtime/gentle_shell_runtime_adapter.rs`,
  `tasks/application/usecases/start_task.rs` (tests only), `src/app_state.rs`, `src/core/db/schema.rs`,
  `migrations/**`, `tests/modules/agent_settings/**`, `tests/modules/tasks/gentle_shell_runtime_adapter_test.rs`.
  Did not touch `src/app.rs` (no route/nav wiring change needed — `AgentSettingsBar` was already
  wired in) or `style/tailwind.css` (existing `.select-pill*`/`.select-pill-divider` classes and the
  `grid-cols-*` utility already cover a third pill; no new class needed).

**Commands run and results:**
- `DATABASE_URL=/tmp/pi-workflow-t1-thinking-db/test.db diesel migration run` — ok (5 migrations
  applied incl. the new one).
- `DATABASE_URL=/tmp/pi-workflow-t1-thinking-db/test.db diesel migration redo` — ok (rolls back and
  reapplies the new migration cleanly).
- `DATABASE_URL=/tmp/pi-workflow-t1-thinking-db/test.db diesel print-schema` diffed against
  `src/core/db/schema.rs` — ok, identical (after the manual `id -> Integer` fix above); temp DB dir
  removed afterwards.
- `cargo build --features ssr` — ok, 0 warnings.
- `cargo fmt --all -- --check` — ok (after `cargo fmt --all`).
- `cargo clippy --features ssr --tests -- -D warnings` — ok, 0 warnings (only the pre-existing
  unrelated `proc-macro-error2` future-incompatibility notice).
- `cargo clippy --features hydrate --target wasm32-unknown-unknown -- -D warnings` — ok, 0 warnings
  (same unrelated notice).
- `cargo test --features ssr` — ok, 264 passed total (214 lib unit tests, up from 207: incl.
  `thinking_level` parse/as_str roundtrip + invalid, `PrepareLaunch` with thinking,
  `UpdateAgentSettings` saving a thinking-only settings object, DTO mapping incl. thinking, 2 new
  `StartTask` tests (model pass-through updated + new thinking pass-through), `thinking_option_label`
  unit tests; 50 integration tests, incl. `save_and_get_roundtrip_thinking_only` in
  `agent_settings_repository_adapter_test.rs` and 2 new `gentle_shell_runtime_adapter_test.rs` cases
  `appends_thinking_flag_when_set_with_model`/`appends_thinking_flag_when_set_without_model`), 0
  failed.
- `cargo leptos build` — ok (tailwind + wasm-bindgen + ssr binary built cleanly).
- Browser check (Playwright/Chromium headless, reusing the existing install at `/tmp/pwcheck`): ran
  the built `target/debug/pi-workflow` on `127.0.0.1:3211` with `DATABASE_URL` pointed at a **copy**
  of `~/.pi-dashboard/workspaces.db` under `/tmp/pi-workflow-thinking-check/` (the real dashboard db
  was never opened). At 390px and 1280px on `/`: `document.documentElement.scrollWidth ==
  window.innerWidth` in both cases (no horizontal overflow), zero console/page errors on load. Nav
  height matched `--nav-h` in both cases (390px: two-row mobile layout, `--nav-h: 6.75rem` (108px) vs
  measured 109px; 1280px: single-row, `--nav-h: 3.5rem` (56px) vs measured 57px — 1px rounding from
  `getBoundingClientRect`, not a mismatch). Verified the thinking select lists all 8 options ("Por
  defecto" + the 7 Spanish labels) sourced from `list_thinking_levels`. Chose "Alto" -> persisted
  (`value="high"`) across a full page reload; reset to "Por defecto" -> reload confirmed empty value,
  and a direct `sqlite3` query on the copied db showed `1||||` (all four columns NULL, incl.
  `thinking`) after the reset. Cleaned up: killed the server (`kill $(pgrep -x pi-workflow)`), deleted
  `/tmp/pi-workflow-thinking-check/` (incl. the DB copy) and the throwaway check scripts (kept the
  reusable Playwright install in `/tmp/pwcheck/` since it already existed). Did not commit.
  Screenshots saved to `/tmp/pi-workflow-thinking-390.png`, `/tmp/pi-workflow-thinking-1280.png`.

**Left unverified:** an actual `gentle-shell`/pi process picking up `--thinking <level>` end-to-end
(i.e. running a real task and inspecting its argv/behavior) — same category of gap already noted at
  the end of T2/T3 in `agent-settings-selectors.md` for `--provider`/`--model`; covered instead by the
  `gentle_shell_runtime_adapter_test.rs` integration tests against the fake RPC agent fixture (which do
  assert the exact flag/value appear in argv) and by code review of the bridge/mapping chain. Also,
  same as the earlier nav restyle note, `.select-pill` in light color-scheme was not screenshotted
  (screenshots above are in the default dark theme).
