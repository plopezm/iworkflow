# Go-mode e2e runner

`end2end/go/run_go.sh` runs the browser e2e suite against the Go server (`cmd/iworkflow`) with fake
agents and no model use. It builds the server, starts it on a free port with an isolated `HOME`,
database and worktrees in a temporary directory (never `~/.iworkflow`), starts the Jira and GitLab
mocks, and runs the scripts in Go mode (`IWF_E2E_TARGET=go`).

## Requirements

- `go`, `node`, `python3`, `git`, `curl`.
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
  `git_changes` uses its own repo, `tab_state` its own, `background_jobs` its own, and the rest use the
  terminal repo.
- `SMOKE_WORKTREES`, the server's worktrees directory (same as `IWF_WORKTREES_DIR`). Needed by
  `git_changes_smoke.cjs` to find the agent's worktree.
- `lang_shim.cjs` preloaded, which sets the language cookie the smokes do not set.

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

Full suite: `end2end/go/run_go.sh`. Result: 69 passed, 42 failed (111 tests).

| Script | Pass | Fail | Status |
|---|---|---|---|
| `go/workspaces.e2e.cjs` | 8 | 0 | ok |
| `go/preferences.e2e.cjs` | 7 | 0 | ok |
| `go/agent_settings.e2e.cjs` | 6 | 0 | ok in this run; was intermittent before (see below) |
| `go/agent_profiles.e2e.cjs` | 14 | 0 | ok |
| `agents_files.e2e.cjs` (root, no Go copy) | 8 | 0 | ok |
| `settings.e2e.cjs` (root, no Go copy) | 13 | 0 | ok |
| `terminal.e2e.cjs` (root, no Go copy) | 6 | 0 | ok |
| `go/project_tree.e2e.cjs` | 4 | 5 | 5 fail: the sidebar `Tablero` link is missing (tasks) |
| `go/code_hosting.e2e.cjs` | 3 | 4 | 4 fail: MR/push flow starts from the board (tasks) |
| `jira.e2e.cjs` (root, no Go copy) | 0 | 15 | blocked: jira module not migrated (the settings "Jira" section is a placeholder) |
| `tasks.e2e.cjs` (root, no Go copy) | 0 | 18 | blocked: tasks module not migrated (no board, no task routes) |

Standalone smokes (`IWF_E2E_SCRIPTS`):

| Script | Result | Blocked by |
|---|---|---|
| `git_changes_smoke.cjs` | fails at the sidebar `Tablero` link | tasks (it needs a running agent task) |
| `dock_smoke.cjs` | fails at the sidebar `Tablero` link | tasks (the board) |
| `tab_state_smoke.cjs` | fails at the sidebar `Tablero` link | tasks (the board) |
| `background_jobs_smoke.cjs` | fails at the sidebar `Tablero` link | tasks (the board) |
| `stale_dialog_smoke.cjs` | fails at the sidebar `Tablero` link | tasks (the board) |
| `ui_smoke.cjs` | fails at the sidebar `Tablero` link | tasks (the board) |
| `terminal_keys_smoke.cjs` | passes | — |
| `terminal_reconnect_smoke.cjs` | passes | — |

The `Tablero` failures mean the Go shell does not list the board. The Go shell lists only the migrated
destinations (Workspaces, AGENTS.md, Settings). The board and the tasks screens are in the `tasks` module.

Two notes on the measurements:

- `agent_settings.e2e.cjs` failed once, at the restore step of `update_agent_settings`, on the first
  full run after a server restart. It passed in three isolated re-runs and in the later full runs. The
  cause is not identified; it looks like a race in the bar's htmx swap. Re-run before reading it as a
  regression.
- `workspaces.list_workspaces` (empty-state) needs an empty database. Running the suite twice on the
  same database fails it, which is a precondition issue, not a regression. Use a fresh
  `IWF_DATABASE_URL` (the default `run_go.sh` run already uses a fresh one).

## Open items

- Module-side: the `tasks` and `jira` modules, and the board. They are needed for the 42 failing tests
  and for the 6 standalone smokes listed above.
- Test-side: `end2end/follow_bottom_test.mjs` targets the legacy `src/core` copy and needs its
  expectations updated, or removal, once the Rust copy goes away.
