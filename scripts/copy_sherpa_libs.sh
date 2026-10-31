#!/usr/bin/env bash
# Copies the sherpa-onnx shared libraries of the current OS and architecture next to a packaged server.
# Usage: scripts/copy_sherpa_libs.sh <dist-dir>
#   linux, darwin: <dist-dir>/lib (the binary finds them through its rpath, see the dist target of the Makefile)
#   windows:       <dist-dir> itself (Windows loads the DLLs from the folder of the executable)
set -euo pipefail

dist="${1:?usage: copy_sherpa_libs.sh <dist-dir>}"
goos="$(go env GOOS)"
goarch="$(go env GOARCH)"

case "$goarch" in
  amd64) arch=x86_64 ;;
  arm64) arch=aarch64 ;;
  386) arch=i686 ;;
  arm) arch=arm ;;
  *) echo "no sherpa-onnx libraries for GOARCH=$goarch" >&2; exit 1 ;;
esac

case "$goos" in
  linux) module=github.com/k2-fsa/sherpa-onnx-go-linux; triple="$arch-unknown-linux-gnu"; pattern='*.so'; target="$dist/lib" ;;
  darwin) module=github.com/k2-fsa/sherpa-onnx-go-macos; triple="$arch-apple-darwin"; pattern='*.dylib'; target="$dist/lib" ;;
  windows) module=github.com/k2-fsa/sherpa-onnx-go-windows; triple="$arch-pc-windows-gnu"; pattern='*.dll'; target="$dist" ;;
  *) echo "no sherpa-onnx libraries for GOOS=$goos" >&2; exit 1 ;;
esac

source_dir="$(go list -m -f '{{.Dir}}' "$module")/lib/$triple"
[ -d "$source_dir" ] || { echo "missing $source_dir" >&2; exit 1; }
mkdir -p "$target"
# -L follows symlinks, so the package holds real files.
find "$source_dir" -maxdepth 1 -name "$pattern" -exec cp -L {} "$target"/ \;
