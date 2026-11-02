#!/usr/bin/env bash
# Copies the sherpa-onnx shared libraries of the current OS and architecture next to a packaged server.
# Usage: scripts/copy_sherpa_libs.sh <dist-dir>
#   SHERPA_LIBS_SOURCE=<dir> reads the libraries from <dir> instead of the Go module (used by the tests).
#   linux, darwin: <dist-dir>/lib (the binary finds them through its rpath, see the dist target of the Makefile)
#   windows:       <dist-dir> itself (Windows loads the DLLs from the folder of the executable), plus the MinGW
#                  runtime DLLs the sherpa-onnx ones import (found through gcc; MINGW_RUNTIME_DIR overrides the lookup).
#                  A machine without MinGW cannot start the server without them (exit code 0xC0000135).
#   COPY_LIBS_GOOS=<os> overrides the detected OS (used by the tests).
set -euo pipefail

dist="${1:?usage: copy_sherpa_libs.sh <dist-dir>}"
goos="${COPY_LIBS_GOOS:-$(go env GOOS)}"
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

source_dir="${SHERPA_LIBS_SOURCE:-$(go list -m -f '{{.Dir}}' "$module")/lib/$triple}"
[ -d "$source_dir" ] || { echo "missing $source_dir" >&2; exit 1; }
mkdir -p "$target"
# -L follows symlinks, so the package holds real files. An empty folder is an error: a package without the libraries
# starts and then fails to load the recognizer.
copied=0
while IFS= read -r -d '' library; do
  cp -L "$library" "$target"/
  copied=$((copied + 1))
done < <(find "$source_dir" -maxdepth 1 -name "$pattern" -print0)
[ "$copied" -gt 0 ] || { echo "no $pattern libraries in $source_dir" >&2; exit 1; }
echo "copied $copied sherpa-onnx libraries to $target"

if [ "$goos" = windows ]; then
  for runtime in libstdc++-6.dll libgcc_s_seh-1.dll libwinpthread-1.dll; do
    if [ -n "${MINGW_RUNTIME_DIR:-}" ]; then
      found="$MINGW_RUNTIME_DIR/$runtime"
    else
      found="$(gcc -print-file-name="$runtime")"
    fi
    # gcc echoes the bare name back when it cannot find the file.
    [ -f "$found" ] || { echo "MinGW runtime $runtime not found ($found)" >&2; exit 1; }
    cp -L "$found" "$target"/
  done
  echo "copied the MinGW runtime to $target"
fi
