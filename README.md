# iworkflow

<p align="center">
  <a href="https://buymeacoffee.com/plopezm" target="_blank" rel="noopener">
    <img src="https://img.buymeacoffee.com/button-api/?text=Buy%20me%20a%20coffee&emoji=%E2%98%95&slug=plopezm&button_colour=FFDD00&font_colour=000000&font_family=Cookie&outline_colour=000000&coffee_colour=ffffff" alt="Buy me a coffee" height="50" />
  </a>
</p>

> 🇪🇸 Versión en castellano: [README.es.md](README.es.md)

**iworkflow is an ADE (Agentic Development Environment)**: an AI-powered development environment conceived as an evolution of the traditional IDE. Instead of writing every line yourself, you orchestrate autonomous AI agents that act as active collaborators. The main workflow stops being file editing and becomes **task management**: you launch, coordinate and review the work of several agents at once.

- **Agent orchestration**: one unified surface to launch, follow and steer several agents in parallel (`gentle-shell`, `pi`, `claude`, `cursor`).
- **Isolated workspaces**: each task runs in its own **git worktree**, so parallel agents never overwrite each other's work.
- **Review, don't redo**: when an agent finishes you review its diff and decide whether to integrate it (`merge --no-ff`) or discard it. Agents can run tests, and tasks connect to GitLab/GitHub (MR/PR and CI) and Jira.
- **Developer role shift**: from "doing everything" to **orchestrating and reviewing** what the agents execute.

The app imposes no workflow: each agent uses the configuration and plugins you have installed, plus the project's `AGENTS.md`. Under the hood, agents are driven over RPC (or ACP / stream-json, depending on the runtime).

## Index

