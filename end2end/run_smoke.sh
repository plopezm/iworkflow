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

REPO="$WORK/repo"
mkdir -p "$REPO"
git -C "$REPO" init -q -b main
echo hola > "$REPO/README.md"
git -C "$REPO" add .
git -C "$REPO" -c user.name=smoke -c user.email=smoke@localhost commit -qm init

PIWF_DATABASE_URL="$WORK/db.sqlite" \
PIWF_WORKTREES_DIR="$WORK/worktrees" \
PIWF_GENTLE_SHELL_BIN="$ROOT/tests/fixtures/smoke_gentle_shell.sh" \
PIWF_PI_BIN="$ROOT/tests/fixtures/smoke_pi.sh" \
LEPTOS_SITE_ROOT=target/site \
LEPTOS_SITE_ADDR="127.0.0.1:$PORT" \
  ./target/debug/pi-workflow > "$WORK/server.log" 2>&1 &
SERVER_PID=$!
for _ in $(seq 1 50); do
  curl -fs "http://127.0.0.1:$PORT/" > /dev/null && break
  sleep 0.2
done

SMOKE_BASE_URL="http://127.0.0.1:$PORT" SMOKE_REPO="$REPO" SMOKE_SCREENSHOT="$WORK/ui-review.png" node end2end/ui_smoke.cjs
git -C "$REPO" log --oneline -1 | grep -q "Merge tarea pi:" && echo "merge verificado en $REPO"
