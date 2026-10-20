#!/usr/bin/env bash
# Bump the app version in Cargo.toml (+ Cargo.lock) and electron/package.json (+ package-lock.json).
#
#   ./scripts/bump-version.sh           # shows the current versions and asks for the new one
#   ./scripts/bump-version.sh 0.16.0    # non-interactive
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CARGO_TOML="$ROOT/Cargo.toml"
CARGO_LOCK="$ROOT/Cargo.lock"
PKG_JSON="$ROOT/electron/package.json"
PKG_LOCK="$ROOT/electron/package-lock.json"
SEMVER_RE='^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$'

# perl, not `sed -n '0,/re/...'`: BSD sed (macOS) does not support the `0,/re/` address.
cargo_version="$(perl -0ne 'print $1 if /^version *= *"(.*)"/m' "$CARGO_TOML")"
electron_version="$(perl -0ne 'print $1 if /^  "version": "(.*)",/m' "$PKG_JSON")"

echo "Current versions:"
echo "  Cargo.toml            $cargo_version"
echo "  electron/package.json $electron_version"

new_version="${1:-}"
if [[ -z "$new_version" ]]; then
  read -r -p "New version: " new_version
fi
if [[ ! "$new_version" =~ $SEMVER_RE ]]; then
  echo "Invalid version '$new_version' (expected MAJOR.MINOR.PATCH)" >&2
  exit 1
fi

# perl instead of `sed -i`: BSD sed (macOS) has no `0,/re/` address and a different `-i`.
# Without /g, `s///` replaces only the first match, which is what each file needs.
# Cargo.toml: first `version` line, which belongs to [package].
NEW="$new_version" perl -0pi -e 's/^version *= *".*"/version = "$ENV{NEW}"/m' "$CARGO_TOML"
# Cargo.lock: the entry of this crate only.
NEW="$new_version" perl -0pi -e 's/^(name = "iworkflow"\n)version = ".*"/$1version = "$ENV{NEW}"/m' "$CARGO_LOCK"
# electron/package.json: top-level version (2-space indent).
NEW="$new_version" perl -0pi -e 's/^  "version": ".*",/  "version": "$ENV{NEW}",/m' "$PKG_JSON"
# electron/package-lock.json: root version and the root package entry.
if [[ -f "$PKG_LOCK" ]]; then
  NEW="$new_version" perl -0pi -e 's/^  "version": ".*",/  "version": "$ENV{NEW}",/m; s/^      "version": ".*",/      "version": "$ENV{NEW}",/m' "$PKG_LOCK"
fi

echo "Bumped to $new_version:"
git -C "$ROOT" diff --stat -- Cargo.toml Cargo.lock electron/package.json electron/package-lock.json
