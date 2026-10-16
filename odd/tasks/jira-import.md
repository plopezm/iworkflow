# jira-import

Connect to an on-premise Jira (Server/Data Center) with URL + user token, import tasks, manage them
from iworkflow (incl. writing back to Jira) and convert one into an iworkflow task through an
editable mapping. Keep Spanish UI copy; code/comments in English.

## Specs

- S1. "Para conectar se requiere la URL y un token de usuario" — one connection (singleton), Bearer
  Personal Access Token (`Authorization: Bearer`), REST API v2. Saving validates the connection
  (`/rest/api/2/myself`) and never returns the token to the client (blank token on edit keeps it).
- S2. "Esas tareas se importan" — manual sync with a button; default JQL
  `assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC`, editable in the
  connection. Imported issues are stored locally; local state (new/converted/ignored) survives re-syncs.
- S3. "permite gestionarlas desde esta app" — list with search/status/local-state filters, detail
  (description, comments), transition status, add comment (written to Jira), ignore/restore, link to Jira.
- S4. "convertir una tarea de jira en una tarea de iworkflow con un botón" — button on list and detail.
- S5. "Me permitirá mapearla y editar el prompt antes de crear finalmente la task de iworkflow" —
  conversion page prefilled from the issue (title, prompt, priority hint from Jira priority) where the
  user picks workspace/base branch/priority, edits title and prompt, then creates the task; the issue
  is marked converted and linked to the task.
- S6. Architecture: new `jira` vertical slice; cross-feature access (create task, workspaces/branches)
  only through outbound ports bridged in `app_state.rs`.

## Tasks

- [x] T1 (S1,S2,S5,S6) domain models, errors, ports, mocks, prompt builder + priority hint — commit
- [x] T2 (S1,S2) migration + schema + Diesel adapters (connection, issues) + integration tests — commit
- [x] T3 (S1,S3) Jira REST client adapter (reqwest) + integration test against a stub server — commit
- [x] T4 (S1-S5) use cases with mockall tests — commit
- [ ] T5 (S1-S6) DI context, DTOs, server fns, app_state bridges + mapping tests — commit
- [ ] T6 (S1-S5) presentation (state, pages, components), dock routes, sidebar entry — commit
- [ ] T7 (all) fmt, clippy ssr/hydrate, tests, docs, codegraph — commit

## Log

- L1 (user): "Quiero conectar a un server de jira on premise. Esas tareas se importan y permite gestionarlas desde esta app. Luego, puedo convertir una tarea de jira en una tarea de iworkflow con un botón. Me permitirá mapearla y editar el prompt antes de crear finalmente la task de iworkflow. Para conectar se requiere la URL y un token de usuario"
- L2 (user decisions): una conexión, Bearer PAT; mis tareas abiertas + JQL editable; "gestionar" = además escribir en Jira (transiciones y comentarios).
