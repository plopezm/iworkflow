#!/bin/sh
# Sustituto de `pi` para pruebas: igual que smoke_gentle_shell.sh pero deja una marca
# distinta, para comprobar qué binario lanzó el runtime elegido en la UI.
echo "cambio del agente pi" > CAMBIO.md
exec python3 -u "$(dirname "$0")/fake_agent.py" "$@"
