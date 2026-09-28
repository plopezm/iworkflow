#!/usr/bin/env bash
# Smoke de extremo a extremo con un agente falso (no consume modelo).
# Requiere cargo-leptos, python3 y Playwright (PLAYWRIGHT_MODULE=/ruta/a/node_modules/playwright).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"
trap 'kill "${SERVER_PID:-}" 2>/dev/null || true; rm -rf "$WORK"' EXIT

cd "$ROOT"
cargo leptos build

REPO="$WORK/repo"
mkdir -p "$REPO"
git -C "$REPO" init -q -b main
echo hola > "$REPO/README.md"
git -C "$REPO" add .
git -C "$REPO" -c user.name=smoke -c user.email=smoke@localhost commit -qm init

DATABASE_URL="$WORK/db.sqlite" \
WORKTREES_DIR="$WORK/worktrees" \
GENTLE_SHELL_BIN="$ROOT/tests/fixtures/smoke_gentle_shell.sh" \
LEPTOS_SITE_ROOT=target/site \
  ./target/debug/pi-workflow > "$WORK/server.log" 2>&1 &
SERVER_PID=$!
sleep 2

SMOKE_REPO="$REPO" SMOKE_SCREENSHOT="$WORK/ui-review.png" node end2end/ui_smoke.cjs
git -C "$REPO" log --oneline -1 | grep -q "Merge tarea pi:" && echo "merge verificado en $REPO"
