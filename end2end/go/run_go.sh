#!/usr/bin/env bash
# E2E suite against the Go server with fake agents (no model use).
# Builds cmd/iworkflow, starts it on a free port with an isolated HOME and database in a temporary
# directory (never the user's ~/.iworkflow), starts the Jira and GitLab mocks, and runs the runner
# in Go mode (IWF_E2E_TARGET=go).
#
# Usage:
#   end2end/go/run_go.sh                       # whole suite
#   end2end/go/run_go.sh workspaces            # only modules whose name contains "workspaces"
#   IWF_E2E_SCRIPTS="end2end/dock_smoke.cjs end2end/tab_state_smoke.cjs" end2end/go/run_go.sh
#                                              # standalone smoke scripts, one after another
#
# The standalone scripts load end2end/go/lang_shim.cjs, which sets the language cookie they do not set.
# Requires go, node, python3 and a Playwright install:
#   PLAYWRIGHT_MODULE=/path/to/node_modules/playwright   (defaults to the local cache)
#   PLAYWRIGHT_BROWSERS_PATH=/path/to/browsers           (defaults to ~/.cache/ms-playwright)
#   SMOKE_CHROMIUM_PATH=/path/to/chrome                  (optional)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORK="$(mktemp -d)"
trap 'kill "${SERVER_PID:-}" "${MOCK_PID:-}" "${GITLAB_PID:-}" 2>/dev/null || true; rm -rf "$WORK"' EXIT

# The browsers live in the real home; the test processes run with the isolated HOME (see below).
export PLAYWRIGHT_BROWSERS_PATH="${PLAYWRIGHT_BROWSERS_PATH:-$HOME/.cache/ms-playwright}"
export PLAYWRIGHT_MODULE="${PLAYWRIGHT_MODULE:-/home/pablolm/.cache/iwf-playwright/node_modules/playwright}"

free_port() { python3 -I -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])'; }

make_repo() {
  mkdir -p "$1"
  git -C "$1" init -q -b main
  echo hola > "$1/README.md"
  git -C "$1" add .
  git -C "$1" -c user.name=smoke -c user.email=smoke@localhost commit -qm init
}

cd "$ROOT"
go build -o "$WORK/iworkflow" ./cmd/iworkflow

# Isolated home: gentle-pi config, pi agents and the database live under $WORK.
mkdir -p "$WORK/home/.config/gentle-pi" "$WORK/tmp"
# Seed one active profile and the global routing from the fixture, so the profile screens have data.
python3 -I - "$ROOT/tests/fixtures/gentle_pi_profile.export.json" "$WORK/home/.config/gentle-pi" <<'PY'
import json
import pathlib
import sys

fixture = json.load(open(sys.argv[1], encoding="utf-8"))
out = pathlib.Path(sys.argv[2])
config = fixture["config"]
store = {"kind": "gentle-pi.agent_model_profiles", "active": "Claude", "profiles": {"Claude": config}}
(out / "profiles.json").write_text(json.dumps(store, indent=2), encoding="utf-8")
(out / "models.json").write_text(json.dumps(config, indent=2), encoding="utf-8")
PY

REPO="$WORK/repo"
make_repo "$REPO"
REPO_GIT_CHANGES="$WORK/git-changes/project";   make_repo "$REPO_GIT_CHANGES"
REPO_TAB_STATE="$WORK/tab-state/project";       make_repo "$REPO_TAB_STATE"
REPO_BG_JOBS="$WORK/bg-jobs/project";           make_repo "$REPO_BG_JOBS"
REPO_TERMINAL="$WORK/terminal/project";         make_repo "$REPO_TERMINAL"

PORT="$(free_port)"
JIRA_MOCK_PORT="$(free_port)"
GITLAB_MOCK_PORT="$(free_port)"

