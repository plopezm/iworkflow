#!/usr/bin/env bash
# Real-process check of the "move the data folder" card: builds the server, runs it with an isolated HOME
# and NO IWF_HOME (the pointer is only honoured without it), asks for a move over HTTP and checks that, after
# the unsupervised restart, the data is in the new folder, the old one is gone and the pointer names the new one.
# Nothing outside a temporary directory is touched (never the user's ~/.iworkflow or configuration folder).
#
# Usage: end2end/go/move_home_smoke.sh      (needs go, git, curl, python3)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORK="$(mktemp -d)"
PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
BASE="http://127.0.0.1:$PORT"
# Build before isolating HOME, so Go keeps using the real module and build caches.
(cd "$ROOT" && go build -o "$WORK/iworkflow" ./cmd/iworkflow)
export HOME="$WORK/home" XDG_CONFIG_HOME="$WORK/home/.config" APPDATA="$WORK/home/.config" IWORKFLOW_ADDR="127.0.0.1:$PORT"
unset IWF_HOME
mkdir -p "$HOME"
OLD="$HOME/.iworkflow"
NEW="$WORK/elsewhere/iwf"
POINTER="$XDG_CONFIG_HOME/iworkflow/data-home"

finish() {
  local status=$?
  pkill -f "$WORK/iworkflow" 2>/dev/null || true
  if [ "$status" -ne 0 ]; then
    echo "--- server log ---" >&2
    cat "$WORK"/server.log "$NEW"/logs/iworkflow.log 2>/dev/null | tail -60 >&2 || true
  fi
  chmod -R u+w "$WORK" 2>/dev/null || true
  rm -rf "$WORK"
  exit "$status"
}
trap finish EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

wait_for_server() {
  for _ in $(seq 1 300); do
    curl -fs -o /dev/null "$BASE/data-home/card" && return 0
    sleep 0.2
  done
  fail "server did not answer on $BASE"
}

cd "$ROOT"
git init -q -b main "$WORK/repo"
git -C "$WORK/repo" -c user.email=t@t -c user.name=t commit -q --allow-empty -m init

(cd "$ROOT" && nohup "$WORK/iworkflow" --no-open >"$WORK/server.log" 2>&1 &)
wait_for_server
curl -fs -X POST --data-urlencode "name=smoke" --data-urlencode "path=$WORK/repo" "$BASE/workspaces/register" -o /dev/null || fail "register workspace"
mkdir -p "$OLD/attachments/t" && echo "keep me" >"$OLD/attachments/t/marker.txt"
curl -fs "$BASE/data-home/card" | grep -q "$OLD" || fail "the card does not show the current folder $OLD"

# A relative destination is refused and nothing is recorded.
curl -fs -X POST --data-urlencode "destination=relative/dir" "$BASE/data-home/move" | grep -qi "absolute" || fail "relative destination was not refused"
[ ! -e "$XDG_CONFIG_HOME/iworkflow/move-pending.json" ] || fail "a refused request left a pending move"

curl -fs -X POST --data-urlencode "destination=$NEW" "$BASE/data-home/move" -o /dev/null || fail "move request"
# The server shuts down, moves the folder and starts again from the new place.
for _ in $(seq 1 300); do
  [ "$(cat "$POINTER" 2>/dev/null || true)" = "$NEW" ] && break
  sleep 0.2
done
[ "$(cat "$POINTER" 2>/dev/null || true)" = "$NEW" ] || fail "the pointer does not name $NEW"
wait_for_server

[ -f "$NEW/workspaces.db" ] || fail "no database in the new folder"
[ "$(cat "$NEW/attachments/t/marker.txt")" = "keep me" ] || fail "the marker file was not copied"
[ ! -e "$OLD" ] || fail "the old folder $OLD still exists"
curl -fs "$BASE/data-home/card" | grep -q "$NEW" || fail "the card does not show the new folder"
curl -fs "$BASE/" -o /dev/null || fail "the app does not serve after the move"
echo "OK: the data folder moved from $OLD to $NEW"
