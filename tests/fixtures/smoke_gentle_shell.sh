#!/bin/sh
# Sustituto de gentle-shell para pruebas: escribe un cambio en el worktree (cwd)
# y habla el protocolo RPC mediante fake_agent.py.
echo "cambio del agente" > CAMBIO.md
exec python3 -u "$(dirname "$0")/fake_agent.py" "$@"
