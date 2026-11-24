#!/usr/bin/env bash
# Packages the Wails desktop shell together with the server staged by scripts/stage_server.sh, for one OS.
# Usage: scripts/wails_package.sh <goos> <goarch> <version> <desktop-binary> <server-dir> <out-dir>
#   goos:           linux | darwin | windows
#   goarch:         amd64 | arm64
#   desktop-binary: the built iworkflow-desktop executable (iworkflow-desktop.exe on Windows)
#   server-dir:     the folder written by stage_server.sh: server binary, site/ and its libraries (lib/ or *.dll)
#   out-dir:        receives the package. Scratch space comes from mktemp and is removed on exit.
# The desktop binary looks for server/ next to itself (internal/desktop ServerDir), so every layout puts it there.
#
# Output per OS:
#   linux   iworkflow-<version>-linux-<arch>.tar.gz: one folder with iworkflow-desktop, server/, iworkflow.png and
#           iworkflow.desktop. An AppImage is not built: `wails3 generate appimage` downloads linuxdeploy from its
#           mutable "continuous" tag and the type2 runtime without a checksum, which breaks the pinned-download rule.
#   darwin  iworkflow-<version>-darwin-<arch>.dmg (UNVERIFIED on macOS): iworkflow.app with the server in
#           Contents/MacOS/server, ad hoc signed like the Electron build (no Apple certificate).
#   windows iworkflow-<version>-windows-<arch>-setup.exe (UNVERIFIED on Windows): per-user NSIS installer
#           (cmd/iworkflow-desktop/build/windows/installer.nsi) with the data-folder page.
set -euo pipefail

usage="usage: wails_package.sh <goos> <goarch> <version> <desktop-binary> <server-dir> <out-dir>"
goos="${1:?$usage}"
goarch="${2:?$usage}"
version="${3:?$usage}"
desktop="${4:?$usage}"
server="${5:?$usage}"
out="${6:?$usage}"

here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/.." && pwd)"
build="$root/cmd/iworkflow-desktop/build"
icon="$root/cmd/iworkflow-desktop/icon.png"

case "$goos" in
  linux | darwin | windows) ;;
  *) echo "unsupported OS '$goos' (linux, darwin or windows)" >&2; exit 2 ;;
esac
case "$goarch" in
  amd64 | arm64) ;;
  *) echo "unsupported arch '$goarch' (amd64 or arm64)" >&2; exit 2 ;;
esac
echo "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$' ||
  { echo "invalid version '$version' (expected MAJOR.MINOR.PATCH)" >&2; exit 2; }
[ -f "$desktop" ] || { echo "No desktop binary at $desktop; build cmd/iworkflow-desktop first." >&2; exit 1; }
[ -d "$server/site" ] || { echo "No server folder at $server; run scripts/stage_server.sh first." >&2; exit 1; }
[ -f "$server/iworkflow" ] || [ -f "$server/iworkflow.exe" ] || { echo "No server binary in $server." >&2; exit 1; }

need() {
  command -v "$1" >/dev/null 2>&1 || { echo "$1 is required to package for $goos" >&2; exit 1; }
}

mkdir -p "$out"
out="$(cd "$out" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
name="iworkflow-$version-$goos-$goarch"

package_linux() {
  local top="$work/$name"
  mkdir -p "$top"
  install -m 755 "$desktop" "$top/iworkflow-desktop"
  cp -R "$server" "$top/server"
  install -m 644 "$icon" "$top/iworkflow.png"
  install -m 644 "$build/linux/iworkflow.desktop" "$top/iworkflow.desktop"
  tar -C "$work" -czf "$out/$name.tar.gz" "$name"
  echo "$out/$name.tar.gz"
}

# UNVERIFIED: not run on macOS yet.
package_darwin() {
  need hdiutil
  need codesign
  local app="$work/dmg/iworkflow.app"
  mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
  install -m 755 "$desktop" "$app/Contents/MacOS/iworkflow-desktop"
  cp -R "$server" "$app/Contents/MacOS/server"
  sed "s/@VERSION@/$version/g" "$build/darwin/Info.plist" > "$app/Contents/Info.plist"
  install -m 644 "$build/darwin/icon.icns" "$app/Contents/Resources/icon.icns"
  codesign --force --deep --sign - "$app"
  ln -s /Applications "$work/dmg/Applications"
  hdiutil create -volname "iworkflow $version" -srcfolder "$work/dmg" -ov -format UDZO "$out/$name.dmg" >/dev/null
  echo "$out/$name.dmg"
}

# makensis needs Windows paths when run from MSYS or Cygwin, so the paths are converted with cygpath first.
native_path() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -w "$1"; else printf '%s\n' "$1"; fi
}

# UNVERIFIED: not run on Windows yet.
package_windows() {
  need makensis
  local stage="$work/windows"
  mkdir -p "$stage"
  install -m 755 "$desktop" "$stage/iworkflow-desktop.exe"
  cp -R "$server" "$stage/server"
  makensis -V2 \
    -DVERSION="$version" -DARCH="$goarch" \
    -DSTAGE="$(native_path "$stage")" -DBUILD="$(native_path "$build")" -DOUTDIR="$(native_path "$out")" \
    "$(native_path "$build/windows/installer.nsi")" >/dev/null
  echo "$out/iworkflow-$version-windows-$goarch-setup.exe"
}

case "$goos" in
  linux) package_linux ;;
  darwin) package_darwin ;;
  windows) package_windows ;;
esac
