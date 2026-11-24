#!/usr/bin/env bash
# Packages the Wails desktop shell, the update helper and the server staged by scripts/stage_server.sh, for one OS.
# Usage: scripts/wails_package.sh <goos> <goarch> <version> <desktop-binary> <updater-binary> <server-dir> <out-dir>
#   goos:           linux | darwin | windows
#   goarch:         amd64 | arm64
#   desktop-binary: the built iworkflow-desktop executable (iworkflow-desktop.exe on Windows)
#   updater-binary: the built iworkflow-updater helper from cmd/iworkflow-updater (iworkflow-updater.exe on Windows)
#   server-dir:     the folder written by stage_server.sh: server binary, site/ and its libraries (lib/ or *.dll)
#   out-dir:        receives the packages and SHA256SUMS. Scratch space comes from mktemp and is removed on exit.
# The desktop binary looks for server/ next to itself (internal/desktop ServerDir), so every layout puts it there.
# The update helper sits next to the desktop binary in every layout: it replaces the same folder on update.
#
# Output per OS:
#   linux   iworkflow-<version>-linux-<arch>.tar.gz: one folder with iworkflow-desktop, iworkflow-updater, server/,
#           iworkflow.png and iworkflow.desktop. An AppImage is not built: `wails3 generate appimage` downloads linuxdeploy
#           from its mutable "continuous" tag and the type2 runtime without a checksum, which breaks the pinned-download rule.
#           This archive is also the update asset for linux.
#   darwin  iworkflow-<version>-darwin-<arch>.dmg (UNVERIFIED on macOS): iworkflow.app with the server and
#           iworkflow-updater in Contents/MacOS/, ad hoc signed like the Electron build (no Apple certificate).
#           iworkflow-<version>-darwin-<arch>.tar.gz (UNVERIFIED on macOS): the update asset, one top folder iworkflow.app.
#   windows iworkflow-<version>-windows-<arch>-setup.exe (UNVERIFIED on Windows): per-user NSIS installer
#           (cmd/iworkflow-desktop/build/windows/installer.nsi) with the data-folder page.
#           iworkflow-<version>-windows-<arch>.tar.gz (UNVERIFIED on Windows): the update asset, one top folder holding
#           the install folder contents (iworkflow-desktop.exe, iworkflow-updater.exe, server/).
#   all     SHA256SUMS: one "<sha256>  <file>" line per file written by this run, checkable with `sha256sum -c`.
set -euo pipefail

usage="usage: wails_package.sh <goos> <goarch> <version> <desktop-binary> <updater-binary> <server-dir> <out-dir>"
goos="${1:?$usage}"
goarch="${2:?$usage}"
version="${3:?$usage}"
desktop="${4:?$usage}"
updater="${5:?$usage}"
server="${6:?$usage}"
out="${7:?$usage}"

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
[ -f "$updater" ] || { echo "No updater binary at $updater; build cmd/iworkflow-updater first." >&2; exit 1; }
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
produced=()

# record <file name>: notes a file written to $out and prints its path.
record() {
  produced+=("$1")
  echo "$out/$1"
}

package_linux() {
  local top="$work/$name"
  mkdir -p "$top"
  install -m 755 "$desktop" "$top/iworkflow-desktop"
  install -m 755 "$updater" "$top/iworkflow-updater"
  cp -R "$server" "$top/server"
  install -m 644 "$icon" "$top/iworkflow.png"
  install -m 644 "$build/linux/iworkflow.desktop" "$top/iworkflow.desktop"
  tar -C "$work" -czf "$out/$name.tar.gz" "$name"
  record "$name.tar.gz"
}

# UNVERIFIED: not run on macOS yet.
package_darwin() {
  need hdiutil
  need codesign
  local app="$work/dmg/iworkflow.app"
  mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
  install -m 755 "$desktop" "$app/Contents/MacOS/iworkflow-desktop"
  install -m 755 "$updater" "$app/Contents/MacOS/iworkflow-updater"
  cp -R "$server" "$app/Contents/MacOS/server"
  sed "s/@VERSION@/$version/g" "$build/darwin/Info.plist" > "$app/Contents/Info.plist"
  install -m 644 "$build/darwin/icon.icns" "$app/Contents/Resources/icon.icns"
  codesign --force --deep --sign - "$app"
  # The update asset: the signed bundle is the single top folder, and the updater replaces it on update.
  tar -C "$work/dmg" -czf "$out/$name.tar.gz" iworkflow.app
  record "$name.tar.gz"
  ln -s /Applications "$work/dmg/Applications"
  hdiutil create -volname "iworkflow $version" -srcfolder "$work/dmg" -ov -format UDZO "$out/$name.dmg" >/dev/null
  record "$name.dmg"
}

# makensis needs Windows paths when run from MSYS or Cygwin, so the paths are converted with cygpath first.
native_path() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -w "$1"; else printf '%s\n' "$1"; fi
}

# UNVERIFIED: not run on Windows yet.
package_windows() {
  need makensis
  # The install folder is staged under the update asset's top folder, so the installer and the update archive take the same files.
  local stage="$work/update/$name"
  mkdir -p "$stage"
  install -m 755 "$desktop" "$stage/iworkflow-desktop.exe"
  install -m 755 "$updater" "$stage/iworkflow-updater.exe"
  cp -R "$server" "$stage/server"
  makensis -V2 \
    -DVERSION="$version" -DARCH="$goarch" \
    -DSTAGE="$(native_path "$stage")" -DBUILD="$(native_path "$build")" -DOUTDIR="$(native_path "$out")" \
    "$(native_path "$build/windows/installer.nsi")" >/dev/null
  record "iworkflow-$version-windows-$goarch-setup.exe"
  tar -C "$work/update" -czf "$out/$name.tar.gz" "$name"
  record "$name.tar.gz"
}

case "$goos" in
  linux) package_linux ;;
  darwin) package_darwin ;;
  windows) package_windows ;;
esac

# SHA256SUMS covers the files this run wrote. shasum -a 256 is the fallback where sha256sum is missing (macOS).
sums="$work/SHA256SUMS"
(
  cd "$out"
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "${produced[@]}"; else shasum -a 256 "${produced[@]}"; fi
) > "$sums"
cp "$sums" "$out/SHA256SUMS"
echo "$out/SHA256SUMS"
