#!/usr/bin/env bash
# Regenerates the documentation GIFs in docs/media/ from the real app.
#
# It builds the server and runs scripts/docs/record_gifs.cjs through end2end/go/run_go.sh, which gives the
# recording an isolated HOME and IWF_HOME in a temporary directory, the seeded usage bar (Claude 13 % / 25 %, Codex
# 75 % / 95 %) and the mocks. The server runs scripts/docs/demo_agent.sh instead of a model: it streams thinking and
# text, makes tool calls and really edits the task worktree (src/<slug>.js and a README line). The browser is forced
# to English (iw-lang=en). Nothing touches the real ~/.iworkflow.
#
# Usage:
#   scripts/docs/record_gifs.sh                       # all scenes -> docs/media/*.gif
#   DOCS_GIF_SCENES=task-hooks scripts/docs/record_gifs.sh   # one scene
#
# Environment:
#   DOCS_GIF_OUT      where the .webm recordings are kept (default /tmp/iwf-docs-gifs). Each run writes to its own
#                     subdirectory, so the recordings survive the removal of the run's scratch directory.
#   DOCS_GIF_SCENES   comma-separated scene names to record (default: all).
#   PLAYWRIGHT_MODULE, PLAYWRIGHT_BROWSERS_PATH, SMOKE_CHROMIUM_PATH   as for end2end/go/run_go.sh.
#
# Requires: go, node, python3, ffmpeg, tailwindcss (only when web/static/app.css is missing), a Playwright install
# with its Chromium build.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT_ROOT="${DOCS_GIF_OUT:-/tmp/iwf-docs-gifs}"
RUN_OUT="$OUT_ROOT/$(date +%Y%m%d-%H%M%S)-$$"
MEDIA="$ROOT/docs/media"
# 8 fps at 900 px keeps every GIF under 3 MB for its length (the 960 px / 10 fps variant made 4-6 MB).
GIF_FILTER='fps=8,scale=900:-1:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=96[p];[s1][p]paletteuse=dither=bayer:bayer_scale=4'

command -v ffmpeg >/dev/null 2>&1 || { echo "ffmpeg is required to convert the recordings" >&2; exit 1; }
mkdir -p "$RUN_OUT" "$MEDIA"

# The recording runs in English whatever the default language of the app is.
export SMOKE_LANG=en
export DOCS_GIF_OUT="$RUN_OUT"
export DEMO_AGENT="$ROOT/scripts/docs/demo_agent.sh"

status=0
IWF_E2E_PI_BIN="$DEMO_AGENT" \
IWF_E2E_GENTLE_SHELL_BIN="$DEMO_AGENT" \
IWF_E2E_SCRIPTS="scripts/docs/record_gifs.cjs" \
  "$ROOT/end2end/go/run_go.sh" || status=$?

shopt -s nullglob
converted=0
for webm in "$RUN_OUT"/*.webm; do
  name="$(basename "$webm" .webm)"
  # Failed scenes keep a partial recording that is not a documentation GIF.
  case "$name" in *.failed) continue ;; esac
  gif="$MEDIA/$name.gif"
  ffmpeg -v error -y -i "$webm" -an -filter_complex "$GIF_FILTER" -loop 0 "$gif"
  echo "▶ $gif ($(du -h "$gif" | cut -f1))"
  converted=$((converted + 1))
done
shopt -u nullglob

echo "▶ converted $converted recording(s) from $RUN_OUT"
if [ "$status" -ne 0 ]; then
  echo "▶ the recorder reported failures (exit $status); failed scenes are kept as *.failed.webm in $RUN_OUT" >&2
fi
exit "$status"
