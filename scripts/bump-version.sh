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

cargo_version="$(sed -n '0,/^version *= *"\(.*\)"/s//\1/p' "$CARGO_TOML")"
electron_version="$(sed -n '0,/^  "version": "\(.*\)",/s//\1/p' "$PKG_JSON")"

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

# Cargo.toml: first `version` line, which belongs to [package].
sed -i '0,/^version *= *".*"/s//version = "'"$new_version"'"/' "$CARGO_TOML"
# Cargo.lock: the entry of this crate only.
sed -i '/^name = "iworkflow"$/{n;s/^version = ".*"/version = "'"$new_version"'"/}' "$CARGO_LOCK"
# electron/package.json: top-level version (2-space indent).
sed -i '0,/^  "version": ".*",/s//  "version": "'"$new_version"'",/' "$PKG_JSON"
# electron/package-lock.json: root version and the root package entry.
if [[ -f "$PKG_LOCK" ]]; then
  sed -i '0,/^  "version": ".*",/s//  "version": "'"$new_version"'",/' "$PKG_LOCK"
  sed -i '0,/^      "version": ".*",/s//      "version": "'"$new_version"'",/' "$PKG_LOCK"
fi

echo "Bumped to $new_version:"
git -C "$ROOT" diff --stat -- Cargo.toml Cargo.lock electron/package.json electron/package-lock.json
