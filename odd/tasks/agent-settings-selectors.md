# agent-settings-selectors

## Objective
Two global selectors in the top bar: the orchestrator model (pi `/model`) and the gentle-pi
agent-model profile (`/gentle:profiles`). The selection is persisted and applies to **newly
started tasks** (not to resumed ones, not to running agents).

## Scope / constraints
- New slice `src/modules/agent_settings`, plus `src/modules/tasks`, `src/app_state.rs`, `src/app.rs`,
  `src/core/config.rs`, `migrations/`. Follow AGENTS.md strictly (hexagonal, explicit `let` types,
  no unwrap/expect/unsafe outside tests, no `Effect` to coordinate backend calls).
- Features must not import each other: tasks gets its own outbound port bridged in `app_state.rs`.
- Checks: `cargo fmt --all -- --check`, `cargo clippy --features ssr --tests -- -D warnings`,
  `cargo clippy --features hydrate --target wasm32-unknown-unknown -- -D warnings`, `cargo test --features ssr`.

## Verified facts (external tools)
- pi CLI: `--provider <name> --model <id>` select the model for that process only (non-persistent).
  `gentle-shell` forwards unknown args to pi.
- `gentle-shell --list-models` prints a whitespace table; header line
  `provider model context max-out thinking images`, then one row per model (first two columns matter).
- Profiles store: `<GENTLE_PI_CONFIG_HOME or ~/.pi/gentle-ai>/profiles.json`:
  `{"kind":"gentle-pi.agent_model_profiles","version":..,"active":"Claude"|absent,"profiles":{"<name>":{..}}}`.
- Per-repository local pin (resolved by gentle-pi at every launch, routes subagents only, never the
  orchestrator): `<git-common-dir>/gentle-ai/profile-pin.json` with exact content
  `{\n  "kind": "gentle-pi.agent_model_profile_pin",\n  "version": 1,\n  "profile": "<name>"\n}\n`.
  `git rev-parse --git-common-dir` from a task worktree gives the clone's common dir.
  Valid profile name: 1-64 chars of `[A-Za-z0-9._-]`, starting with a letter or digit.

## Design decisions
- Replace the previous broken attempt: delete `src/core/settings/`, `src/core/components/settings_selector.rs`,
  revert its lines in `src/core/mod.rs`, `src/core/components/mod.rs`, `src/app.rs`, and the `js-sys`
  line in `Cargo.toml` (+ Cargo.lock). Backup at `/tmp/pi-workflow-backup/`.