- [Designed for mobile](#designed-for-mobile)
- [Quick start](#quick-start)
- [Tabbed workspace](#tabbed-workspace)
- [Workspace board](#workspace-board)
- [Task lifecycle](#task-lifecycle)
- [GitLab and GitHub integration](#gitlab-and-github-integration)
- [Jira integration (Server / Data Center)](#jira-integration-server--data-center)
- [Agent profiles and models](#agent-profiles-and-models)
- [Configuration](#configuration)
  - [Agent runtimes](#agent-runtimes)
  - [Executable arguments](#executable-arguments)
- [Production](#production)
- [Developing from your phone with Tailscale](#developing-from-your-phone-with-tailscale)
- [Development](#development)
- [License](#license)
- [Support the project](#support-the-project)

## Designed for mobile

iworkflow is built **mobile-first**. Since your job is to launch, follow and review agents (not to type code), it works well from a phone:

- The UI is responsive: on small screens the side menu becomes a drawer opened from a top bar, and the tabbed workspace adapts to the viewport.
- The agent keeps running on your machine while you are away. From the phone you check progress, answer agent questions, send a message, review the diff and merge or discard.
- On iOS (Safari → Share → *Add to Home Screen*) and Android (Chrome → menu → *Add to Home screen*) you can pin it as a shortcut. For now it opens in the browser; it does not yet ship a web app manifest, so it is not a standalone full-screen app.

The server runs on your computer and the phone is only a browser, so you need a secure way to reach it: see [Developing from your phone with Tailscale](#developing-from-your-phone-with-tailscale).

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

> **Security:** iworkflow has no authentication and launches agents that run commands as your user. Do not expose it to untrusted networks; see [Production](#production) and [Tailscale](#developing-from-your-phone-with-tailscale). Development listens on `127.0.0.1:3000`; the production script on `0.0.0.0:9000`.

## Tabbed workspace

The interface is an IDE-like workspace: every screen (board, task, diff, terminal, settings, files…) opens as a **tab** inside groups you can split by dragging. Tabs can be reordered by dragging them within their bar, moved to another group or dropped into a new split.

- **File explorer and editor** (with syntax highlighting) over a workspace or over a task's worktree. In the task panel: **Review → Explore files**.
- Modified, added, untracked, deleted or conflicted files and folders are colored according to `git status`.
- While you edit, the left gutter marks added, modified and deleted lines against `HEAD` (as in VS Code).
- When **editing a task**, the CPU and memory limits live under *Advanced options*.

## Workspace board

Each workspace has its own **board**: a canvas of task nodes joined by arrows (`/board/<workspace>`; `/` opens the first workspace by name). Clicking a workspace in the sidebar opens its board, and the workspace chips at the top of the board switch between workspaces. The sidebar item menu keeps **View files** (the project tree), **Open terminal** and **Manage workspaces**.

- **Dependency graph:** drag from a node's right port onto another task to make it wait for the first one. Select an arrow to delete it. Self-references, duplicates, cycles, arrows across workspaces and arrows into a task that is no longer in *To do* are rejected with a message.
- **Merge trigger:** a task with predecessors starts on its own, on its configured base branch, once **all** of them are `merged`. A *To do* task without predecessors is started manually with **Start**. Arrows are drawn muted while the predecessor is pending, with a flowing dashed line while it works, and solid green once it is merged. (Edge conditions are not implemented yet; the model leaves room for them.)
- **Nested and parallel tasks:** *Add next task* creates a task that depends on the selected one; *Add parallel task* creates one with the same predecessors. Both are created in *To do*.
- **Canvas:** drag the background to pan, use the wheel to zoom and **Fit view** to frame everything. Node positions are saved; tasks without a saved position are laid out automatically in layers.
- **Loose tasks panel:** on the left, tasks with no arrows and no saved position (every task made with *New task* starts here) are listed with their usual actions (Start, Edit, Delete, Open, Merge…), so they can run standalone without any chain. Drag a card onto the canvas, or press **Add to graph**, to make it a node; *Add next / parallel task* tasks go straight into the graph. **Remove from graph** on a node deletes all its arrows and saved position (atomically) and sends it back to this panel; successors left without predecessors become ordinary standalone tasks. The panel is collapsible and counts its tasks; on screens under 768 px it becomes a **Loose (N) / Graph** tab switch.

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

This feature is the integration with [Gentle AI](https://github.com/Gentleman-Programming/gentle-ai), the agent harness behind `gentle-shell` and its gentle-pi package. Profiles and per-agent model routing are Gentle AI concepts: iworkflow does not define its own, it reads and writes the same configuration files (`profiles.json`, `models.json`), so changes made here are seen by the Gentle AI TUI and vice versa. Install and configure Gentle AI first; without it these screens have nothing to manage.

Two screens equivalent to the gentle-pi commands, working on the same files:

| Screen | Equivalent to | What it does |
|---|---|---|
| `/profiles` | `/gentle:profiles` | Create, duplicate, rename, delete, import and export profiles (`profiles.json`), edit model and effort per agent and create new agent roles. |
| `/models` | `/gentle:models` | Edit the global routing (`models.json`): model and effort per agent. |

Profiles are chosen in the agent settings dropdown, which reads the same `profiles.json`. These screens do not activate a profile globally; they only manage them. The write lock prevents clobbering within one process, not against another process (e.g. the gentle-pi TUI) editing at the same time.

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

### Executable arguments

On startup, `iworkflow` opens the web UI in the browser (`http://<host>:<port>` of the server; if it listens on `0.0.0.0`, `127.0.0.1` is used).

| Argument | Effect |
|---|---|
| `--no-open` | Starts only the server, without opening the browser. |
| `-h`, `--help` | Shows the help. |

Examples: `./iworkflow --no-open`, `./scripts/prod.sh run --no-open`, `cargo leptos watch -- --no-open`.

In development (`cargo leptos watch`) the server listens only on `127.0.0.1`.

## Production

Prerequisites: the Rust toolchain, `cargo-leptos` (`cargo install cargo-leptos --locked`) and the `wasm32-unknown-unknown` target (`rustup target add wasm32-unknown-unknown`).

`scripts/prod.sh` builds and runs the app in release mode:

```bash
./scripts/prod.sh build     # 1. compile in release and package into dist/
./scripts/prod.sh run       # 2. start what is already packaged
./scripts/prod.sh           # both steps in a row (build + run)
```

Typical flow: run `build` once (and again after every update), then `run` whenever you want to start the server. Arguments after `run` are passed to the binary (e.g. `--no-open`, see [Executable arguments](#executable-arguments)). The server listens on `http://0.0.0.0:9000` by default; change it with `LEPTOS_SITE_ADDR`:

```bash
LEPTOS_SITE_ADDR=127.0.0.1:3000 ./scripts/prod.sh run --no-open
DIST_DIR=/opt/iworkflow ./scripts/prod.sh build     # package somewhere else
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
| `LEPTOS_SITE_ADDR` | `0.0.0.0:9000` (all interfaces) |

> **Security:** the app has no authentication and launches agents that run commands
> as your user. With the bind on `0.0.0.0`, anyone who can reach the port can use it.
> Restrict access with a firewall, a private network (Tailscale/WireGuard) or a
> reverse proxy with authentication and TLS. To limit it to the local machine:
> `LEPTOS_SITE_ADDR=127.0.0.1:3000 ./scripts/prod.sh run`.

To deploy on another machine, just copy `dist/` and launch it with
`LEPTOS_SITE_ROOT=site LEPTOS_ENV=PROD LEPTOS_SITE_ADDR=0.0.0.0:3000 ./iworkflow`
from inside that folder.

## Developing from your phone with Tailscale

[Tailscale](https://tailscale.com) creates a private network (tailnet) between your devices, so your phone can reach the server running on your computer without opening ports to the internet. This matters because iworkflow has no authentication and launches agents that run commands as your user.

1. Install Tailscale on the computer that runs iworkflow and on the phone (App Store / Google Play), and sign in to the **same account** on both.
2. Start the server listening on the Tailscale interface (or all interfaces, which is the default of `scripts/prod.sh`):

   ```bash
   ./scripts/prod.sh run                                        # binds 0.0.0.0:9000
   # or only reachable from this machine and the tailnet via Tailscale Serve (see below)
   LEPTOS_SITE_ADDR=127.0.0.1:3000 ./scripts/prod.sh run --no-open
   ```

3. Find the computer's tailnet name or IP:

   ```bash
   tailscale ip -4        # e.g. 100.101.102.103
   tailscale status       # shows the MagicDNS name, e.g. mypc.tailnet-name.ts.net
   ```

4. On the phone, open `http://<tailnet-ip>:9000` or `http://mypc.tailnet-name.ts.net:9000`.

**Recommended: HTTPS with Tailscale Serve.** Keep the bind on `127.0.0.1` and let Tailscale expose it only inside your tailnet, with a valid certificate:

```bash
tailscale serve --bg 3000
# https://mypc.tailnet-name.ts.net  (only reachable by your tailnet devices)
```

Terminal WebSockets are accepted when the `Origin` host matches the `Host` header, which holds when you use the same address in the browser.

Tips:

- Do **not** use Tailscale Funnel: it would publish the app to the whole internet.
- Restrict who can reach the machine with Tailscale ACLs if the tailnet is shared.
- The computer must stay awake and the server running; use `tmux`/a systemd user service for long sessions.

## Development

Architecture and code guide in [ARCHITECTURE.md](ARCHITECTURE.md); project rules in [AGENTS.md](AGENTS.md). Checks:

```bash
cargo fmt --all -- --check
cargo clippy --features ssr --all-targets -- -D warnings
cargo clippy --features hydrate --target wasm32-unknown-unknown --lib -- -D warnings
cargo test --features ssr
PLAYWRIGHT_MODULE=/path/to/node_modules/playwright ./end2end/run_smoke.sh   # UI with a fake agent
```

## License

Copyright 2026 Pablo López (plopezm). Licensed under the [PolyForm Shield License 1.0.0](LICENSE).

You may use, modify and distribute iworkflow freely, including inside companies and for commercial work, but you may not use it to provide a product that competes with it (for example, reselling it or offering it as a hosted service). This is a source-available license, not an OSI-approved open-source one.

Bundled third-party assets keep their own licenses: xterm.js (MIT) and highlight.js (BSD-3-Clause) in `public/vendor/`, and the Instrument Serif and Martian Mono fonts (SIL OFL 1.1) in `public/fonts/`. All Rust dependencies use permissive licenses (MIT, Apache-2.0, BSD, Zlib, Unicode…).

## Support the project

If iworkflow saves you time, you can buy me a coffee at **[buymeacoffee.com/plopezm](https://buymeacoffee.com/plopezm)**. Thank you! ☕
