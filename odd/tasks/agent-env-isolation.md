# agent-env-isolation

Goal: every agent execution (task runtime + `--list-models` catalog) starts from an empty env with only
system session vars (HOME, USER, LOGNAME, SHELL, LANG, LC_*, DISPLAY, WAYLAND_DISPLAY, XDG_*,
DBUS_SESSION_BUS_ADDRESS, SSH_AUTH_SOCK...) and is launched through `$SHELL -l -c "exec '<agent>' ..."`
so the login shell rebuilds PATH (decision by user: login-shell launch).

- [x] 1. Extract shared session-env logic into `src/core/env.rs` (ssr) and reuse it in the PTY adapter
- [x] 2. Launch task agents via login shell with clean env (gentle_shell_runtime_adapter) + tests
- [x] 3. Same for model catalog `--list-models` + tests
- [x] 4. fmt, clippy ssr/hydrate, tests; commit
