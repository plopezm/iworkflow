#!/usr/bin/env bash
# Tests of scripts/stage_server.sh: run with `scripts/stage_server_test.sh` (also part of `make test`).
set -uo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
failures=0

check() { # check <name> <condition exit code>
  if [ "$2" -eq 0 ]; then echo "ok   $1"; else echo "FAIL $1"; failures=$((failures + 1)); fi
}

mkdir -p "$work/dist/site" "$work/dist/lib" "$work/dist/electron"
touch "$work/dist/iworkflow" "$work/dist/site/index.html" "$work/dist/lib/libsherpa.so" "$work/dist/a.dll" "$work/dist/electron/old.exe"
"$here/stage_server.sh" "$work/dist" "$work/out" iworkflow >/dev/null 2>&1
check "binary, site, lib and dlls are copied" "$([ $? -eq 0 ] && [ -f "$work/out/iworkflow" ] && [ -f "$work/out/site/index.html" ] && [ -f "$work/out/lib/libsherpa.so" ] && [ -f "$work/out/a.dll" ]; echo $?)"
check "other build output is not copied" "$([ ! -e "$work/out/electron" ]; echo $?)"

"$here/stage_server.sh" "$work/missing" "$work/out2" iworkflow >/dev/null 2>&1
check "a missing package fails" "$([ $? -ne 0 ]; echo $?)"

exit "$failures"
