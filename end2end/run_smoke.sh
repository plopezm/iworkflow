#!/usr/bin/env bash
# Smoke de extremo a extremo con un agente falso (no consume modelo).
# Requiere cargo-leptos, python3 y Playwright (PLAYWRIGHT_MODULE=/ruta/a/node_modules/playwright).
# Opcional: SMOKE_CHROMIUM_PATH=/ruta/a/chrome si Playwright no tiene su Chromium descargado.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"
trap 'kill "${SERVER_PID:-}" 2>/dev/null || true; rm -rf "$WORK"' EXIT

cd "$ROOT"
cargo leptos build

# Puerto libre propio: nunca reutilizar un servidor de desarrollo ya en marcha (el smoke
# registra workspaces y cambia ajustes, y ensuciaría su base de datos).
PORT="$(python3 -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')"

# Un repo por smoke: el servidor rechaza registrar dos veces la misma ruta y comparten BD.
make_repo() {
  mkdir -p "$1"
  git -C "$1" init -q -b main
  echo hola > "$1/README.md"
  git -C "$1" add .
  git -C "$1" -c user.name=smoke -c user.email=smoke@localhost commit -qm init
}
REPO="$WORK/repo"
REPO_KEYS="$WORK/keys/project"
REPO_RECONNECT="$WORK/reconnect/project"
make_repo "$REPO"
make_repo "$REPO_KEYS"
make_repo "$REPO_RECONNECT"

IWF_DATABASE_URL="$WORK/db.sqlite" \
IWF_WORKTREES_DIR="$WORK/worktrees" \
IWF_GENTLE_SHELL_BIN="$ROOT/tests/fixtures/smoke_gentle_shell.sh" \
IWF_PI_BIN="$ROOT/tests/fixtures/smoke_pi.sh" \
LEPTOS_SITE_ROOT=target/site \
LEPTOS_SITE_ADDR="127.0.0.1:$PORT" \
  ./target/debug/iworkflow > "$WORK/server.log" 2>&1 &
SERVER_PID=$!
for _ in $(seq 1 50); do
  curl -fs "http://127.0.0.1:$PORT/" > /dev/null && break
  sleep 0.2
done

SMOKE_BASE_URL="http://127.0.0.1:$PORT" SMOKE_SCREENSHOT="$WORK/dock-review.png" node end2end/dock_smoke.cjs
SMOKE_BASE_URL="http://127.0.0.1:$PORT" SMOKE_REPO="$REPO_KEYS" node end2end/terminal_keys_smoke.cjs
SMOKE_BASE_URL="http://127.0.0.1:$PORT" SMOKE_REPO="$REPO_RECONNECT" node end2end/terminal_reconnect_smoke.cjs
SMOKE_BASE_URL="http://127.0.0.1:$PORT" SMOKE_REPO="$REPO" SMOKE_SCREENSHOT="$WORK/ui-review.png" node end2end/ui_smoke.cjs
git -C "$REPO" log --oneline -1 | grep -q "Merge tarea pi:" && echo "merge verificado en $REPO"
