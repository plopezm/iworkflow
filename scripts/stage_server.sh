#!/usr/bin/env bash
# Copies a packaged server (see the dist target of the Makefile) into a folder that ships it.
# Usage: scripts/stage_server.sh <dist-dir> <dest-dir> <binary-name>
# The binary, site/ and the sherpa-onnx libraries it loads at start travel together: lib/ on Linux and macOS,
# *.dll next to the binary on Windows. A package without them dies at load time (0xC0000135 on Windows).
set -euo pipefail

src="${1:?usage: stage_server.sh <dist-dir> <dest-dir> <binary-name>}"
dest="${2:?usage: stage_server.sh <dist-dir> <dest-dir> <binary-name>}"
bin="${3:?usage: stage_server.sh <dist-dir> <dest-dir> <binary-name>}"

[ -f "$src/$bin" ] && [ -d "$src/site" ] || { echo "No server package in $src; run 'make dist' first." >&2; exit 1; }
mkdir -p "$dest"
cp "$src/$bin" "$dest"/
cp -r "$src/site" "$dest/site"
if [ -d "$src/lib" ]; then cp -r "$src/lib" "$dest/lib"; fi
find "$src" -maxdepth 1 -name '*.dll' -exec cp {} "$dest"/ \;
