#!/usr/bin/env bash
# E2E suite against the Go server with fake agents (no model use).
# Builds cmd/iworkflow, starts it on a free port with an isolated HOME and database in a temporary
# directory (never the user's ~/.iworkflow), starts the Jira and GitLab mocks, and runs the runner
# in Go mode (IWF_E2E_TARGET=go). See end2end/go/README.md for the full usage.
#
# Usage:
#   end2end/go/run_go.sh                       # whole Go suite (end2end/go/*.e2e.cjs)
#   end2end/go/run_go.sh workspaces            # only modules whose name contains "workspaces"
#   IWF_E2E_SCRIPTS="end2end/dock_smoke.cjs end2end/tab_state_smoke.cjs" end2end/go/run_go.sh
#                                              # standalone smoke scripts, one after another
#   IWF_E2E_UNIT=1 end2end/go/run_go.sh        # also run the Node unit steps first (no server needed)
#   end2end/go/run_go.sh usage_limits          # usage bar e2e: fake Claude OAuth usage API + fake Codex rollout;
#                                              # screenshots land in $RUN_DIR/screenshots (SMOKE_SCREENSHOT_DIR)
#
# The standalone scripts load end2end/go/lang_shim.cjs, which sets the language cookie they do not set.
# Every run keeps its server log, mock logs and runner output under RUN_DIR (printed at the end).
# Requires go, node, python3 and a Playwright install:
#   PLAYWRIGHT_MODULE=/path/to/node_modules/playwright   (defaults to the local cache)
#   PLAYWRIGHT_BROWSERS_PATH=/path/to/browsers           (defaults to ~/.cache/ms-playwright)
#   SMOKE_CHROMIUM_PATH=/path/to/chrome                  (optional)
#   IWF_E2E_RUN_DIR=/path/to/run-dir                     (optional; default /tmp/iwf-e2e-go-runs/<stamp>-<pid>)
#   IWF_E2E_PI_BIN=/path/to/agent                        (optional; the pi binary the server runs; default
#                                                        tests/fixtures/smoke_pi.sh. scripts/docs/record_gifs.sh sets it
#                                                        to the demo agent, scripts/docs/demo_agent.sh)
#   IWF_E2E_GENTLE_SHELL_BIN=/path/to/shell              (optional; the gentle-shell binary the server runs; default
#                                                        tests/fixtures/smoke_gentle_shell.sh; same use as above)
#   SMOKE_SCREENSHOT_DIR                                 (set by the runner to $RUN_DIR/screenshots)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORK="$(mktemp -d)"
RUN_DIR="${IWF_E2E_RUN_DIR:-/tmp/iwf-e2e-go-runs/$(date +%Y%m%d-%H%M%S)-$$}"
IWF_HOME_DIR="$WORK/iwf"
WORKTREES_DIR="$IWF_HOME_DIR/worktrees"
mkdir -p "$RUN_DIR"

