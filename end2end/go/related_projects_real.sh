#!/usr/bin/env bash
# Real-agent e2e of the related projects feature (end2end/related_projects.e2e.cjs). Unlike run_go.sh, which runs
# every agent as a fake fixture, this starts the Go server with the real claude binary, so it spends model usage.
# Keep the scenarios short. Isolation: the server runs with an isolated HOME, IWF_HOME and worktrees under a
# mktemp -d. The claude config dir is a scratch dir that only links the login file of the host (never copied, never
# written through), so the host's ~/.claude and ~/.claude.json are not touched.
#
# Usage:
#   end2end/go/related_projects_real.sh
#   IWF_REAL_CLAUDE_BIN=/path/to/claude end2end/go/related_projects_real.sh
#   IWF_E2E_RUN_DIR=/path/to/run-dir end2end/go/related_projects_real.sh   # where the logs are kept
# Requires go, node, python3, git, curl and the Playwright install that run_go.sh needs.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORK="$(mktemp -d)"
RUN_DIR="${IWF_E2E_RUN_DIR:-/tmp/iwf-e2e-real-runs/$(date +%Y%m%d-%H%M%S)-$$}"
CLAUDE_BIN="${IWF_REAL_CLAUDE_BIN:-$HOME/.local/bin/claude}"
HOST_LOGIN="$HOME/.claude/.credentials.json"
IWF_HOME_DIR="$WORK/iwf"
mkdir -p "$RUN_DIR"

# Stops the server (its shutdown stops the agents it started), keeps the logs in RUN_DIR, then removes the scratch dir.
# Any agent process still running with its cwd under the scratch dir is ours, so it is stopped too; processes outside
# the scratch dir are never touched.
finish() {
  local status=$?
  kill "${SERVER_PID:-}" 2>/dev/null || true
  wait "${SERVER_PID:-}" 2>/dev/null || true
  local proc cwd
  for proc in /proc/[0-9]*; do
    cwd="$(readlink "$proc/cwd" 2>/dev/null || true)"
    case "$cwd" in
      "$WORK"/*) echo "▶ stopping leftover agent ${proc#/proc/} in $cwd" >&2; kill "${proc#/proc/}" 2>/dev/null || true ;;
    esac
  done
  mkdir -p "$RUN_DIR"
  cp "$WORK"/*.log "$RUN_DIR"/ 2>/dev/null || true
  cp "$IWF_HOME_DIR"/workspaces.db* "$RUN_DIR"/ 2>/dev/null || true
  rm -rf "$WORK"
  echo "▶ run logs kept in: $RUN_DIR" >&2
  exit "$status"
}
trap finish EXIT

export PLAYWRIGHT_BROWSERS_PATH="${PLAYWRIGHT_BROWSERS_PATH:-$HOME/.cache/ms-playwright}"
export PLAYWRIGHT_MODULE="${PLAYWRIGHT_MODULE:-/home/pablolm/.cache/iwf-playwright/node_modules/playwright}"

free_port() { python3 -I -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])'; }

[ -x "$CLAUDE_BIN" ] || { echo "claude binary not found or not executable: $CLAUDE_BIN" >&2; exit 1; }
[ -f "$HOST_LOGIN" ] || { echo "no login file at $HOST_LOGIN: claude cannot authenticate" >&2; exit 1; }
command -v tailwindcss >/dev/null 2>&1 || { echo "tailwindcss is not installed: run 'make tools' (the CSS is built into the scratch dir)" >&2; exit 1; }

cd "$ROOT"
go build -tags sherpa -o "$WORK/iworkflow" ./cmd/iworkflow

# The site root is a scratch copy, so the worktree never gets the git-ignored app.css. Tailwind scans the templates
# from the repository root (cwd).
cp -r "$ROOT/web/static" "$WORK/site"
tailwindcss -i "$ROOT/web/tailwind.css" -o "$WORK/site/app.css" --minify > "$WORK/tailwind.log" 2>&1

# Isolated home with the claude config of the run. Children of the server keep only HOME among the variables that
# locate the config (internal/core/env), so the login must live in $HOME/.claude. It is a copy, not a link: a token
# refresh then writes to the scratch copy, never to the host's file. The scratch dir is removed on exit.
mkdir -p "$WORK/home/.claude" "$WORK/tmp"
install -m 600 "$HOST_LOGIN" "$WORK/home/.claude/.credentials.json"
# The default model of the account is the largest one; the scenarios only need a cheap model.
printf '%s\n' '{"model": "claude-haiku-5-5"}' > "$WORK/home/.claude/settings.json"

PORT="$(free_port)"

# Server: started from the scratch dir, as in run_go.sh. The real claude binary is the only agent; pi and cursor stay
# on their fixtures because this suite does not use them.
(
  cd "$WORK"
  exec env -i PATH="$PATH" HOME="$WORK/home" TMPDIR="$WORK/tmp" \
    GIT_AUTHOR_NAME=smoke GIT_AUTHOR_EMAIL=smoke@localhost \
    GIT_COMMITTER_NAME=smoke GIT_COMMITTER_EMAIL=smoke@localhost \
    IWF_HOME="$IWF_HOME_DIR" \
    IWF_GENTLE_PI_CONFIG_HOME="$WORK/home/.config/gentle-pi" \
    IWF_PI_AGENTS_DIR="$WORK/home/.pi/agent/agents" \
    IWF_GENTLE_SHELL_BIN="$ROOT/tests/fixtures/smoke_gentle_shell.sh" \
    IWF_LOG_LEVEL=debug \
    IWF_PI_BIN="$ROOT/tests/fixtures/smoke_pi.sh" \
    IWF_CURSOR_BIN="$ROOT/tests/fixtures/smoke_cursor.sh" \
    IWF_CLAUDE_BIN="$CLAUDE_BIN" \
    "$WORK/iworkflow" --addr "127.0.0.1:$PORT" --site-root "$WORK/site" > "$WORK/server.log" 2>&1
) &
SERVER_PID=$!

for _ in $(seq 1 50); do
  curl -fs "http://127.0.0.1:$PORT/healthz" > /dev/null && break
  sleep 0.2
done
curl -fs "http://127.0.0.1:$PORT/healthz" > /dev/null || { echo "the server did not start: see $WORK/server.log" >&2; exit 1; }

export SMOKE_BASE_URL="http://127.0.0.1:$PORT"
export SMOKE_TMP_DIR="$WORK/tmp" WORK_TMP_DIR="$WORK/tmp"
export IWF_E2E_SERVER_LOG="$WORK/server.log" IWF_E2E_RUN_DIR="$RUN_DIR"

set +e
env HOME="$WORK/home" node end2end/run.e2e.cjs related_projects 2>&1 | tee "$RUN_DIR/runner.log"
STATUS=${PIPESTATUS[0]}
set -e
exit "$STATUS"
