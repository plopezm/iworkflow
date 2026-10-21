#!/bin/sh
# Sustituto de `agent` (Cursor) para el smoke: habla ACP con el agente falso.
exec python3 -u "$(dirname "$0")/fake_cursor.py" "$@"
