# iworkflow

> 🇪🇸 Versión en castellano: [README.es.md](README.es.md)

**iworkflow is an ADE (Agentic Development Environment)**: a self-hosted web app to launch, follow and review several AI coding agents in parallel. Instead of editing files yourself, you manage **tasks**: each one runs an agent in its own git worktree, and you review the diff before anything reaches your branch.

<p align="center">
  <img src="docs/media/task-graph.gif" alt="iworkflow task graph: chained tasks wait for their predecessors, and a merged task starts its successors automatically" width="900" />
  <br />
  <sub>Chain tasks on a board; when one is merged, the tasks that depend on it start on their own.</sub>
</p>

> **Status:** early and actively developed. It runs on your own machine, has no user accounts (only an optional access token for network access) and is source-available (PolyForm Shield 1.0.0), not OSI open source. See [License](#license).

## Highlights

- **Parallel agents, one place**: launch and steer `gentle-shell`, `pi`, `claude`, `cursor` or `agy` side by side.
- **Isolated by design**: every task gets its own **git worktree**, so agents never overwrite each other.
- **Review, don't redo**: when an agent finishes you read its diff and either merge it (`merge --no-ff`) or discard it.
- **Task graph**: chain tasks on a board; a task starts by itself once its predecessors are merged.
- **Usage limits at a glance**: a usage bar shows your Claude and Codex five-hour and weekly windows, and tasks stopped by a limit resume when it renews.
- **Task hooks**: run scripts or agent prompts before or after a task starts or finishes, per project.
- **Integrated**: GitLab / GitHub (MR/PR and CI status), Jira Server / Data Center and Engram memory.
- **Mobile-first**: check progress, answer an agent's question and merge from your phone (through Tailscale).
- **Tabbed IDE-like workspace**: board, task, diff, terminal and file editor as draggable, splittable tabs, plus voice dictation for prompts.
- **Desktop app**: an Electron installer for Linux, macOS and Windows wraps the same server.

iworkflow imposes no workflow: each agent uses the configuration and plugins you already have installed, plus the project's `AGENTS.md`. Under the hood agents are driven over RPC (or ACP / stream-json, depending on the runtime). It is built with Go (`net/http` + `chi`), SSR with `templ` and `htmx`, Tailwind CSS, `sqlc` and SQLite.

## Quick start

```bash
git clone https://github.com/plopezm/iworkflow.git
cd iworkflow
make dev                    # https://127.0.0.1:3000 (self-signed: accept the browser warning once)
```

You need `git`, Go, `templ`, `tailwindcss` and at least one agent CLI installed and logged in. Then register a workspace, write a prompt, follow the agent live, review the diff and merge. The full walkthrough is in [Getting started](docs/getting-started.md).

> **Security:** iworkflow is protected only by an optional access token and launches agents that run commands as your user. Do not expose it to untrusted networks.

<p align="center">
  <img src="docs/media/task-review.gif" alt="iworkflow task review: launch an agent, follow it live, review its diff and merge it" width="800" />
  <br />
  <sub>One task end to end: launch, follow the agent live, review the diff, merge.</sub>
</p>

## Learn more

| Topic | What you will find |
|---|---|
| [Getting started](docs/getting-started.md) | Requirements, first workspace, first task, review and merge |
| [Workspace, board and lifecycle](docs/workspace.md) | Tabbed workspace, task graph, task states, [usage limits](docs/workspace.md#usage-limits), [task hooks](docs/workspace.md#task-hooks) |
| [Integrations](docs/integrations.md) | GitLab and GitHub, Jira, Gentle AI profiles and models, Engram memory |
| [Configuration](docs/configuration.md) | Environment variables, data folder, logging, voice dictation, HTTPS, agent runtimes, network access |
| [Production](docs/production.md) | Release build, running and deploying the server |
| [Desktop app](docs/desktop.md) | Electron installer, updates and release workflow |
| [Mobile and Tailscale](docs/mobile.md) | Using it from your phone, safely |
| [Development](docs/development.md) | Make targets, tests and recording the GIFs |
| [Architecture](ARCHITECTURE.md) | Code map: modules, runtimes, persistence, frontend |
| [Project rules](AGENTS.md) | Coding and process rules for contributors and agents |

## License

Copyright 2026 Pablo López (plopezm). Licensed under the [PolyForm Shield License 1.0.0](LICENSE).

You may use, modify and distribute iworkflow freely, including inside companies and for commercial work, but you may not use it to provide a product that competes with it (for example, reselling it or offering it as a hosted service). This is a source-available license, not an OSI-approved open-source one.

Bundled third-party assets keep their own licenses: xterm.js (MIT) and highlight.js (BSD-3-Clause) in `web/static/vendor/`, and the Instrument Serif and Martian Mono fonts (SIL OFL 1.1) in `web/static/fonts/`. Go dependencies are declared in `go.mod`; see [NOTICE](NOTICE) for attributions.

## Support the project

<p align="center">
  <a href="https://buymeacoffee.com/plopezm" target="_blank" rel="noopener">
    <img src="https://img.buymeacoffee.com/button-api/?text=Buy%20me%20a%20coffee&emoji=%E2%98%95&slug=plopezm&button_colour=FFDD00&font_colour=000000&font_family=Cookie&outline_colour=000000&coffee_colour=ffffff" alt="Buy me a coffee" height="50" />
  </a>
</p>

If iworkflow saves you time, you can buy me a coffee at **[buymeacoffee.com/plopezm](https://buymeacoffee.com/plopezm)**. Thank you! ☕
