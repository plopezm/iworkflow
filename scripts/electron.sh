#!/usr/bin/env bash
# Package iworkflow as a desktop app (Electron) for the current platform.
#
#   ./scripts/electron.sh build   # release server build + installer in $DIST_DIR/electron
#   ./scripts/electron.sh dev     # open the Electron window on the packaged server in $DIST_DIR
#   ./scripts/electron.sh         # build
#
# Environment:
#   DIST_DIR            server package produced by scripts/prod.sh (default: <repo>/dist)
#   SKIP_SERVER_BUILD   set to 1 to reuse an existing $DIST_DIR instead of rebuilding it
# The installer launches the server on 127.0.0.1:3000 like the plain binary does: the saved
# network-access setting, LEPTOS_SITE_ADDR and the access token keep working unchanged.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST_DIR="${DIST_DIR:-$ROOT/dist}"
ELECTRON_DIR="$ROOT/electron"

server_binary() {
  case "$(uname -s)" in
    MINGW* | MSYS* | CYGWIN*) echo "iworkflow.exe" ;;
    *) echo "iworkflow" ;;
  esac
}

builder_platform() {
  case "$(uname -s)" in
    Linux) echo "--linux" ;;
    MINGW* | MSYS* | CYGWIN*) echo "--win" ;;
    *)
      echo "Unsupported platform: $(uname -s) (Linux and Windows only)." >&2
      exit 1
      ;;
  esac
}

require_server_package() {
  local bin
  bin="$(server_binary)"
  if [[ ! -f "$DIST_DIR/$bin" || ! -d "$DIST_DIR/site" ]]; then
    echo "No server package in $DIST_DIR; run scripts/prod.sh build first." >&2
    exit 1
  fi
}

build() {
  if [[ "${SKIP_SERVER_BUILD:-0}" != "1" ]]; then
    DIST_DIR="$DIST_DIR" "$ROOT/scripts/prod.sh" build
  fi
  require_server_package
  rm -rf "$ELECTRON_DIR/resources/server"
  mkdir -p "$ELECTRON_DIR/resources/server"
  cp "$DIST_DIR/$(server_binary)" "$ELECTRON_DIR/resources/server/"
  cp -r "$DIST_DIR/site" "$ELECTRON_DIR/resources/server/site"
  cd "$ELECTRON_DIR"
  npm ci
  npx electron-builder "$(builder_platform)" --publish never \
    --config.directories.output="$DIST_DIR/electron"
  echo "Installer written to $DIST_DIR/electron"
}

dev() {
  require_server_package
  cd "$ELECTRON_DIR"
  npm ci
  IWF_SERVER_DIR="$DIST_DIR" npx electron .
}

case "${1:-build}" in
  build) build ;;
  dev) dev ;;
  *)
    echo "Usage: $0 [build|dev]" >&2
    exit 2
    ;;
esac