# Server: started from the scratch dir, since the fake agents write to their cwd. exec makes the
# server the background process itself, so the trap stops it.
(
  cd "$WORK"
  exec env -i PATH="$PATH" HOME="$WORK/home" TMPDIR="$WORK/tmp" \
    GIT_AUTHOR_NAME=smoke GIT_AUTHOR_EMAIL=smoke@localhost \
    GIT_COMMITTER_NAME=smoke GIT_COMMITTER_EMAIL=smoke@localhost \
    IWF_DATABASE_URL="$WORK/db.sqlite" \
    IWF_WORKTREES_DIR="$WORK/worktrees" \
    IWF_GENTLE_PI_CONFIG_HOME="$WORK/home/.config/gentle-pi" \
    IWF_PI_AGENTS_DIR="$WORK/home/.pi/agent/agents" \
    IWF_GENTLE_SHELL_BIN="$ROOT/tests/fixtures/smoke_gentle_shell.sh" \
    IWF_PI_BIN="$ROOT/tests/fixtures/smoke_pi.sh" \
    IWF_CURSOR_BIN="$ROOT/tests/fixtures/smoke_cursor.sh" \
    IWF_CLAUDE_BIN="$ROOT/tests/fixtures/smoke_claude.sh" \
    LEPTOS_SITE_ROOT="$ROOT/web/static" \
    LEPTOS_SITE_ADDR="127.0.0.1:$PORT" \
    "$WORK/iworkflow" > "$WORK/server.log" 2>&1
) &
SERVER_PID=$!

for _ in $(seq 1 50); do
  curl -fs "http://127.0.0.1:$PORT/healthz" > /dev/null && break
  sleep 0.2
done

# Mocks for the jira and code_hosting (GitLab) modules.
JIRA_MOCK_PORT="$JIRA_MOCK_PORT" node end2end/jira_mock.cjs > "$WORK/jira_mock.log" 2>&1 &
MOCK_PID=$!
GITLAB_MOCK_PORT="$GITLAB_MOCK_PORT" node end2end/gitlab_mock.cjs > "$WORK/gitlab_mock.log" 2>&1 &
GITLAB_PID=$!
for _ in $(seq 1 25); do
  curl -fs "http://127.0.0.1:$JIRA_MOCK_PORT/_health" > /dev/null && \
    curl -fs "http://127.0.0.1:$GITLAB_MOCK_PORT/_health" > /dev/null && break
  sleep 0.1
done

export SMOKE_BASE_URL="http://127.0.0.1:$PORT"
export WORK_TMP_DIR="$WORK/tmp" SMOKE_TMP_DIR="$WORK/tmp" PROOF_REPO="$REPO"
export JIRA_MOCK_URL="http://127.0.0.1:$JIRA_MOCK_PORT" JIRA_MOCK_PORT
export GITLAB_MOCK_URL="http://127.0.0.1:$GITLAB_MOCK_PORT" GITLAB_MOCK_PORT
export REPO_GIT_CHANGES REPO_TAB_STATE REPO_BG_JOBS REPO_TERMINAL
export IWF_E2E_TARGET=go

# The test processes see the isolated HOME: workspaces.e2e.cjs browses under the process HOME, which
# must be the server's home_dir.
run_node() { env HOME="$WORK/home" node "$@"; }

echo "▶ Go e2e suite base=$SMOKE_BASE_URL ${IWF_E2E_SCRIPTS:+scripts=$IWF_E2E_SCRIPTS} ${1:+filter=$1}"
set +e
if [ -n "${IWF_E2E_SCRIPTS:-}" ]; then
  STATUS=0
  for script in $IWF_E2E_SCRIPTS; do
    case "$script" in
      *tab_state*) SMOKE_REPO="$REPO_TAB_STATE" ;;
      *)           SMOKE_REPO="$REPO_TERMINAL" ;;
    esac
    echo "── $script"
    SMOKE_REPO="$SMOKE_REPO" run_node -r "$ROOT/end2end/go/lang_shim.cjs" "$script" || STATUS=1
  done
else
  run_node end2end/run.e2e.cjs "$@"
  STATUS=$?
fi
set -e
cp "$WORK/server.log" /tmp/iwf_go_server.log 2>/dev/null || true
exit "$STATUS"
