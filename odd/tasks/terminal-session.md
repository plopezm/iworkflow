# Feature: terminal-session

Open an interactive terminal (real PTY + xterm.js) in a workspace's directory to run commands.

Decision: real PTY streamed over WebSocket (axum `ws`), `portable-pty` on the server, xterm.js in the browser.

## Tasks
- [x] 1. Domain + use cases: `terminal` slice (TerminalSession model, errors, `PtyPort` + `WorkspacePathPort` outbound ports, Open/Write/Resize/Close use cases with mockall tests, `mocks.rs`)
- [x] 2. PTY adapter (`portable-pty`, ssr only) + session registry; integration test spawning `sh`
- [x] 3. DI context (`terminal_context.rs`) + `AppState` wiring + WebSocket inbound adapter (axum router merged in `main.rs`) with message-mapping tests
- [ ] 4. Presentation: xterm.js asset, `TerminalPage`/component, state, route `/workspaces/:id/terminal`, entry point from workspaces page (tailwind)
- [ ] 5. Checks: fmt, clippy ssr/hydrate, tests, manual smoke; update codegraph

## Evidence
(commits recorded here per task)
- task 1: 0dfa8fb
- task 2: d883b3b
- task 3: 1cc403f
