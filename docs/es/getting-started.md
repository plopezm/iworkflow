# Arranque rápido

> 🇬🇧 [English version](../getting-started.md) · [← README](../../README.es.md)

Requisitos: `git`, Go (la versión de `go.mod`), `templ`, `tailwindcss` y al menos un CLI de agente instalado y con sesión iniciada (`gentle-shell`, `pi`, `claude`, `agent` para Cursor, o `agy` para Antigravity).

```bash
git clone https://github.com/plopezm/iworkflow.git
cd iworkflow
make dev                    # https://127.0.0.1:3000 (autofirmado: acepta el aviso del navegador una vez)
```

1. **Workspaces** → registra una carpeta con git (necesita al menos un commit).
2. **Tareas** → elige un workspace, escribe el prompt y lanza.
3. En el detalle de la tarea sigues al agente en vivo y puedes:
   - enviar mensajes (un prompt si está esperando; *steer* o *follow-up* si está trabajando),
   - responder a los diálogos del agente (`ask_user_question`, confirmaciones…),
   - abortar la ejecución en curso.
4. **Finalizar y revisar** → cierra el agente, hace commit de lo pendiente y muestra el diff.
5. **Merge** (integra con `--no-ff` y elimina el worktree) o **Descartar**.

> **Seguridad:** iworkflow solo se protege con un token de acceso opcional y lanza agentes que ejecutan comandos con tu usuario. No lo expongas a redes no confiables; consulta [Producción](production.md) y [Tailscale](mobile.md). El desarrollo escucha en `127.0.0.1:3000`; el script de producción, en `0.0.0.0:9000`.
