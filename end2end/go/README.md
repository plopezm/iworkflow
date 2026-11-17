# Go-mode e2e runner

`end2end/go/run_go.sh` runs the browser e2e suite against the Go server (`cmd/iworkflow`) with fake
agents and no model use. It builds the server, starts it on a free port with an isolated `HOME`,
database and worktrees in a temporary directory (never `~/.iworkflow`), starts the Jira and GitLab
mocks, and runs the scripts in Go mode (`IWF_E2E_TARGET=go`).

## Requirements

- `go`, `node`, `python3`, `git`, `curl`.
- `tailwindcss` (`make tools`). `web/static/app.css` is a git-ignored build artifact; the runner builds it when it is
  missing and stops with a clear message when `tailwindcss` is not installed. Without the CSS no Tailwind class applies
  (hidden dock panels stay visible), so board tests fail with "strict mode violation" on `board-list`.
- Playwright and a browser already installed. Nothing is installed by the runner.
  - `PLAYWRIGHT_MODULE` defaults to `/home/pablolm/.cache/iwf-playwright/node_modules/playwright`.
  - `PLAYWRIGHT_BROWSERS_PATH` defaults to `~/.cache/ms-playwright`.
  - `SMOKE_CHROMIUM_PATH` is optional, for an explicit Chromium binary.

## Commands

| What | Command |
|---|---|
| Whole Go suite | `end2end/go/run_go.sh` |
| One module (name substring) | `end2end/go/run_go.sh workspaces` |
| Standalone smoke scripts, one server | `IWF_E2E_SCRIPTS="end2end/dock_smoke.cjs end2end/tab_state_smoke.cjs" end2end/go/run_go.sh` |
| Node unit steps first (no server needed) | `IWF_E2E_UNIT=1 end2end/go/run_go.sh ...` |
| Unit steps only, without a server | `node --test web/static/js/` |
| Terminal keys and touch scroll unit test (29 tests, no server) | `node --test end2end/terminal_keys_test.mjs` |
| Legacy follow-bottom test under Node | `node --import ./end2end/go/dom_shim.mjs --test end2end/follow_bottom_test.mjs` |

Module filters match the module name or file name. `workspaces` runs `go/workspaces.e2e.cjs` only.

In Go mode the runner uses `end2end/go/<module>.e2e.cjs` when it exists. A root module with the same
name is replaced. Root modules without a Go copy still run, so their failures show the gaps.
`settings`, `terminal`, `tasks`, `jira` and `agents_files` are root modules.

## Run directory and logs

Every run keeps its evidence in a run directory, printed on exit as `▶ run logs kept in: <dir>`:

- `runner.log`: the runner or smoke output (the same text shown on the terminal).
- `server.log`: the server's stdout and stderr.
- `jira_mock.log`, `gitlab_mock.log`: the mocks.

The default is `/tmp/iwf-e2e-go-runs/<timestamp>-<pid>`. Set `IWF_E2E_RUN_DIR` to choose another
directory. Logs are copied before the scratch directory is removed, so failing runs keep them.

The runner starts the server with `IWF_LOG_LEVEL=debug`, so `server.log` has one `request` line per
HTTP request (method, path, status, duration; the query string is left out because it can carry the
access token). Without that variable the server logs at info level and prints no request lines.

## Standalone smokes

The smokes in `end2end/*_smoke.cjs` run one by one with `IWF_E2E_SCRIPTS`. The runner sets:

- `SMOKE_BASE_URL`, the server URL.
- `SMOKE_REPO`, a git repo that the script registers as a workspace. It depends on the script:
  `git_changes`, `tab_state` and `background_jobs` each use their own repo, `ui_smoke`, `stale_dialog`,
  `terminal_keys` and `terminal_reconnect` each have one too (the server refuses a second workspace on the same
  root), and the rest use the terminal repo.
- `SMOKE_WORKTREES`, the server's worktrees directory (`worktrees` under the server's `IWF_HOME`). Needed by
  `git_changes_smoke.cjs` to find the agent's worktree.
- `lang_shim.cjs` preloaded, which sets the language cookie the smokes do not set.

## Fake Engram