# Stops the servers, keeps the logs of this run in RUN_DIR, then removes the scratch dir. Logs are
# copied before the removal so a failing run still leaves its evidence behind.
finish() {
  local status=$?
  kill "${SERVER_PID:-}" "${MOCK_PID:-}" "${GITLAB_PID:-}" "${USAGE_PID:-}" 2>/dev/null || true
  wait "${SERVER_PID:-}" "${MOCK_PID:-}" "${GITLAB_PID:-}" "${USAGE_PID:-}" 2>/dev/null || true
  mkdir -p "$RUN_DIR"
  cp "$WORK"/*.log "$RUN_DIR"/ 2>/dev/null || true
  rm -rf "$WORK"
  echo "▶ run logs kept in: $RUN_DIR" >&2
  exit "$status"
}
trap finish EXIT

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

# Optional unit steps. They need no server, so they run first and stop the script on failure.
# The gate is web/static/js (the Go module's browser helpers). end2end/follow_bottom_test.mjs targets
# the legacy src/core copy and has known failing expectations under the shim (see README), so it is
# reported but does not stop the run.
if [ "${IWF_E2E_UNIT:-0}" = "1" ]; then
  echo "▶ unit: web/static/js (browser helpers under plain Node)"
  node --test web/static/js/
  echo "▶ unit (informational): end2end/follow_bottom_test.mjs with the DOM shim"
  node --import "$ROOT/end2end/go/dom_shim.mjs" --test end2end/follow_bottom_test.mjs || echo "  ⚠ legacy follow_bottom test has known failures; see end2end/go/README.md"
fi

# web/static/app.css is a build artifact (git-ignored). Without it no Tailwind class applies: hidden panels
# stay visible and the browser tests fail in ways that look like app bugs. Build it when it is missing.
if [ ! -f "$ROOT/web/static/app.css" ]; then
  command -v tailwindcss >/dev/null 2>&1 || { echo "web/static/app.css is missing and tailwindcss is not installed: run 'make tools css'" >&2; exit 1; }
  echo "▶ build: web/static/app.css (missing)"
  tailwindcss -i "$ROOT/web/tailwind.css" -o "$ROOT/web/static/app.css" --minify
fi

# The sherpa tag enables the cgo speech recognizer used by the transcription e2e.
go build -tags sherpa -o "$WORK/iworkflow" ./cmd/iworkflow

# The Qwen3-ASR model lives in the real home (`make model`); the server runs with an isolated HOME, so it is passed by path.
ASR_MODEL_DIR="${IWF_ASR_MODEL_DIR:-${IWF_HOME:-$HOME/.iworkflow}/models/sherpa-onnx-qwen3-asr-0.6B-int8-2026-03-25}"
[ -f "$ASR_MODEL_DIR/decoder.int8.onnx" ] || ASR_MODEL_DIR=""
# The server finds the model under IWF_HOME/models; link the real one there.
mkdir -p "$IWF_HOME_DIR/models"
[ -z "$ASR_MODEL_DIR" ] || ln -s "$ASR_MODEL_DIR" "$IWF_HOME_DIR/models/sherpa-onnx-qwen3-asr-0.6B-int8-2026-03-25"

# Isolated home: gentle-pi config, pi agents and the database live under $WORK.
mkdir -p "$WORK/home/.config/gentle-pi" "$WORK/tmp"
# Fake subscription logins for the usage bar: a Claude OAuth credentials file (read by the usage mock, which accepts
# any token) and one Codex rollout whose rate_limits give the bar its warn (75 %) and critical (95 %) levels.
python3 -I - "$WORK/home" <<'PY'
import datetime
import json
import os
import pathlib
import sys

home = pathlib.Path(sys.argv[1])
creds_dir = home / ".claude"
creds_dir.mkdir(parents=True, exist_ok=True)
creds = creds_dir / ".credentials.json"
creds.write_text(json.dumps({"claudeAiOauth": {"accessToken": "e2e-fake-token"}}), encoding="utf-8")
os.chmod(creds, 0o600)

sessions = home / ".codex" / "sessions" / "2026" / "11" / "15"
sessions.mkdir(parents=True, exist_ok=True)
now = datetime.datetime.now(datetime.timezone.utc).isoformat().replace("+00:00", "Z")
event = {
    "timestamp": now,
    "type": "event_msg",
    "payload": {
        "type": "token_count",
        "info": None,
        "rate_limits": {
            "primary": {"used_percent": 75.0, "window_minutes": 300, "resets_in_seconds": 1800},
            "secondary": {"used_percent": 95.0, "window_minutes": 10080, "resets_in_seconds": 86400},
        },
    },
}
# Compact separators: the Codex facade looks for the literal '"rate_limits":{', as Codex writes it.
(sessions / "rollout-e2e.jsonl").write_text(json.dumps(event, separators=(",", ":")) + "\n", encoding="utf-8")
PY
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
# One repo per standalone smoke that registers a workspace: the server refuses a second workspace on the same root.
REPO_UI="$WORK/ui/project";                     make_repo "$REPO_UI"
REPO_STALE_DIALOG="$WORK/stale-dialog/project"; make_repo "$REPO_STALE_DIALOG"
REPO_TERMINAL_KEYS="$WORK/terminal-keys/project"; make_repo "$REPO_TERMINAL_KEYS"
REPO_TERMINAL_RECONNECT="$WORK/terminal-reconnect/project"; make_repo "$REPO_TERMINAL_RECONNECT"

PORT="$(free_port)"
# Secret of the fake Engram daemon's /_stop, so no other local process can stop it.
FAKE_ENGRAM_TOKEN="$(head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n')"
export IWF_E2E_FAKE_ENGRAM_TOKEN="$FAKE_ENGRAM_TOKEN"
JIRA_MOCK_PORT="$(free_port)"
GITLAB_MOCK_PORT="$(free_port)"
# Fake Engram API: the server reads it through IWF_ENGRAM_URL (env -i drops every other variable). The engram
# suite starts the fake on this port; the other suites see it as unavailable, as they would see the default.
ENGRAM_PORT="$(free_port)"
# Fake Claude usage API (usage_mock.cjs) for the usage bar; the server reads it through IWF_CLAUDE_USAGE_URL.
USAGE_MOCK_PORT="$(free_port)"

# Server: started from the scratch dir, since the fake agents write to their cwd. exec makes the
# server the background process itself, so the trap stops it. stdout and stderr both go to server.log.
(
  cd "$WORK"
  exec env -i PATH="$ROOT/end2end/go/fake_engram:$PATH" IWF_E2E_FAKE_ENGRAM_LOG="$WORK/fake-engram-launches.log" IWF_E2E_FAKE_ENGRAM_TOKEN="$FAKE_ENGRAM_TOKEN" HOME="$WORK/home" TMPDIR="$WORK/tmp" \
    GIT_AUTHOR_NAME=smoke GIT_AUTHOR_EMAIL=smoke@localhost \
    GIT_COMMITTER_NAME=smoke GIT_COMMITTER_EMAIL=smoke@localhost \
    IWF_HOME="$IWF_HOME_DIR" \
    IWF_TLS=off \
    IWF_ENGRAM_URL="http://127.0.0.1:$ENGRAM_PORT" \
    IWF_CLAUDE_CREDENTIALS="$WORK/home/.claude/.credentials.json" \
    IWF_CLAUDE_USAGE_URL="http://127.0.0.1:$USAGE_MOCK_PORT/api/oauth/usage" \
    IWF_CODEX_SESSIONS_DIR="$WORK/home/.codex/sessions" \
    IWF_GENTLE_PI_CONFIG_HOME="$WORK/home/.config/gentle-pi" \
    IWF_GENTLE_SHELL_BIN="${IWF_E2E_GENTLE_SHELL_BIN:-$ROOT/tests/fixtures/smoke_gentle_shell.sh}" \
    IWF_LOG_LEVEL=debug \
    IWF_PI_BIN="${IWF_E2E_PI_BIN:-$ROOT/tests/fixtures/smoke_pi.sh}" \
    IWF_CURSOR_BIN="$ROOT/tests/fixtures/smoke_cursor.sh" \
    IWF_CLAUDE_BIN="$ROOT/tests/fixtures/smoke_claude.sh" \
    "$WORK/iworkflow" --addr "127.0.0.1:$PORT" --site-root "$ROOT/web/static" > "$WORK/server.log" 2>&1
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
USAGE_MOCK_PORT="$USAGE_MOCK_PORT" node end2end/usage_mock.cjs > "$WORK/usage_mock.log" 2>&1 &
USAGE_PID=$!
for _ in $(seq 1 25); do
  curl -fs "http://127.0.0.1:$JIRA_MOCK_PORT/_health" > /dev/null && \
    curl -fs "http://127.0.0.1:$GITLAB_MOCK_PORT/_health" > /dev/null && \
    curl -fs "http://127.0.0.1:$USAGE_MOCK_PORT/_health" > /dev/null && break
  sleep 0.1
done

export SMOKE_BASE_URL="http://127.0.0.1:$PORT"
export WORK_TMP_DIR="$WORK/tmp" SMOKE_TMP_DIR="$WORK/tmp" PROOF_REPO="$REPO"
# Same directory the server writes task worktrees to (IWF_HOME/worktrees).
export SMOKE_WORKTREES="$WORKTREES_DIR"
export JIRA_MOCK_URL="http://127.0.0.1:$JIRA_MOCK_PORT" JIRA_MOCK_PORT
export GITLAB_MOCK_URL="http://127.0.0.1:$GITLAB_MOCK_PORT" GITLAB_MOCK_PORT
export REPO_GIT_CHANGES REPO_TAB_STATE REPO_BG_JOBS REPO_TERMINAL REPO_UI REPO_STALE_DIALOG REPO_TERMINAL_KEYS REPO_TERMINAL_RECONNECT
export IWF_E2E_TARGET=go IWF_ASR_MODEL_DIR="$ASR_MODEL_DIR"
# Screenshots of the suites that take them (usage_limits): kept with the run logs.
export SMOKE_SCREENSHOT_DIR="$RUN_DIR/screenshots"
mkdir -p "$SMOKE_SCREENSHOT_DIR"
export IWF_E2E_ENGRAM_URL="http://127.0.0.1:$ENGRAM_PORT"
# Launches of the fake `engram run -d` (end2end/go/fake_engram), one line each: the Engram settings suite reads it.
export IWF_E2E_ENGRAM_LAUNCH_LOG="$WORK/fake-engram-launches.log"

# The test processes see the isolated HOME: workspaces.e2e.cjs browses under the process HOME, which
# must be the server's home_dir.
run_node() { env HOME="$WORK/home" node "$@"; }

# The repo each standalone smoke registers as its workspace (SMOKE_REPO).
repo_for_script() {
  case "$1" in
    *git_changes*)     echo "$REPO_GIT_CHANGES" ;;
    *tab_state*)       echo "$REPO_TAB_STATE" ;;
    *background_jobs*) echo "$REPO_BG_JOBS" ;;
    *ui_smoke*)        echo "$REPO_UI" ;;
    *stale_dialog*)    echo "$REPO_STALE_DIALOG" ;;
    *terminal_keys*)   echo "$REPO_TERMINAL_KEYS" ;;
    *terminal_reconnect*) echo "$REPO_TERMINAL_RECONNECT" ;;
    *)                 echo "$REPO_TERMINAL" ;;
  esac
}

# Runs the suite (or the standalone scripts) and returns the first failing status.
run_tests() {
  echo "▶ Go e2e suite base=$SMOKE_BASE_URL ${IWF_E2E_SCRIPTS:+scripts=$IWF_E2E_SCRIPTS} ${1:+filter=$1}"
  if [ -n "${IWF_E2E_SCRIPTS:-}" ]; then
    local status=0 script
    for script in $IWF_E2E_SCRIPTS; do
      echo "── $script"
      SMOKE_REPO="$(repo_for_script "$script")" run_node -r "$ROOT/end2end/go/lang_shim.cjs" "$script" || status=1
    done
    return "$status"
  fi
  run_node end2end/run.e2e.cjs "$@"
}

set +e
run_tests "$@" 2>&1 | tee "$RUN_DIR/runner.log"
STATUS=${PIPESTATUS[0]}
set -e
exit "$STATUS"
