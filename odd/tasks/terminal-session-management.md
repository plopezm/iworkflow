# Feature: terminal-session-management

List open terminals and close them. Decision: persistent sessions — the shell survives leaving the page; sessions can be re-attached (with recent-output scrollback) or closed from a list.

Builds on feat/terminal-session (unmerged).

## Design
- Session lifecycle decoupled from the WebSocket: created by `open_terminal` server fn, killed by `close_terminal` or shell exit.
- Output: broadcast to N subscribers + bounded scrollback ring buffer replayed on attach.
- WS route attaches by session id: `GET /terminal/ws/session/{session_id}`.
- Pages: `/terminals` (list: workspace name snapshot, started at, status, Open/Close), `/terminals/:session_id` (terminal), `/workspaces/:id/terminal` opens a new session and redirects.
- Workspace name snapshot captured at open (resolver returns name + path); terminal slice never imports workspaces.

## Tasks
- [x] 1. Domain + use cases: session metadata (workspace name, created_at), `PtyPort` list/attach (scrollback + broadcast), ListTerminals / AttachTerminal use cases, OpenTerminal returns session info; mocks + tests
- [x] 2. PTY adapter: persistent registry, broadcast + scrollback ring buffer, exit detection, drop kills all; integration tests (reattach replays output, list, close)
- [x] 3. WS adapter attach-by-id + DI + server fns (`open_terminal`, `list_terminals`, `close_terminal`) + DTOs and mapping tests
- [x] 4. UI: `/terminals` list page + sidebar link, `/terminals/:id` page with close button, new-session redirect from `/workspaces/:id/terminal`, state + helpers tests
- [ ] 5. Checks (fmt, clippy ssr/hydrate, tests, live smoke) + codegraph sync

## Evidence
- task 1: 5bb5407
- task 2: 53405ec
- task 3: 0f54b74
- task 4: 9f956d4
