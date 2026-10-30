#!/usr/bin/env bash
# Helpers shared by scripts/prod.sh and scripts/electron.sh. Source it, do not run it.

# Name of the server executable for the current platform.
server_binary() {
  case "$(uname -s)" in
    MINGW* | MSYS* | CYGWIN*) echo "iworkflow.exe" ;;
    *) echo "iworkflow" ;;
  esac
}