`run_go.sh` reserves a port for a fake Engram API and starts the server with `IWF_ENGRAM_URL` pointing at it
(the server runs under `env -i`, so the variable is passed explicitly). It also exports the same URL to the suites as
`IWF_E2E_ENGRAM_URL`. The `engram` suite starts the fake on that port; every other suite sees Engram as not available,
which is what they would see with the default address.

- `redesign.e2e.cjs`: PWA head (manifest and icons), sidebar header and current page, board of a
  workspace with two seeded tasks, status badges. Run it alone with `end2end/go/run_go.sh redesign`.
- `engram.e2e.cjs`: the Engram chip of the Workspaces screen and the collapsed Memory panel of a task, against the
  fake. The fake records every request, and the suite checks that only GET reached it. Run it alone with
  `end2end/go/run_go.sh engram`. Its tests run in order: the last one stops the fake.

## Usage bar (usage_limits)

`end2end/go/usage_limits.e2e.cjs` covers the usage bar in the sidebar footer. Run it alone with
`end2end/go/run_go.sh usage_limits`.

- `end2end/usage_mock.cjs`: fake Claude usage API on `USAGE_MOCK_PORT` (`GET /api/oauth/usage`, `/_health`). It
  answers 401 unless the request carries `Authorization: Bearer <token>` and `anthropic-beta: oauth-2025-04-20`.
  It returns Claude 13 % (5 h, resets in about 2 h 10 min) and 25 % (week, resets in 3 days).
- The runner writes a fake `$WORK/home/.claude/.credentials.json` (mode 600) and a Codex rollout under
  `$WORK/home/.codex/sessions/2026/11/15/rollout-e2e.jsonl` with 75 % (5 h) and 95 % (week). The server reads them
  through `IWF_CLAUDE_CREDENTIALS`, `IWF_CLAUDE_USAGE_URL` (pointing at the mock) and `IWF_CODEX_SESSIONS_DIR`.
- `SMOKE_SCREENSHOT_DIR` is set by the runner to `<run dir>/screenshots` (default `/tmp/iwf-e2e-screenshots` when
  the suite runs without the runner). The suite saves `usage-bar.png` and `usage-sidebar-full.png` there and prints
  their absolute paths.

## Node unit steps

`IWF_E2E_UNIT=1` runs two Node steps before the server starts:

- `node --test web/static/js/`. This is the gate: a failure stops the run. It covers the browser helpers
  of the Go module, including `follow_bottom.js` (7 tests). These tests need no shim.
- `end2end/follow_bottom_test.mjs` with `end2end/go/dom_shim.mjs`. This is informational: a failure is
  reported but does not stop the run.

`dom_shim.mjs` adds an inert `MutationObserver` when the global is missing. It never fires the callbacks,
so tests that depend on mutations must trigger them explicitly. It does not change a real global.

`end2end/follow_bottom_test.mjs` targets the legacy `src/core/follow_bottom.js` (`pi_*` names), not the
Go module. With the shim it runs 5/7. The two failures are expectation differences, not DOM gaps: the
test calls `pi_follow_bottom(id)` without the `auto` argument, and its assertions for "stuck" and
"does not move" depend on that argument. With `auto=true` in every call, two other tests fail the other
way. The Go module's equivalent (`web/static/js/follow_bottom_test.mjs`) passes 7/7 without the shim.

## Status of the scripts (measured on the current tree)

Full suite (`end2end/go/run_go.sh`), per module, first run of the audit (before) and the last full run (after):

| Module | Before (pass/fail) | After (pass/fail) | What was wrong |
|---|---|---|---|
| `workspaces` | 1 / 7 | 8 / 0 | Registration moved to `/workspaces/new`; the `+` button and `#modal-panel` locators |
| `agents_files` 4/4 → 8/0, `board_live_sidebar` 0/1 → 1/0, `board_refresh` 3/1 → 4/0, `board_tabs` 0/1 → 1/0, `diff_report` 0/1 → 1/0, `git_changes` 0/1 → 1/0, `jira` 12/4 → 16/0, `project_tree` 0/9 → 9/0, `stale_dialog` 0/4 → 4/0, `submit_progress` 0/3 → 3/0, `task_attachments_lightbox` 0/2 → 2/0, `task_composer` 0/5 → 5/0, `task_form_paste` 0/3 → 3/0, `task_hooks` 5/4 → 9/0, `task_hooks_strip` 0/4 → 4/0, `terminal` 1/5 → 6/0, `terminal_shell` 5/1 → 6/0, `transcription` 0/2 → 2/0, `usage_limits` 4/1 → 5/0 | (listed) | (listed) | Same registration route: the helpers opened `/workspaces` and filled a form that now lives at `/workspaces/new` |
| `redesign` | 7 / 1 | 7 / 1 (8 / 0 when re-run alone) | Backlog count read before the second card arrived (race, fixed by waiting for the card) |
| `engram`, `related_projects` | 14 / 0, 5 / 0 | unchanged | The `+` click stays on `/workspaces` |

