# Integrations

> 🇪🇸 [Versión en castellano](es/integrations.md) · [← README](../README.md)

## GitLab and GitHub integration

From each task you can create a merge request (GitLab) or pull request (GitHub), watch its CI and merge it. A workspace can be a repository on either: the provider is detected from the host of the `origin` remote and that provider's configuration is applied.

1. **Settings** has one card per provider.
   - **GitLab**: your instance URL (e.g. `https://gitlab.mycompany.local`) and a personal token with the `api` scope.
   - **GitHub**: `https://github.com` (proposed by default) or your GitHub Enterprise URL, and a personal token (`repo` scope; fine-grained: Pull requests and Contents read/write, Actions read).
   Use "Test connection" to validate them. If your certificate comes from an internal CA or is self-signed, tick "Accept untrusted TLS certificate".
2. The project is derived from the `origin` remote (https, ssh or scp). Its host must match the URL of a configured provider; if it matches none you are warned, and if it is github.com and GitHub is not configured you are asked to configure it. On GitHub the repository is `owner/repository`.
3. In the task panel, **PR / MR** tab: create the MR/PR (task branch → base branch), see its status and the latest pipeline (GitLab) or workflow (GitHub) with its jobs, and **Merge** (asks for confirmation).

Notes:
- Creating the MR/PR publishes the task branch to `origin` with `git push` if it is not there yet. The push goes to `<provider URL>/<project>.git` over HTTPS and authenticates with the token configured here (HTTP Basic, user `x-access-token` on GitHub and `oauth2` on GitLab), so it works even if `origin` is an SSH remote and needs no git credentials of your own. The token is handed to git through the process environment (never the command line), only for that instance, and any credential helper is ignored.
- The target is the task's base branch **as named on the server**: a remote reference such as `origin/develop` is sent as `develop`. If that branch is not on `origin`, or it is the task's own branch, creation is rejected with an explicit message instead of the provider's raw error.
- The token is stored in clear text in the local database (`workspaces.db` in the data folder) and is never sent to the browser. Leaving it empty when editing keeps the stored one, unless you change the URL: then it has to be entered again, so it is not sent to a different server.
- HTTP redirects are disabled so the token does not leak to another host.

## Jira integration (Server / Data Center)

The **Jira** tab connects to an on-premise Jira using the server URL and a personal access token (PAT, `Authorization: Bearer`, REST API v2). On save, the connection is validated against Jira; the token is stored in the local database and never returns to the browser (leaving it empty when editing keeps the stored one).

- **Import**: "Sync" fetches the issues matching the connection's JQL (default `assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC`, at most 500). The local state (new / converted / ignored) is preserved across syncs; new issues Jira no longer returns are removed.
- **Manage**: list with search and filters, detail with description and comments, and writing back to Jira: change the status (workflow transitions) and comment. Ignore/restore is local only.
- **Convert**: the "Convert to task" button opens a form with the title, prompt and priority proposed from the Jira issue (priority is mapped from the Jira name); you choose the workspace and base branch, edit whatever you want and the task is created (optionally started). The Jira issue is marked as converted and linked.
- If the server uses a self-signed certificate or one from an internal CA the system does not know, tick "Accept unverified certificate" in the connection.

## Agent profiles and models

This feature is the integration with [Gentle AI](https://github.com/Gentleman-Programming/gentle-ai), the agent harness behind `gentle-shell` and its gentle-pi package. Profiles and per-agent model routing are Gentle AI concepts: iworkflow does not define its own, it reads and writes the same configuration files (`profiles.json`, `models.json`), so changes made here are seen by the Gentle AI TUI and vice versa. Install and configure Gentle AI first; without it these screens have nothing to manage.

Two screens equivalent to the gentle-pi commands, working on the same files:

| Screen | Equivalent to | What it does |
|---|---|---|
| `/profiles` | `/gentle:profiles` | Create, duplicate, rename, delete, import and export profiles (`profiles.json`), edit model and effort per agent and create new agent roles. |
| `/models` | `/gentle:models` | Edit the global routing (`models.json`): model and effort per agent. |

Profiles are chosen in the agent settings dropdown, which reads the same `profiles.json`. These screens do not activate a profile globally; they only manage them. The write lock prevents clobbering within one process, not against another process (e.g. the gentle-pi TUI) editing at the same time.

### Engram memory

Off by default. With `IWF_ENGRAM_URL=http://127.0.0.1:7437` (or the address of your Engram), iworkflow shows, for each workspace, how many memories Engram keeps for its project (the chip on Workspaces), and, on each task, the type, title and date of its five most recent memories (the Memory panel). The access is read-only: iworkflow only sends `GET` requests (`/projects` and `/observations/recent`) and never saves, edits or deletes memories. The content of a memory is never shown, only its type, title and date. When Engram does not answer, the screens say it is not available and keep working. Without the variable, or with an invalid URL, the feature is off and iworkflow sends no request to Engram.
