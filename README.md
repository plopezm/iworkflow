# iworkflow

<p align="center">
  <a href="https://buymeacoffee.com/plopezm" target="_blank" rel="noopener">
    <img src="https://img.buymeacoffee.com/button-api/?text=Buy%20me%20a%20coffee&emoji=%E2%98%95&slug=plopezm&button_colour=FFDD00&font_colour=000000&font_family=Cookie&outline_colour=000000&coffee_colour=ffffff" alt="Buy me a coffee" height="50" />
  </a>
</p>

> 🇪🇸 Versión en castellano: [README.es.md](README.es.md)

Web dashboard to launch and control `gentle-shell` (pi) instances in RPC mode.
Each task runs in its own **git worktree**; when it finishes you review the diff and decide whether to integrate it.

The app imposes no workflow: each instance uses the gentle-shell configuration and plugins
you have installed, plus the project's `AGENTS.md`.

## Quick start

```bash
cargo leptos watch          # http://127.0.0.1:3000
```

1. **Workspaces** → register a folder with git (it needs at least one commit).
2. **Tasks** → pick a workspace, write the prompt and launch.
3. In the task detail you follow the agent live and can:
   - send messages (a prompt if it is waiting; *steer* or *follow-up* if it is working),
   - answer agent dialogs (`ask_user_question`, confirmations…),
   - abort the current run.
4. **Finish and review** → closes the agent, commits whatever is pending and shows the diff.
5. **Merge** (integrates with `--no-ff` and deletes the worktree) or **Discard**.

## Tabbed workspace

The interface is an IDE-like workspace: every screen (board, task, diff, terminal, settings, files…) opens as a **tab** inside groups you can split by dragging. Tabs can be reordered by dragging them within their bar, moved to another group or dropped into a new split.

- **File explorer and editor** (with syntax highlighting) over a workspace or over a task's worktree. In the task panel: **Review → Explore files**.
- Modified, added, untracked, deleted or conflicted files and folders are colored according to `git status`.
- While you edit, the left gutter marks added, modified and deleted lines against `HEAD` (as in VS Code).
- When **editing a task**, the CPU and memory limits live under *Advanced options*.

## Task lifecycle

```text
starting → running ⇄ idle → finishing → awaiting_review → merged
                                              │               ↘ discarded
                                              ↓
              conflict · failed · interrupted ── Resume (gentle-shell --continue)
```

| Situation | What the app does |
|---|---|
| Conflict while integrating | `merge --abort`; the task moves to `conflict` and keeps its worktree. Resume the agent to resolve it. |
| Base branch with uncommitted changes | Rejects the merge without touching anything. |
| Base branch not checked out | Integrates in a temporary worktree. |
| Server restart | Active tasks move to `interrupted`; they can be resumed. |

## Configuration

Environment variables (`.env` is supported, see `.env.example`):

