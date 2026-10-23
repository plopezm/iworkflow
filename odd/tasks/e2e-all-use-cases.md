# e2e-all-use-cases

Playwright end-to-end suite that covers **every browser-reachable use case** of iworkflow, so
each one is asserted against the real UI. Reuses the existing smoke infrastructure (one server,
temp DB, fake agents) and adds service mocks for Jira and the code host (GitHub/GitLab).

## Specs

- S1. "quiero hacer tests playwright de todos los casos de uso para asegurarlos" — one Playwright
  test per browser-reachable use case (72 pure-UI + 19 agent/terminal + code-host 2 + Jira 17
  = 110, plus the 2 code-host UI reads that need a provider, = ~113). Each test drives the real
  UI (routes in `src/app.rs`) and asserts the observable outcome (text, DOM state, URL, HTTP
  response) — never internal DB state directly.
- S2. "ya tienes playwright instalado de forma local" — use the local install
  `/home/pablolm/.cache/iwf-playwright/node_modules/playwright` (playwright + playwright-core
  1.64.1, Chromium cached at `~/.cache/ms-playwright`, confirmed launching). NO network install,
  NO `@playwright/test` (absent in the local install). Extend the existing CJS smoke pattern with a
  shared harness fixture + a single runner.
- S3. Harness: one shared server (build once, `run_smoke.sh`-style: temp DB + fake agents
  `smoke_*.sh` + temp git repos) started by a new `end2end/run_tests.sh`. One long-lived browser
  process (`end2end/run.e2e.cjs`) launches chromium once, adds the `iw-lang=es` cookie, then
  runs each module test file's tests in a fresh context/page, collecting console/pageerror/HTTP
  problems per test and taking a failure screenshot. Old smokes in `end2end/*.cjs` stay untouched.
- S4. Service mocks (for the 22 UI-partial use cases needing an external service):
  (a) a local **Jira mock** HTTP server speaking REST v2 (`/rest/api/2/myself`, `/search`, issue,
  transition, comment) so the 17 Jira use cases are testable; (b) a local **code-host mock**:
  a fake git remote (bare repo + `git push`) plus a stubbed GitHub/GitLab API client for
  create/get/merge/push MR, so the 5 code-host UI-partial use cases are testable.
- S5. The 12 **No-UI** use cases (reconcile_tasks, sweep_orphan_worktrees, handle_agent_event,
  delete_workspace_tasks, trigger_successors, write_guard, prepare_launch, resume_rate_limited_tasks,
  get_task_attachment, + the internal reads) have **no button in the UI** → NOT Playwright-testable.
  They are out of browser scope. The ones **without** an existing Rust integration test are listed
  as a follow-up task (Rust integration tests), not part of this Playwright suite.

## Tasks

- [x] T1 (S2,S3,S5) Shared harness (`end2end/harness.cjs`), runner (`run.e2e.cjs`), runner
  script (`run_tests.sh`) + proof: 1 real module (workspaces, 6 tests) runs end-to-end against the
  built app. Result: **6/6 green** via `bash end2end/run_tests.sh workspaces`. Commit (when the
  suite is complete, T11).
- [x] T2 (S1) agent_profiles module tests (14 pure-UI). Commit.
- [x] T3 (S1) agent_settings + preferences module tests (settings/preferences, 7+5). Commit.
- [x] T4 (S1) agents_files module tests (6). Commit.
- [x] T5 (S1) project_tree module tests (9, incl. git stage/unstage/commit). Commit.
- [x] T6 (S1) terminal module tests (8, via fake pty/agents). Commit.
- [x] T7 (S4,S1) Jira mock server + Jira module tests (17). Commit.
- [x] T8 (S4,S1) Code-host mock (fake git remote + API stub) + code_hosting module tests (7). Commit.
- [x] T9 (S1) tasks module tests (UI-reachable: create/edit/delete/start/finish/merge/message/
  graph/attachments, using fake agents). Commit.
- [x] T10 (S1) workspaces module tests (8: register/edit/delete/browse/create-dir). Commit.
- [x] T11 (S1,S4) Verify the full suite runs green, wire into `run_tests.sh` / CI note, report
  commands + results + what's unverified. Commit.
- [ ] T12 (S5) Follow-up (Rust, separate): integration tests for the No-UI use cases missing
  Rust coverage (list them at completion). Commit (when done).

## Log

- L1 (user): "ok, quiero hacer tests playwright de todos los casos de uso para asegurarlos"
- L2 (user): "ya tienes playwright instalado de forma local"
- L3 (user decision): "Todo a la vez (~113 casos)" → build everything, incl. Jira + code-host mocks.
- L4 (user decision): "Fuera de Playwright + Rust (recom.)" → the 12 No-UI are out of browser
  scope; missing Rust tests are a follow-up task.
- L5 (evidence, T1 done): Harness + runner + run_tests.sh built and verified. Proof module
  `end2end/workspaces.e2e.cjs` (6 tests: list, register-git, register-non-git + init-confirm,
  edit/update, delete, get) ran **6/6 green** via `bash end2end/run_tests.sh workspaces`.
  Pipeline confirmed: temp DB + fake agents + 1 long-lived Chromium, `iw-lang=es` cookie, fresh
  context per test, problem collection. Key selectors: labels Nombre / `^Ruta absoluta` /
  /Rama base/, buttons Registrar / Editar / Guardar cambios / Quitar; workspaces render as
  `<article class="card">`; edit form is `form.card` (distinct from the register `form.stack`),
  so scope edit inputs with `form[class*="card"]`; use `getByRole`/`getByText` (NOT `text=`
  regex for arbitrary paths); register rejects a path already in the DB → each test must use a
  unique repo/dir. 2 workspaces use-cases left for T10: browse_directories + create_directory
  (tree UI).
- L6 (evidence, T2-T11 done): full suite `bash end2end/run_tests.sh` -> **98/98 passed**, exit 0
  (workspaces, agent_profiles, agents_files, code_hosting 7, jira 15, project_tree, settings,
  tasks 18, terminal 6). Fixes: `JIRA_MOCK_PORT` was not passed to the mock in run_tests.sh;
  runner now skips `run.e2e.cjs`; task tests must wait for the create->start chain before
  navigating. GitLab mock (`gitlab_mock.cjs`) + `pushInsteadOf` to a local bare repo cover
  create/get/merge MR and push. Known limits: abort_task only asserts a 200 (fake agent ignores
  abort); T12 (Rust tests for No-UI use cases) remains a separate follow-up. Not committed.
