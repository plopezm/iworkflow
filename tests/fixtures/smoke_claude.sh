#!/bin/sh
# Sustituto de `claude` para el smoke: habla stream-json con el agente falso.
exec python3 -u "$(dirname "$0")/fake_claude.py" "$@"
