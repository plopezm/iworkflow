# Getting started

> 🇪🇸 [Versión en castellano](es/getting-started.md) · [← README](../README.md)

Requirements: `git`, Go (the version in `go.mod`), `templ`, `tailwindcss` and at least one agent CLI installed and logged in (`gentle-shell`, `pi`, `claude`, `agent` for Cursor, or `agy` for Antigravity).

```bash
git clone https://github.com/plopezm/iworkflow.git
cd iworkflow
make dev                    # https://127.0.0.1:3000 (self-signed: accept the browser warning once)
```

1. **Workspaces** → register a folder with git (it needs at least one commit).
2. **Tasks** → pick a workspace, write the prompt and launch.
3. In the task detail you follow the agent live and can:
   - send messages (a prompt if it is waiting; *steer* or *follow-up* if it is working),
   - answer agent dialogs (`ask_user_question`, confirmations…),
   - abort the current run.
4. **Finish and review** → closes the agent, commits whatever is pending and shows the diff. If processes still use the worktree, a dialog lets you choose which to close first.
5. **Merge** (integrates with `--no-ff` and deletes the worktree) or **Discard**.

> **Security:** iworkflow is protected only by an optional access token and launches agents that run commands as your user. Do not expose it to untrusted networks; see [Production](production.md) and [Tailscale](mobile.md#developing-from-your-phone-with-tailscale). Development listens on `127.0.0.1:3000`; the production script on `0.0.0.0:9000`.
