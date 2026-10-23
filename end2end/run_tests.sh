#!/usr/bin/env bash
# Suite de tests e2e con agentes falsos (no consume modelo).
# Levanta un servidor con una BD temporal y agentes falsos, y ejecuta el runner
# de end2end/*.e2e.cjs.
#
# Uso:
#   end2end/run_tests.sh                 # toda la suite
#   end2end/run_tests.sh workspaces      # solo módulos cuyo nombre contenga "workspaces"
#
# Requiere cargo-leptos, python3 y Playwright local:
#   PLAYWRIGHT_MODULE=/ruta/a/node_modules/playwright
#   SMOKE_CHROMIUM_PATH=/ruta/a/chrome  (opcional, si falta el Chromium de Playwright)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"
trap 'cp "$WORK/server.log" /tmp/iwf_server.log 2>/dev/null; kill "${SERVER_PID:-}" 2>/dev/null || true; kill "${MOCK_PID:-}" 2>/dev/null || true; kill "${GITLAB_PID:-}" 2>/dev/null || true; rm -rf "$WORK"' EXIT

# ---- build (solo si falta; el suite usa la app construida una sola vez) ----
cd "$ROOT"
if [ "${FORCE_BUILD:-0}" = "1" ] || [ ! -x "$ROOT/target/debug/iworkflow" ] || [ ! -d "$ROOT/target/site" ]; then
  echo "▶ build cargo leptos build ..."
  cargo leptos build
fi

# ---- puerto libre ----
PORT="$(python3 -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')"

# ---- repos temporales (un por smoke/test que los necesita) ----
make_repo() {
  mkdir -p "$1"
  git -C "$1" init -q -b main
  echo hola > "$1/README.md"
  git -C "$1" add .
  git -C "$1" -c user.name=smoke -c user.email=smoke@localhost commit -qm init
}
REPO="$WORK/repo"
make_repo "$REPO"
REPO_GIT_CHANGES="$WORK/git-changes/project";   make_repo "$REPO_GIT_CHANGES"
REPO_TAB_STATE="$WORK/tab-state/project";       make_repo "$REPO_TAB_STATE"
REPO_BG_JOBS="$WORK/bg-jobs/project";           make_repo "$REPO_BG_JOBS"
REPO_TERMINAL="$WORK/terminal/project";         make_repo "$REPO_TERMINAL"

# La carpeta base para carpetas no-git (tests de inicialización).
WORK_TMP_DIR="$WORK/nongit"
mkdir -p "$WORK_TMP_DIR"

# ---- servidor desde el scratch dir (los agentes falsos escriben a su cwd) ----
cd "$WORK"
GIT_AUTHOR_NAME=smoke GIT_AUTHOR_EMAIL=smoke@localhost \
GIT_COMMITTER_NAME=smoke GIT_COMMITTER_EMAIL=smoke@localhost \
IWF_DATABASE_URL="$WORK/db.sqlite" \
IWF_WORKTREES_DIR="$WORK/worktrees" \
IWF_GENTLE_SHELL_BIN="$ROOT/tests/fixtures/smoke_gentle_shell.sh" \
IWF_PI_BIN="$ROOT/tests/fixtures/smoke_pi.sh" \
IWF_CURSOR_BIN="$ROOT/tests/fixtures/smoke_cursor.sh" \
IWF_CLAUDE_BIN="$ROOT/tests/fixtures/smoke_claude.sh" \
LEPTOS_SITE_ROOT="$ROOT/target/site" \
LEPTOS_SITE_ADDR="127.0.0.1:$PORT" \
  "$ROOT/target/debug/iworkflow" > "$WORK/server.log" 2>&1 &
SERVER_PID=$!
cd "$ROOT"

for _ in $(seq 1 50); do
  curl -fs "http://127.0.0.1:$PORT/" > /dev/null && break
  sleep 0.2
done

# ---- Jira mock (REST v2) para el módulo jira ----
JIRA_MOCK_PORT="$(python3 -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')"
JIRA_MOCK_URL="http://127.0.0.1:$JIRA_MOCK_PORT"
JIRA_MOCK_PORT="$JIRA_MOCK_PORT" node end2end/jira_mock.cjs > "$WORK/jira_mock.log" 2>&1 &
MOCK_PID=$!
for _ in $(seq 1 25); do
  curl -fs "http://127.0.0.1:$JIRA_MOCK_PORT/_health" > /dev/null && break
  sleep 0.1
done

# ---- GitLab mock (API v4) para el módulo code_hosting ----
GITLAB_MOCK_PORT="$(python3 -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')"
GITLAB_MOCK_URL="http://127.0.0.1:$GITLAB_MOCK_PORT"
GITLAB_MOCK_PORT="$GITLAB_MOCK_PORT" node end2end/gitlab_mock.cjs > "$WORK/gitlab_mock.log" 2>&1 &
GITLAB_PID=$!
for _ in $(seq 1 25); do
  curl -fs "http://127.0.0.1:$GITLAB_MOCK_PORT/_health" > /dev/null && break
  sleep 0.1
done

# ---- entorno del runner ----
SMOKE_BASE_URL="http://127.0.0.1:$PORT"
PLAYWRIGHT_MODULE="${PLAYWRIGHT_MODULE:-/home/pablolm/.cache/iwf-playwright/node_modules/playwright}"
export SMOKE_BASE_URL PLAYWRIGHT_MODULE WORK_TMP_DIR SMOKE_TMP_DIR="$WORK_TMP_DIR"
export JIRA_MOCK_URL JIRA_MOCK_PORT GITLAB_MOCK_URL GITLAB_MOCK_PORT
export PROOF_REPO="$REPO"
export REPO_GIT_CHANGES REPO_TAB_STATE REPO_BG_JOBS REPO_TERMINAL

FILTER="${1:-}"
echo "▶ running end2end suite (modules: $(ls end2end/*.e2e.cjs 2>/dev/null | sed -e 's/.*\///' -e 's/\.e2e\.cjs$//' | tr '\n' ' ')) base=$SMOKE_BASE_URL ${FILTER:+filter=$FILTER}"
set -- "$@"
node end2end/run.e2e.cjs "$@"