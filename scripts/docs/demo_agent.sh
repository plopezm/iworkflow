#!/bin/sh
# Stand-in for pi and gentle-shell used by scripts/docs/record_gifs.sh: a real edit in the cwd and
# streamed thinking, text and tool calls (scripts/docs/demo_agent.py). Not used by the test suite.
exec python3 -u "$(dirname "$0")/demo_agent.py" "$@"
