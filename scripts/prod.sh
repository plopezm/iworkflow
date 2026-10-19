#!/usr/bin/env bash
# Build and run iworkflow in release mode.
#
#   ./scripts/prod.sh build   # cargo leptos build --release + package into $DIST_DIR
#   ./scripts/prod.sh run     # start the packaged binary
#   ./scripts/prod.sh         # build + run
#
# Environment:
#   DIST_DIR          output directory (default: <repo>/dist)
#   LEPTOS_SITE_ADDR  listen address (default: 0.0.0.0:9000, all interfaces)
# The server also reads IWF_DATABASE_URL, IWF_WORKTREES_DIR, IWF_GENTLE_SHELL_BIN and a
# `.env` file from the current directory.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST_DIR="${DIST_DIR:-$ROOT/dist}"
BIN_NAME="iworkflow"

build() {
  cd "$ROOT"
  cargo leptos build --release
  rm -rf "$DIST_DIR"
  mkdir -p "$DIST_DIR"
  cp "target/release/$BIN_NAME" "$DIST_DIR/"
  cp -r target/site "$DIST_DIR/site"
  echo "Packaged into $DIST_DIR"
}

run() {
  DIST_DIR="$(cd "$DIST_DIR" 2>/dev/null && pwd)" || {
    echo "No build found in $DIST_DIR; run '$0 build' first." >&2
    exit 1
  }
  if [[ ! -x "$DIST_DIR/$BIN_NAME" || ! -d "$DIST_DIR/site" ]]; then
    echo "Incomplete build in $DIST_DIR; run '$0 build' first." >&2
    exit 1
  fi
  export LEPTOS_OUTPUT_NAME="$BIN_NAME"
  export LEPTOS_SITE_ROOT="$DIST_DIR/site"
  export LEPTOS_SITE_PKG_DIR="pkg"
  export LEPTOS_SITE_ADDR="${LEPTOS_SITE_ADDR:-0.0.0.0:9000}"
  export LEPTOS_ENV="PROD"
  echo "Serving on http://$LEPTOS_SITE_ADDR"
  exec "$DIST_DIR/$BIN_NAME" "$@"
}

case "${1:-all}" in
  build) build ;;
  run) shift; run "$@" ;;
  all) build && run ;;
  *)
    echo "Usage: $0 [build|run|all]" >&2
    exit 2
    ;;
esac
