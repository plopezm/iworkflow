# pi-workflow

Dashboard web para lanzar y controlar instancias de `gentle-shell` (pi) en modo RPC.
Cada tarea corre en su propio **git worktree**; al terminar revisas el diff y decides si integrarla.

La app no impone ningún flujo: cada instancia usa la configuración y los plugins de
gentle-shell que tengas instalados y el `AGENTS.md` del proyecto.

## Arranque rápido

```bash
cargo leptos watch          # http://127.0.0.1:3000
```

1. **Workspaces** → registra una carpeta con git (necesita al menos un commit).
2. **Tareas** → elige workspace, escribe el prompt y lanza.
3. En el detalle de la tarea sigues al agente en vivo y puedes:
   - enviar mensajes (prompt si está esperando; *steer* o *follow-up* si está trabajando),
   - responder diálogos del agente (`ask_user_question`, confirmaciones…),
   - abortar la ejecución en curso.
4. **Finalizar y revisar** → cierra el agente, commitea lo pendiente y muestra el diff.
5. **Merge** (integra con `--no-ff` y borra el worktree) o **Descartar**.

## Ciclo de vida de una tarea

```text
starting → running ⇄ idle → finishing → awaiting_review → merged
                                              │               ↘ discarded
                                              ↓
              conflict · failed · interrupted ── Reanudar (gentle-shell --continue)
```

| Situación | Qué hace la app |
|---|---|
| Conflicto al integrar | `merge --abort`; la tarea pasa a `conflict` y conserva el worktree. Reanuda el agente para resolverlo. |
| Rama base con cambios sin commitear | Rechaza el merge sin tocar nada. |
| Rama base no checkouteada | Integra en un worktree temporal. |
| Reinicio del servidor | Las tareas activas pasan a `interrupted`; se pueden reanudar. |

## Configuración

Variables de entorno (admite `.env`, ver `.env.example`):

| Variable | Por defecto |
|---|---|
| `DATABASE_URL` | `~/.pi-dashboard/workspaces.db` (SQLite, migraciones automáticas) |
| `WORKTREES_DIR` | `~/.pi-dashboard/worktrees` |
| `GENTLE_SHELL_BIN` | `gentle-shell` (se lanza como `--mode rpc` con `GENTLE_SHELL_INTERACTIVE_HOST=1`) |

El servidor escucha solo en `127.0.0.1` y no tiene autenticación: está pensado para uso local.

## Desarrollo

Arquitectura y reglas en [AGENTS.md](AGENTS.md). Comprobaciones:

```bash
cargo fmt --all -- --check
cargo clippy --features ssr --all-targets -- -D warnings
cargo clippy --features hydrate --target wasm32-unknown-unknown --lib -- -D warnings
cargo test --features ssr
PLAYWRIGHT_MODULE=/ruta/node_modules/playwright ./end2end/run_smoke.sh   # UI con agente falso
```
