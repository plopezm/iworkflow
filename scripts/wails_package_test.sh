#!/usr/bin/env bash
# Tests of scripts/wails_package.sh: run with `scripts/wails_package_test.sh`. Not part of `make test`: packaging
# needs the full server build (make wails-package). The Linux archive is checked here; macOS and Windows packages
# only get their argument checks, and only on hosts without their tools.
set -uo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
failures=0
export TMPDIR="$work/tmp"
mkdir -p "$TMPDIR"

check() { # check <name> <exit code, 0 = pass>
  if [ "$2" -eq 0 ]; then echo "ok   $1"; else echo "FAIL $1"; failures=$((failures + 1)); fi
}
expect_ok() { # expect_ok <name> <command...>
  local name="$1"
  shift
  "$@" >/dev/null 2>&1
  check "$name" $?
}
expect_fail() { # expect_fail <name> <command...>
  local name="$1"
  shift
  if "$@" >/dev/null 2>&1; then check "$name" 1; else check "$name" 0; fi
}

# A fake desktop binary and a server folder shaped like the output of stage_server.sh.
printf '#!/bin/sh\nexit 0\n' > "$work/iworkflow-desktop"
chmod 755 "$work/iworkflow-desktop"
mkdir -p "$work/server/site" "$work/server/lib" "$work/server-nobin/site"
printf 'server\n' > "$work/server/iworkflow"
chmod 755 "$work/server/iworkflow"
printf '<html></html>\n' > "$work/server/site/index.html"
touch "$work/server/lib/libsherpa.so"
touch "$work/server-nobin/site/index.html"

pkg="$here/wails_package.sh"
top="iworkflow-1.2.3-linux-amd64"
archive="$work/out/$top.tar.gz"

expect_ok "linux package is written" "$pkg" linux amd64 1.2.3 "$work/iworkflow-desktop" "$work/server" "$work/out"
[ -f "$archive" ]
check "archive exists at out/$top.tar.gz" $?

list="$(tar -tzf "$archive" 2>/dev/null)"
for entry in "$top/iworkflow-desktop" "$top/server/iworkflow" "$top/server/site/index.html" \
  "$top/server/lib/libsherpa.so" "$top/iworkflow.png" "$top/iworkflow.desktop"; do
  printf '%s\n' "$list" | grep -qx "$entry"
  check "archive has $entry" $?
done

verbose="$(tar -tvzf "$archive" 2>/dev/null)"
printf '%s\n' "$verbose" | grep -Eq "^-rwxr-xr-x .* $top/iworkflow-desktop$"
check "desktop binary is executable" $?

desktop_file="$(tar -xzOf "$archive" "$top/iworkflow.desktop" 2>/dev/null)"
printf '%s\n' "$desktop_file" | grep -qx 'Exec=iworkflow-desktop'
check ".desktop launches iworkflow-desktop" $?

stray="$(printf '%s\n' "$list" | grep -v "^$top/" || true)"
[ -z "$stray" ]
check "archive holds only the $top folder" $?

leftover="$(ls -A "$TMPDIR")"
[ -z "$leftover" ]
check "scratch space is removed" $?

expect_fail "a missing server folder fails" "$pkg" linux amd64 1.2.3 "$work/iworkflow-desktop" "$work/missing" "$work/out2"
expect_fail "a server folder without its binary fails" "$pkg" linux amd64 1.2.3 "$work/iworkflow-desktop" "$work/server-nobin" "$work/out2"
expect_fail "a missing desktop binary fails" "$pkg" linux amd64 1.2.3 "$work/missing-desktop" "$work/server" "$work/out2"
expect_fail "an unsupported OS fails" "$pkg" freebsd amd64 1.2.3 "$work/iworkflow-desktop" "$work/server" "$work/out2"
expect_fail "an unsupported arch fails" "$pkg" linux 386 1.2.3 "$work/iworkflow-desktop" "$work/server" "$work/out2"
expect_fail "a version that is not MAJOR.MINOR.PATCH fails" "$pkg" linux amd64 v1 "$work/iworkflow-desktop" "$work/server" "$work/out2"
expect_fail "missing arguments fail" "$pkg" linux amd64 1.2.3

if ! command -v hdiutil >/dev/null 2>&1; then
  expect_fail "a macOS package needs hdiutil on this host" "$pkg" darwin arm64 1.2.3 "$work/iworkflow-desktop" "$work/server" "$work/out2"
fi
if ! command -v makensis >/dev/null 2>&1; then
  expect_fail "a Windows package needs makensis on this host" "$pkg" windows amd64 1.2.3 "$work/iworkflow-desktop" "$work/server" "$work/out2"
fi

exit "$failures"