Last full run: 231 passed, 1 failed (`redesign.tablero`, the race above). That module was then re-run alone and passed 8/8. A clean full re-run after that fix was not done.

Standalone smokes (`IWF_E2E_SCRIPTS`), one server, fresh database, last run:

| Script | Result | Note |
|---|---|---|
| `dock_smoke.cjs` | passes | The `Modelos` tab is now `Configuración` (`/settings`) |
| `tab_state_smoke.cjs` | passes | Registration form reopened with `+` after registering |
| `stale_dialog_smoke.cjs` | passes | |
| `terminal_keys_smoke.cjs` | passes | |
| `terminal_reconnect_smoke.cjs` | passes | |
| `touch_targets_smoke.cjs` | passes | Needs a running server (`SMOKE_BASE_URL`); it is not run by the runner's default suite |
| `git_changes_smoke.cjs` | intermittent | Passed in one run; failed in the last run at the first Cambios poll. `go/git_changes.e2e.cjs` passes in both full runs |
| `ui_smoke.cjs` | fails at "la pestaña Archivos se reconstruye al cambiar" | Earlier steps pass. Not resolved (see open items) |
| `background_jobs_smoke.cjs` | fails | Product gap (see open items) |

The Go module is what the full suite uses. The root modules that the Go suite does not replace (`agents_files`, `settings`,
`terminal`, `jira`) run in it and pass. The root copies of `workspaces`, `tasks`, `project_tree`, `related_projects` and
`code_hosting` are replaced by their `go/` copies in Go mode, so they only run without `IWF_E2E_TARGET=go`. Their
registration and route fixes were made, but they were not run.

Two notes on the measurements:

- `agent_settings.e2e.cjs` failed once, at the restore step of `update_agent_settings`, on the first
  full run after a server restart. It passed in three isolated re-runs and in the later full runs. The
  cause is not identified; it looks like a race in the bar's htmx swap. Re-run before reading it as a
  regression.
- `workspaces.list_workspaces` (empty-state) needs an empty database. Running the suite twice on the
  same database fails it, which is a precondition issue, not a regression. Use a fresh
  `IWF_DATABASE_URL` (the default `run_go.sh` run already uses a fresh one).

## Open items

Product (not fixed here; the tests describe the expected behaviour):

- Background jobs: the Go attention rule ignores them. `StageAt` in `internal/modules/tasks/domain/models/task.go`
  counts an idle task as "Requiere atención" unless subagents are working, so a task waiting on a background job
  is counted. `background_jobs_smoke.cjs` asserts the opposite, as the Rust app did.
- Task Archivos tab: `ui_smoke.cjs` expects the Archivos pane content to survive a tab switch. The check fails
  after the switch; this has not been investigated in the DOM.
- Duplicate ids: `internal/core/components/modal.templ` hardcodes `id="modal-panel"` for every `Modal`, and the
  shell renders one more on every page (the chat image modal). `#modal-panel` therefore matches two elements. The
  workspace tests now look for the visible `[data-modal-panel]` instead.
- Commit button: the Go button is never disabled; the message field is `required` (known difference, see the
  go-migration notes).

Test side:

- `end2end/follow_bottom_test.mjs` targets the legacy `src/core` copy and needs its expectations updated, or removal,
  once the Rust copy goes away. It is referenced by `run_go.sh` and this README, so it is kept.
- `move_home_smoke.sh` and `related_projects_real.sh` are not run by the suite and were not run in this change.