- agent_settings domain: `ModelRef { provider, model }`, `ProfileName` (validated newtype),
  `AgentSettings { model: Option<ModelRef>, profile: Option<ProfileName> }` (None = "use pi/gentle
  defaults"), `ProfileCatalog { active: Option<String>, names: Vec<String> }`,
  `AgentSettingsError` (thiserror).
- Outbound ports: `AgentSettingsRepositoryPort { get, save }`, `ModelCatalogPort { list }`,
  `ProfileCatalogPort { list }`, `ProfilePinPort { apply(repo_dir: &Path, profile: Option<&ProfileName>) }`
  (None removes the local pin file if present; missing file is not an error).
- Use cases: `GetAgentSettings`, `ListModels`, `ListProfiles`, `UpdateAgentSettings` (rejects a model
  not in the catalog / a profile not in the store), `PrepareLaunch::execute(cwd) -> Option<ModelRef>`
  (reads settings, applies the pin for `cwd`, returns the model).
- Persistence: migration `create_agent_settings`: single row table
  (`id INTEGER PRIMARY KEY CHECK (id = 1)`, `model_provider TEXT NULL`, `model_id TEXT NULL`,
  `profile TEXT NULL`); missing row = defaults; save = upsert.
- Adapters: `GentleShellModelCatalogAdapter` (runs `<gentle_shell_bin> --list-models` with a timeout,
  pure parse fn unit-tested), `ProfilesFileCatalogAdapter` (reads profiles.json; missing file = empty
  catalog), `GitProfilePinAdapter` (git common dir + atomic write via temp file + rename).
  `AppConfig` gains `gentle_pi_config_home: PathBuf`.
- tasks: domain model `LaunchOptions { model: Option<(String provider, String model)> }` (own type),
  outbound `LaunchSettingsPort { prepare(cwd: &Path) -> Result<LaunchOptions, TaskError> }`.
  `AgentRuntimePort::start` gains `options: &LaunchOptions`; adapter appends
  `--provider P --model M` when set. `StartTask` calls `prepare(worktree)` before starting;
  `ResumeTask` passes `LaunchOptions::default()` (the pi session keeps its model).
  Bridge `LaunchSettingsBridge` in `app_state.rs` maps `PrepareLaunch` -> `LaunchSettingsPort`.
- UI: server fns `get_agent_settings`, `list_model_options`, `list_profile_options`,
  `update_agent_settings(dto)`; state `presentation/state/agent_settings_state.rs`
  (Resources + ServerAction, refetch after update); a generic select component reused by both
  selectors; `AgentSettingsBar` rendered in the top nav of `app.rs`. First option of each select is
  "Por defecto" (None). Profile select shows which one is globally active.

## Tasks
- [x] T1 agent_settings backend: domain, ports, use cases (+ mockall tests), migration + schema,
      adapters (+ integration test for the Diesel repo, unit tests for parsers/pin writer in a temp git repo),
      DTOs + server fns (+ mapping tests), context, AppConfig field, wiring in AppState. Remove old attempt.
- [x] T2 tasks integration: LaunchOptions, LaunchSettingsPort (+ mock), runtime port/adapter args,
      StartTask/ResumeTask (+ tests), bridge in app_state.
- [x] T3 UI: state, generic select component, AgentSettingsBar in top nav (mobile friendly, Tailwind).

## Progress / evidence

### T1 (2026-09-29)
- Removed the previous broken attempt: `src/core/settings/` (dto.rs, server_fns.rs) and
  `src/core/components/settings_selector.rs` were untracked and deleted; reverted the
  corresponding lines in `src/core/mod.rs`, `src/core/components/mod.rs`, `src/app.rs`, and the
  `js-sys` line in `Cargo.toml`. `git status`/`git diff` confirm these files now match `HEAD`
  exactly (the broken attempt had never been committed), so `Cargo.lock` needed no changes.
  Backup kept at `/tmp/pi-workflow-backup/`.
- New slice `src/modules/agent_settings/` (domain: `ModelRef`, `ProfileName` validated newtype,
  `AgentSettings`, `ProfileCatalog`, `AgentSettingsError`; outbound ports
  `AgentSettingsRepositoryPort`, `ModelCatalogPort`, `ProfileCatalogPort`, `ProfilePinPort`;
  use cases `GetAgentSettings`, `ListModels`, `ListProfiles`, `UpdateAgentSettings`,
  `PrepareLaunch`; adapters `AgentSettingsRepositoryAdapter` (Diesel, `replace_into` upsert on a
  single row id=1), `GentleShellModelCatalogAdapter` (`--list-models`, timeout + pure parser),
  `ProfilesFileCatalogAdapter` (`profiles.json`, missing file = empty catalog),
  `GitProfilePinAdapter` (git common dir + atomic write via temp file + rename); DTOs + server
  fns `get_agent_settings`, `list_model_options`, `list_profile_options`,
  `update_agent_settings`; DI `AgentSettingsContext`; `mocks.rs` with mockall for the four
  outbound ports).
- `AppConfig` gained `gentle_pi_config_home: PathBuf` (`GENTLE_PI_CONFIG_HOME` env or
  `~/.pi/gentle-ai`). `AppState` now builds and provides `AgentSettingsContext`.
- Migration `2026-09-29-162425-0000_create_agent_settings` (single-row `agent_settings` table);
  `diesel migration redo` verified against a temp `DATABASE_URL` (`/tmp/pi-workflow-t1-db`,
  removed afterwards; the real `~/.pi-dashboard` db was never touched). `src/core/db/schema.rs`
  regenerated via `diesel print-schema` (with `id -> Integer`, not `Nullable<Integer>`, since
  diesel's SQLite introspection misreports `INTEGER PRIMARY KEY` as nullable).
- Deviation from the design note: `ProfilePinPort::apply` needed an explicit `<'a>` lifetime on
  both the trait and its implementations (production adapter and mockall mock) because mockall's
  `mock!` macro does not go through `async_trait`'s elision rewriting for methods with two
  independent reference parameters; without it, `mock!` fails to compile (`E0106`/`E0195`).
  `PrepareLaunch::execute` returns `Result<Option<ModelRef>, AgentSettingsError>` rather than a
  bare `Option<ModelRef>`, per AGENTS.md's ban on `unwrap`/`panic!` outside tests.
- `tasks` slice untouched (T2 scope); no UI added (T3 scope).

**Commands run and results:**
- `diesel migration generate create_agent_settings`, `diesel migration run`,
  `diesel migration redo`, `diesel print-schema` against `DATABASE_URL=/tmp/pi-workflow-t1-db/test.db`
  — ok (redo rolls back and reapplies cleanly).
- `cargo build --features ssr` — ok, 0 warnings.
- `cargo fmt --all -- --check` — ok (after `cargo fmt --all`).
- `cargo clippy --features ssr --tests -- -D warnings` — ok, 0 warnings.
- `cargo clippy --features hydrate --target wasm32-unknown-unknown -- -D warnings` — ok, 0 warnings.
- `cargo test --features ssr` — ok, 246 passed (200 lib unit tests incl. 27 new for
  `agent_settings`, 3 new `agent_settings_repository_adapter_test`, 4 new
  `git_profile_pin_adapter_test`, plus all pre-existing workspaces/tasks integration tests
  unaffected), 0 failed.

**Left unverified:** end-to-end wiring with `tasks` (T2) and the UI selectors (T3) — out of
scope for T1. The real `gentle-shell --list-models` / `profiles.json` / git-pin adapters were
only tested via unit tests (pure parsers) and integration tests against fixtures/temp repos, not
against a real `gentle-shell` binary or a real `~/.pi/gentle-ai/profiles.json`.

### T2 (2026-09-29)
- `tasks` slice: new domain model `LaunchOptions { model: Option<LaunchModel { provider, model }> }`
  (own type, `Default`) in `domain/models/launch_options.rs`; new outbound port
  `LaunchSettingsPort { async fn prepare(&self, cwd: &Path) -> Result<LaunchOptions, TaskError> }`
  in `domain/ports/outbound/launch_settings_port.rs`, with `MockLaunchSettings` added to
  `mocks.rs`. `AgentRuntimePort::start` gained an `options: &LaunchOptions` parameter (also updated
  in `mocks.rs`'s `MockAgentRuntime` and every call site).
- `GentleShellRuntimeAdapter::start` appends `--provider <p> --model <m>` (via a small
  `model_args` helper) right after the existing `--continue` flag when `options.model` is `Some`;
  unchanged (no extra args) when `None`.
- `StartTask` gained a `launch_settings: Arc<dyn LaunchSettingsPort>` field/constructor param;
  `launch()` now calls `launch_settings.prepare(&task.worktree_path)` before `runtime.start(..)`
  and propagates a `prepare` error through the existing `Err(error)` branch of `execute` (task
  transitions to `Failed` with the error message, exactly like a `runtime.start` failure).
  `ResumeTask` passes `&LaunchOptions::default()` (resuming keeps whatever model the previous pi
  session already had).
  `TasksContext::new`/`with_runtime` gained a `launch_settings: Arc<dyn LaunchSettingsPort>`
  parameter, forwarded into `StartTask::new`; all callers (`app_state.rs`, and the test
  constructors in `start_task.rs`/`create_task.rs`) were updated.
- `src/app_state.rs`: `AgentSettingsContext` is now built *before* `TasksContext` (so its
  `prepare_launch` use case is available); new `LaunchSettingsBridge` implements
  `LaunchSettingsPort` over `agent_settings.prepare_launch`, mapping `ModelRef -> LaunchModel` and
  `AgentSettingsError -> TaskError::Runtime(error.to_string())` (no existing `TaskError` variant
  fit better than `Runtime`, since a failed launch-settings lookup is, from the tasks feature's
  point of view, a reason the agent could not be started).
- Tests added: `start_task.rs` unit tests `passes_prepared_model_to_runtime_start` (asserts the
  model returned by `prepare` reaches `runtime.start`'s `options` argument) and
  `marks_task_failed_when_launch_settings_prepare_fails` (a `prepare` error leaves the task
  `Failed` with that error's message, same as the existing `marks_task_failed_when_agent_cannot_start`
  test for `runtime.start` failures). `gentle_shell_runtime_adapter_test.rs` gained
  `appends_provider_and_model_flags_when_set`, using the existing `fake_agent.py` fixture (which
  echoes `args=...` in a boot notice) to assert `--provider anthropic --model claude-5` appear in
  the spawned process's argv.
- Deviation from the design note: the bridge's error mapping uses `TaskError::Runtime` rather than
  a new variant, since `TaskError` already models "the agent could not be started" that way and
  AGENTS.md/the design note don't call for a new variant.

**Commands run and results:**
- `cargo build --features ssr` — ok, 0 warnings.
- `cargo fmt --all -- --check` — ok (after `cargo fmt --all`).
- `cargo clippy --features ssr --tests -- -D warnings` — ok, 0 warnings.
- `cargo clippy --features hydrate --target wasm32-unknown-unknown -- -D warnings` — ok, 0 warnings.
- `cargo test --features ssr` — ok, 249 passed total (202 lib unit tests, incl. 2 new for
  `StartTask`; 6 `gentle_shell_runtime_adapter_test` incl. 1 new; all other integration suites
  unaffected and still green), 0 failed.
- Manual boundary check (`grep 'use (leptos|diesel|axum)'` over `modules::tasks::domain` and
  `modules::tasks::application`) — no matches, hexagonal boundary intact.

**Left unverified:** UI selectors (T3) — out of scope for T2. The bridge and `PrepareLaunch` are
only exercised through unit/integration tests (mocked ports and the fake RPC agent script), not
against a real `gentle-shell`/pi process picking up `--provider`/`--model`.

### T3 (2026-09-29)
- New generic `OptionSelect` in `src/core/components/select.rs`: controlled by an external
  `value: String` snapshot + `on_change: Callback<String>` (not a bound `RwSignal` like the
  existing `SelectField`, since the two selectors here must dispatch a server action on every
  change rather than just hold local form state) with a `disabled: Signal<bool>` prop and an
  always-present empty "Por defecto" option. Left the existing `SelectField` untouched (still
  used by `task_modal.rs`).
- `agent_settings/presentation/state/agent_settings_state.rs`: `AgentSettingsState` with three
  `Resource`s (`settings`, `models`, `profiles`; `settings`/`profiles` re-fetch on
  `update.version()`, `models` loads once) and one `ServerAction<UpdateAgentSettings>`;
  `select_model`/`select_profile` build the next `AgentSettingsDto` from the *currently loaded*
  settings plus the new selection and dispatch the update immediately (no separate save button —
  each selector persists on change, as the design calls for). A private `MODEL_VALUE_SEPARATOR`
  (`'\u{1}'`, unit-tested) encodes `(provider, model)` into a single `<option>` value since a
  plain `/` or `-` could collide with real provider/model names.
- `agent_settings/presentation/components/agent_settings_bar.rs`: `AgentSettingsBar` renders the
  model and profile selects, each in its **own** `<Transition>`/`Suspend` awaiting only the
  resources it needs (model select awaits `settings` + `models`; profile select awaits `settings`
  + `profiles`) — the design's requirement that a slow `--list-models` must not block the profile
  selector or the rest of the page. Model options are labeled `provider/model`; profile options
  are labeled `name` or `name (activo)` when `name` matches `ProfileCatalogDto.active`. An
  `Effect` (client-only, same pattern as `board_page.rs`) watches `update.value()` and pushes a
  toast (reusing `core::components::toast::ToastStack`) on error. The bar carries a `title`
  tooltip explaining changes apply only to newly started tasks (not resumed/running ones).
- Wired `<AgentSettingsBar />` into the top nav in `src/app.rs`, between the logo and the
  Tablero/Workspaces pills; the bar's own flex-wrap + compact `text-xs`/`py-1.5` select styling
  keeps the nav usable on narrow screens without a separate mobile layout.
- **Bug found and fixed during browser smoke testing:** `update_agent_settings`'s default
  url-encoded server-fn body encoding drops the entire nested `input` argument when every field
  of `AgentSettingsDto` is `None` (both selectors reset to "Por defecto"), so the server replied
  `500` with `missing field 'input'`. Fixed by annotating
  `#[server(input = server_fn::codec::Json)]` on `update_agent_settings` (JSON always serializes
  the struct, even fully empty); re-verified with the same smoke script afterwards.
- Only edited within the allowed surfaces: `agent_settings/presentation/**`, `agent_settings/mod.rs`
  (added `pub mod presentation;`), `agent_settings/infrastructure/adapters/inbound/server_fns.rs`
  (the `input = Json` fix), `core/components/select.rs` (+ `mod.rs`), and `app.rs`. Did not touch
  `src/modules/workspaces` (its pre-existing uncommitted change in `workspace_table.rs` was left
  exactly as found).

**Commands run and results:**
- `cargo build --features ssr` — ok, 0 warnings.
- `cargo leptos build` — ok (tailwind + wasm-bindgen + ssr binary all built cleanly).
- `cargo fmt --all -- --check` — ok (after `cargo fmt --all`).
- `cargo clippy --features ssr --tests -- -D warnings` — ok, 0 warnings.
- `cargo clippy --features hydrate --target wasm32-unknown-unknown -- -D warnings` — ok, 0 warnings.
- `cargo test --features ssr` — ok, 254 passed total (207 lib unit tests, incl. 5 new for
  `agent_settings::presentation`: 3 for the model-value codec, 2 for the model/profile option
  labels; all other suites unaffected), 0 failed.
- Browser smoke test (Playwright/Chromium, headless, via a throwaway `npm install playwright`
  project): ran the built `target/debug/pi-workflow` binary directly (not `cargo leptos watch`)
  on `127.0.0.1:3099` with `DATABASE_URL` pointed at a **copy** of `~/.pi-dashboard/workspaces.db`
  under `/tmp/pi-workflow-t3-db/` (the real dashboard db was never opened by the test run), and
  against the real `gentle-shell --list-models` and the real `~/.pi/gentle-ai/profiles.json`
  (`active: "Claude"`, profiles `Claude`/`Local`/`current`). Verified: zero console/page errors
  on load and after every interaction (including the `input = Json` bug reproduction and its
  fix); the model select lists 70+ real `provider/model` options; the profile select shows
  `Claude (activo)`, `Local`, `current`; changing the profile select persists to
  `agent_settings` (`sqlite3` query showed the row) and survives a full page reload; changing the
  model select and resetting it back to "Por defecto" round-trips correctly across reloads too;
  resetting **both** selectors back to "Por defecto" in the same session (the bug case) now
  succeeds with no errors and leaves the row as `1|||` (all `NULL`). Cleaned up: killed the
  server, deleted `/tmp/pi-workflow-t3-db/`, the throwaway `npm` project, and the smoke scripts.

**Left unverified:** the tooltip/hint text and the toast-on-error path were only checked by code
review (the `title` attribute and the `Effect` wiring), not by forcing an actual server error in
the browser (e.g. by killing the DB mid-session) — doing so felt too disruptive for this pass and
the error-mapping/toast mechanism itself already has unit coverage (`board_page.rs` uses the same
pattern) and integration coverage (`server_fns.rs`'s `to_server_error` mapping test). Genuine
end-to-end exercise of `PrepareLaunch` picking a selected model/profile when actually starting a
task (i.e. running a real task through `gentle-shell`) is also still unverified, same as noted at
the end of T2.

**All T1/T2/T3 tasks are now complete.**

### Nav restyle (2026-09-29)

User feedback: the two selectors didn't fit in the 56px nav bar and crowded the pills.
Restyled to compact pill-style selects.

- `src/core/components/select.rs`: replaced the stacked label + full-width `.input` select
  with a compact `.select-pill` (rounded-full, `min-h-10`, truncated, `appearance-none` with
  a custom chevron). Added `SelectOption { value, label, group: Option<String> }` (constructors
  `new`/`grouped`) and a `group_options` helper that renders consecutive-or-not options under
  the same `group` inside a shared `<optgroup>`, preserving first-appearance order. Added an
  optional `icon: Option<ViewFn>` prop for a leading SVG prefix. The visible label text was
  dropped in favor of an `sr-only` span inside the wrapping `<label>` (implicit a11y
  association) plus a `title` on the wrapper showing the full selected option's label text
  (not the raw encoded value).
- `style/tailwind.css`: new `.select-pill`, `.select-pill-icon`, `.select-pill-input`,
  `.select-pill-chevron`, `.select-pill-skeleton` (loading fallback, same size, no layout
  jump), `.select-pill-divider` (vertical rule, `hidden md:block`) classes, matching the
  existing `.chip`/`.nav-pill` visual language.
- `src/modules/agent_settings/presentation/components/agent_settings_bar.rs`: model options
  now use `SelectOption::grouped(provider, encoded_value, model_id)` (only the model id shows
  as option text; the provider becomes the `<optgroup label>`), so `model_option_label` and its
  test were removed (dead code). Added a small cpu-style SVG icon for the model select and a
  layers-style SVG icon for the profile select (plain `fn() -> impl IntoView`, passed via the
  new `icon` prop). Loading `Transition` fallbacks now render `.select-pill-skeleton` instead of
  plain text. Outer wrapper is `grid grid-cols-2 gap-2 md:flex md:items-center md:gap-1.5` (two
  equal columns on mobile, inline row with a `.select-pill-divider` between the two selects on
  `md`+). Added a small `md:inline-flex`-only info icon carrying the same
  `APPLIES_TO_NEW_TASKS_HINT` as its `title` (the container itself keeps the tooltip too, so it
  still works on mobile where the icon is hidden).
- `src/app.rs`: nav is now `flex flex-wrap ... md:flex-nowrap`, `md:h-14` (fixed height only on
  `md`+; mobile grows with `py-2` padding for the two-row layout). Three `order-*` groups: brand
  (`order-1`), `<AgentSettingsBar />` wrapper (`order-3 w-full` on mobile so it wraps to its own
  full-width row below brand+pills; `md:order-2 md:w-auto` to sit inline between brand and pills
  on `md`+), and the Tablero/Workspaces pills (`order-2` on mobile, `md:order-3`). `AgentSettingsBar`
  is rendered exactly once (its `Resource`s live in a single `AgentSettingsState`); only its CSS
  position changes per breakpoint, so there is no duplicated fetch/state.
- `ToastStack` (`src/core/components/toast.rs`) was already `fixed inset-x-0 bottom-4 ...`
  (unchanged) — confirmed it does not participate in nav layout flow.

**Commands run and results:**
- `cargo fmt --all -- --check` — ok.
- `cargo clippy --features ssr --tests -- -D warnings` — ok, 0 warnings (only an unrelated
  future-incompatibility notice for the `proc-macro-error2` dependency, pre-existing).
- `cargo clippy --features hydrate --target wasm32-unknown-unknown -- -D warnings` — ok, 0
  warnings (same unrelated notice).
- `cargo test --features ssr` — ok, 253 passed total (206 lib unit tests, one fewer than before
  since the dead `model_label_joins_provider_and_model` test was removed with the function it
  tested; every integration suite unaffected), 0 failed.
- `cargo leptos build` — ok (tailwind + wasm-bindgen + ssr binary built cleanly).
- Browser check (Playwright/Chromium headless, reusing an existing throwaway `npm install
  playwright` project under `/tmp/pi-workflow-smoke`): ran the built `target/debug/pi-workflow`
  on `127.0.0.1:3199` with `DATABASE_URL` pointed at a **copy** of `~/.pi-dashboard/workspaces.db`
  under `/tmp/pi-workflow-nav-check/` (the real dashboard db was never opened), against the real
  `gentle-shell --list-models` and `~/.pi/gentle-ai/profiles.json`. Verified at 390px and 1280px
  on both `/` and `/workspaces`: `document.documentElement.scrollWidth <= window.innerWidth` in
  all four cases (no horizontal overflow), zero console/page errors on load. Confirmed the model
  select renders 4 real `<optgroup>`s (one per provider, e.g. `anthropic` with `claude-fable-5`
  as the first plain-model-id option text). Changed the profile selector to `Claude`, reloaded
  the page, and confirmed the select still showed `Claude` selected (persisted via
  `agent_settings` table). Reset both selectors back to "Por defecto" and reloaded, confirming
  both stay empty (the T3 `input = Json` fix for the all-`None` case still holds). Cleaned up:
  killed the server process, deleted `/tmp/pi-workflow-nav-check/` (including the DB copy), and
  the throwaway per-run scripts (kept the reusable `playwright` install in
  `/tmp/pi-workflow-smoke/` since it already existed from a prior session). Did not commit.
  Screenshots saved to `/tmp/pi-workflow-nav-board-390.png`, `/tmp/pi-workflow-nav-board-1280.png`,
  `/tmp/pi-workflow-nav-workspaces-390.png`, `/tmp/pi-workflow-nav-workspaces-1280.png`.

**Left unverified:** the `hydrate`/WASM interactive behavior (option selection dispatching the
`ServerAction`) was exercised through the running server via a real browser (Playwright), which
does execute hydrated JS, so this is covered — but no automated `wasm-bindgen-test`/unit test was
added for the new `select.rs` grouping logic beyond the existing manual Playwright check (the
component has no non-trivial pure logic split out testable without a DOM, unlike
`agent_settings_state.rs`'s codec). Light-color-scheme (`prefers-color-scheme: light`) rendering
of `.select-pill` was not screenshotted (screenshots above are in the default dark theme).
