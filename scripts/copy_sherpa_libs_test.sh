#!/usr/bin/env bash
# Tests of scripts/copy_sherpa_libs.sh: run with `scripts/copy_sherpa_libs_test.sh` (also part of `make test`).
set -uo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
failures=0

check() { # check <name> <condition exit code>
  if [ "$2" -eq 0 ]; then echo "ok   $1"; else echo "FAIL $1"; failures=$((failures + 1)); fi
}

# An empty library folder is an error and leaves nothing behind but the (empty) target.
mkdir "$work/empty"
SHERPA_LIBS_SOURCE="$work/empty" "$here/copy_sherpa_libs.sh" "$work/out-empty" >/dev/null 2>&1
check "an empty source folder fails" "$([ $? -ne 0 ]; echo $?)"

# A missing source folder is an error.
SHERPA_LIBS_SOURCE="$work/missing" "$here/copy_sherpa_libs.sh" "$work/out-missing" >/dev/null 2>&1
check "a missing source folder fails" "$([ $? -ne 0 ]; echo $?)"

# Libraries of the current OS are copied: .so on Linux, .dylib on macOS, .dll on Windows (next to the executable).
case "$(go env GOOS)" in darwin) ext=dylib; sub=lib ;; windows) ext=dll; sub=. ;; *) ext=so; sub=lib ;; esac
mkdir "$work/source"
touch "$work/source/liba.$ext" "$work/source/libb.$ext" "$work/source/notes.txt"
SHERPA_LIBS_SOURCE="$work/source" "$here/copy_sherpa_libs.sh" "$work/dist" >/dev/null 2>&1
check "libraries are copied" "$([ $? -eq 0 ] && [ -f "$work/dist/$sub/liba.$ext" ] && [ -f "$work/dist/$sub/libb.$ext" ]; echo $?)"
check "other files are not copied" "$([ ! -e "$work/dist/$sub/notes.txt" ]; echo $?)"

exit "$failures"