| Variable | Default |
|---|---|
| `IWF_DATABASE_URL` | `~/.iworkflow/workspaces.db` (SQLite, automatic migrations) |
| `IWF_WORKTREES_DIR` | `~/.iworkflow/worktrees` |
| `IWF_GENTLE_SHELL_BIN` | `gentle-shell` (launched as `--mode rpc` with `GENTLE_SHELL_INTERACTIVE_HOST=1`) |
| `IWF_PI_BIN` | `pi` (`pi` runtime, launched as `--mode rpc`) |
| `IWF_CLAUDE_BIN` | `claude` (`claude` runtime: Claude Code with `-p` and the stream-json protocol; chosen in the runtime selector of the side menu; accepts file edits, asks for confirmation for the rest and reads the project's `AGENTS.md`) |
| `IWF_CURSOR_BIN` | `agent` (`cursor` runtime: Cursor CLI in ACP server mode, `agent acp`; requires having signed in with `agent login`) |
| `IWF_CURSOR_SESSIONS_DIR` | `cursor-sessions` next to the database (stores each task's ACP session id so it can be resumed) |
| `IWF_ATTACHMENTS_DIR` | `attachments` next to the database (images attached to tasks; deleted when the task ends) |
| `IWF_AGENTS_FILES_DIR` | `agents-files` next to the database (`AGENTS.md` library) |
| `IWF_GENTLE_PI_CONFIG_HOME` | `~/.pi/gentle-ai` (contains `profiles.json` and `models.json`) |
| `IWF_PI_AGENTS_DIR` | `~/.pi/agent/agents` (one `<role>.md` per agent role) |

The default data directories (`attachments`, `agents-files`, `cursor-sessions`) hang off the database folder: if you change `IWF_DATABASE_URL`, they move with it. On Windows the home directory is taken from `USERPROFILE` when `HOME` is missing.

### Agent runtimes

The runtime selector in the side menu supports `gentle-shell`, `pi`, `claude` and `cursor`. The choice is fixed when each task starts.

| Runtime | Binary | Capabilities |
|---|---|---|
| `gentle-shell` / `pi` | `IWF_GENTLE_SHELL_BIN` / `IWF_PI_BIN` | all (steer, follow-up, dialogs, resume, model, thinking, subagents) |
| `claude` | `IWF_CLAUDE_BIN` | dialogs, resume, model and thinking |
| `cursor` | `IWF_CURSOR_BIN` | dialogs, resume and model (the model list is obtained from the agent itself over ACP). File edits are accepted; everything else asks for confirmation |

## GitLab and GitHub integration

From each task you can create a merge request (GitLab) or pull request (GitHub), watch its CI and merge it. A workspace can be a repository on either: the provider is detected from the host of the `origin` remote and that provider's configuration is applied.

1. **Settings** has one card per provider.
   - **GitLab**: your instance URL (e.g. `https://gitlab.mycompany.local`) and a personal token with the `api` scope.
   - **GitHub**: `https://github.com` (proposed by default) or your GitHub Enterprise URL, and a personal token (`repo` scope; fine-grained: Pull requests and Contents read/write, Actions read).
   Use "Test connection" to validate them. If your certificate comes from an internal CA or is self-signed, tick "Accept untrusted TLS certificate".
2. The project is derived from the `origin` remote (https, ssh or scp). Its host must match the URL of a configured provider; if it matches none you are warned, and if it is github.com and GitHub is not configured you are asked to configure it. On GitHub the repository is `owner/repository`.
3. In the task panel, **PR / MR** tab: create the MR/PR (task branch → base branch), see its status and the latest pipeline (GitLab) or workflow (GitHub) with its jobs, and **Merge** (asks for confirmation).

Notes:
- The app **never runs `git push`**: publish the branch yourself before creating the MR; if it is not on `origin`, you are warned.
- The token is stored in clear text in the local database (`~/.iworkflow/workspaces.db`) and is never sent to the browser. Leaving it empty when editing keeps the stored one, unless you change the URL: then it has to be entered again, so it is not sent to a different server.
- HTTP redirects are disabled so the token does not leak to another host.

## Jira integration (Server / Data Center)

The **Jira** tab connects to an on-premise Jira using the server URL and a personal access token (PAT, `Authorization: Bearer`, REST API v2). On save, the connection is validated against Jira; the token is stored in the local database and never returns to the browser (leaving it empty when editing keeps the stored one).

- **Import**: "Sync" fetches the issues matching the connection's JQL (default `assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC`, at most 500). The local state (new / converted / ignored) is preserved across syncs; new issues Jira no longer returns are removed.
- **Manage**: list with search and filters, detail with description and comments, and writing back to Jira: change the status (workflow transitions) and comment. Ignore/restore is local only.
- **Convert**: the "Convert to task" button opens a form with the title, prompt and priority proposed from the Jira issue (priority is mapped from the Jira name); you choose the workspace and base branch, edit whatever you want and the task is created (optionally started). The Jira issue is marked as converted and linked.
- If the server uses a self-signed certificate or one from an internal CA the system does not know, tick "Accept unverified certificate" in the connection.

## Agent profiles and models

Two screens equivalent to the gentle-pi commands, working on the same files:

| Screen | Equivalent to | What it does |
|---|---|---|
| `/profiles` | `/gentle:profiles` | Create, duplicate, rename, delete, import and export profiles (`profiles.json`), edit model and effort per agent and create new agent roles. |
| `/models` | `/gentle:models` | Edit the global routing (`models.json`): model and effort per agent. |

Profiles are chosen in the agent settings dropdown, which reads the same `profiles.json`. These screens do not activate a profile globally; they only manage them. The write lock prevents clobbering within one process, not against another process (e.g. the gentle-pi TUI) editing at the same time.

In development (`cargo leptos watch`) the server listens only on `127.0.0.1`.

### Executable arguments

On startup, `iworkflow` opens the web UI in the browser (`http://<host>:<port>` of the server; if it listens on `0.0.0.0`, `127.0.0.1` is used).

| Argument | Effect |
|---|---|
| `--no-open` | Starts only the server, without opening the browser. |
| `-h`, `--help` | Shows the help. |

Examples: `./iworkflow --no-open`, `./scripts/prod.sh run --no-open`, `cargo leptos watch -- --no-open`.

## Production

```bash
./scripts/prod.sh           # builds in release, packages into dist/ and starts
./scripts/prod.sh build     # build and package only
./scripts/prod.sh run       # start what is already packaged
```

`build` runs `cargo leptos build --release` and copies the `iworkflow` binary and the
`site/` folder (WASM, JS and CSS) into `dist/`. Both are required: without
`site/` the page is served but does not hydrate.

`run` exports the Leptos variables (`LEPTOS_SITE_ROOT`, `LEPTOS_ENV=PROD`…) and
starts the binary from the current directory, so the `.env` in that directory
is applied too. Variables specific to the script:

| Variable | Default |
|---|---|
| `DIST_DIR` | `dist/` at the repo root |
| `LEPTOS_SITE_ADDR` | `0.0.0.0:3000` (all interfaces) |

> **Security:** the app has no authentication and launches agents that run commands
> as your user. With the bind on `0.0.0.0`, anyone who can reach the port can use it.
> Restrict access with a firewall, a private network (Tailscale/WireGuard) or a
> reverse proxy with authentication and TLS. To limit it to the local machine:
> `LEPTOS_SITE_ADDR=127.0.0.1:3000 ./scripts/prod.sh run`.

To deploy on another machine, just copy `dist/` and launch it with
`LEPTOS_SITE_ROOT=site LEPTOS_ENV=PROD LEPTOS_SITE_ADDR=0.0.0.0:3000 ./iworkflow`
from inside that folder.

## Prebuilt binaries (GitHub Actions)

The `.github/workflows/release.yml` workflow builds on Linux (`ubuntu-22.04`) and on
Windows (`windows-latest`) with the same steps as `scripts/prod.sh build`
(`cargo leptos build --release` + binary and `site/`).

- **When it runs:** when pushing a `v*` tag, manually (*Run workflow*) and on PRs that
  modify the workflow itself.
- **Where the binaries end up:** as run artifacts
  (`iworkflow-linux-x86_64`, `iworkflow-windows-x86_64`). With a `v*` tag they are also
  published to the GitHub Release (`.tar.gz` on Linux, `.zip` on Windows).

```bash
git tag v0.5.0 && git push origin v0.5.0
```

Each archive contains `iworkflow` (or `iworkflow.exe`) and the `site/` folder, which must
stay together. To start it, from the extracted folder:

```bash
# Linux
LEPTOS_OUTPUT_NAME=iworkflow LEPTOS_SITE_ROOT=site LEPTOS_ENV=PROD \
  LEPTOS_SITE_ADDR=127.0.0.1:3000 ./iworkflow
```

```powershell
# Windows (PowerShell)
$env:LEPTOS_OUTPUT_NAME="iworkflow"; $env:LEPTOS_SITE_ROOT="site"; $env:LEPTOS_ENV="PROD"
$env:LEPTOS_SITE_ADDR="127.0.0.1:3000"; .\iworkflow.exe
```

**What is self-contained:** SQLite is bundled in both binaries and the Windows one links
the MSVC runtime statically (no VC++ Redistributable required). The Linux one uses
dynamic glibc; building on Ubuntu 22.04 keeps the minimum requirement low.

**Limitations:**

- It is not a single file: web assets are served from `site/` on disk. A single
  executable would require embedding them in the binary (e.g. with `rust-embed`).
- Windows (native): the terminal and agents are launched with `powershell.exe` (no login
  shell or `exec`; the system `PATH` is inherited) and on close the process tree is
  killed with `taskkill /T`. The home directory is taken from `USERPROFILE` when `HOME` is
  missing. Per-task RAM/CPU limits (systemd) exist only on Linux. The support was written
  and tested with logic tests on Linux, but **has not yet been run on a real Windows**:
  the agents (`gentle-shell`, `pi`, `claude`) must be on PowerShell's `PATH`.

## Development

Architecture and code guide in [ARCHITECTURE.md](ARCHITECTURE.md); project rules in [AGENTS.md](AGENTS.md). Checks:

```bash
cargo fmt --all -- --check
cargo clippy --features ssr --all-targets -- -D warnings
cargo clippy --features hydrate --target wasm32-unknown-unknown --lib -- -D warnings
cargo test --features ssr
PLAYWRIGHT_MODULE=/path/to/node_modules/playwright ./end2end/run_smoke.sh   # UI with a fake agent
```

## Support the project

<p align="center">
  <a href="https://buymeacoffee.com/plopezm" target="_blank" rel="noopener">
    <img src="https://img.buymeacoffee.com/button-api/?text=Buy%20me%20a%20coffee&emoji=%E2%98%95&slug=plopezm&button_colour=FFDD00&font_colour=000000&font_family=Cookie&outline_colour=000000&coffee_colour=ffffff" alt="Buy me a coffee" height="60" />
  </a>
</p>

If iworkflow saves you time, you can buy me a coffee at **[buymeacoffee.com/plopezm](https://buymeacoffee.com/plopezm)**. Thank you! ☕
