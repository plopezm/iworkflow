# iworkflow

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
| `IWF_DATABASE_URL` | `~/.iworkflow/workspaces.db` (SQLite, migraciones automáticas) |
| `IWF_WORKTREES_DIR` | `~/.iworkflow/worktrees` |
| `IWF_GENTLE_SHELL_BIN` | `gentle-shell` (se lanza como `--mode rpc` con `GENTLE_SHELL_INTERACTIVE_HOST=1`) |
| `IWF_PI_BIN` | `pi` (runtime `pi`, se lanza como `--mode rpc`) |
| `IWF_CLAUDE_BIN` | `claude` (runtime `claude`: Claude Code con `-p` y protocolo stream-json; se elige en el selector de runtime del menú lateral; acepta ediciones de ficheros, pide confirmación para el resto y lee el `AGENTS.md` del proyecto) |
| `IWF_GENTLE_PI_CONFIG_HOME` | `~/.pi/gentle-ai` (contiene `profiles.json` y `models.json`) |
| `IWF_PI_AGENTS_DIR` | `~/.pi/agent/agents` (un `<rol>.md` por rol de agente) |

## Integración con GitLab (on-premise)

Desde cada tarea puedes crear un merge request, ver el CI y mezclarlo.

1. En **Configuración → GitLab** indica la URL de tu instancia (p. ej. `https://gitlab.miempresa.local`) y un token de acceso personal con scope `api`. Usa "Probar conexión" para validarlos. Si tu certificado es de una CA interna o autofirmado, marca "Aceptar certificado TLS no confiable".
2. El proyecto se deduce del remoto `origin` del repositorio (https, ssh o scp); su host debe coincidir con la URL configurada.
3. En el panel de la tarea, pestaña **GitLab**: crear el MR (rama de la tarea → rama base), ver su estado y el último pipeline con sus jobs, y **Mezclar MR** (pide confirmación).

Notas:
- La app **nunca hace `git push`**: publica tú la rama antes de crear el MR; si no está en `origin`, se te avisa.
- El token se guarda en claro en la base de datos local (`~/.iworkflow/workspaces.db`) y nunca se envía al navegador.
- Las redirecciones HTTP están desactivadas para no filtrar el token a otro host.

## Perfiles y modelos de agentes

Dos pantallas equivalentes a los comandos de gentle-pi, sobre los mismos ficheros:

| Pantalla | Equivale a | Qué hace |
|---|---|---|
| `/profiles` | `/gentle:profiles` | Crear, duplicar, renombrar, eliminar, importar y exportar perfiles (`profiles.json`), editar modelo y esfuerzo por agente y crear roles de agente nuevos. |
| `/models` | `/gentle:models` | Editar el routing global (`models.json`): modelo y esfuerzo por agente. |

Los perfiles se eligen en el desplegable de ajustes del agente, que lee el mismo `profiles.json`. Estas pantallas no activan un perfil de forma global; solo lo gestionan. El bloqueo de escritura evita pisadas dentro de un proceso, no frente a otro proceso (p. ej. el TUI de gentle-pi) editando a la vez.

En desarrollo (`cargo leptos watch`) el servidor escucha solo en `127.0.0.1`.

### Argumentos del ejecutable

Al arrancar, `iworkflow` abre la web en el navegador (`http://<host>:<puerto>` del servidor; si escucha en `0.0.0.0` se usa `127.0.0.1`).

| Argumento | Efecto |
|---|---|
| `--no-open` | Arranca solo el servidor, sin abrir el navegador. |
| `-h`, `--help` | Muestra la ayuda. |

Ejemplos: `./iworkflow --no-open`, `./scripts/prod.sh run --no-open`, `cargo leptos watch -- --no-open`.

## Producción

```bash
./scripts/prod.sh           # compila en release, empaqueta en dist/ y arranca
./scripts/prod.sh build     # solo compila y empaqueta
./scripts/prod.sh run       # solo arranca lo ya empaquetado
```

`build` ejecuta `cargo leptos build --release` y copia en `dist/` el binario
`iworkflow` y la carpeta `site/` (WASM, JS y CSS). Ambos son necesarios: sin
`site/` la página se sirve pero no hidrata.

`run` exporta las variables de Leptos (`LEPTOS_SITE_ROOT`, `LEPTOS_ENV=PROD`…) y
arranca el binario desde el directorio actual, así que el `.env` de ese directorio
también se aplica. Variables propias del script:

| Variable | Por defecto |
|---|---|
| `DIST_DIR` | `dist/` en la raíz del repo |
| `LEPTOS_SITE_ADDR` | `0.0.0.0:3000` (todas las interfaces) |

> **Seguridad:** la app no tiene autenticación y lanza agentes que ejecutan comandos
> con tu usuario. Con el bind a `0.0.0.0`, cualquiera que alcance el puerto puede
> usarla. Restringe el acceso con firewall, red privada (Tailscale/WireGuard) o un
> reverse proxy con autenticación y TLS. Para limitarlo a la máquina local:
> `LEPTOS_SITE_ADDR=127.0.0.1:3000 ./scripts/prod.sh run`.

Para desplegar en otra máquina basta con copiar `dist/` y lanzarlo con
`LEPTOS_SITE_ROOT=site LEPTOS_ENV=PROD LEPTOS_SITE_ADDR=0.0.0.0:3000 ./iworkflow`
desde dentro de esa carpeta.

## Binarios precompilados (GitHub Actions)

El workflow `.github/workflows/release.yml` compila en Linux (`ubuntu-22.04`) y en
Windows (`windows-latest`) con los mismos pasos que `scripts/prod.sh build`
(`cargo leptos build --release` + binario y `site/`).

- **Cuándo corre:** al subir un tag `v*`, manualmente (*Run workflow*) y en PRs que
  modifiquen el propio workflow.
- **Dónde salen los binarios:** como artefactos del run
  (`iworkflow-linux-x86_64`, `iworkflow-windows-x86_64`). Con un tag `v*` se publican
  además en la GitHub Release (`.tar.gz` en Linux, `.zip` en Windows).

```bash
git tag v0.5.0 && git push origin v0.5.0
```

Cada archivo contiene `iworkflow` (o `iworkflow.exe`) y la carpeta `site/`, que deben
estar juntos. Para arrancarlo, desde la carpeta descomprimida:

```bash
# Linux
LEPTOS_OUTPUT_NAME=iworkflow LEPTOS_SITE_ROOT=site LEPTOS_ENV=PROD \
  LEPTOS_SITE_ADDR=127.0.0.1:3000 ./iworkflow
```

```powershell
# Windows (PowerShell)
$env:LEPTOS_OUTPUT_NAME="iworkflow"; $env:LEPTOS_SITE_ROOT="site"; $env:LEPTOS_ENV="PROD"
$env:LEPTOS_SITE_ADDR="127.0.0.1:3000"; .\iworkflow.exe
```

**Qué es autocontenido:** SQLite va incluido en ambos binarios y el de Windows enlaza
el runtime de MSVC de forma estática (no requiere el VC++ Redistributable). El de
Linux usa glibc dinámica; compilar en Ubuntu 22.04 mantiene bajo el requisito mínimo.

**Limitaciones:**

- No es un único fichero: los assets web se sirven desde `site/` en disco. Un
  ejecutable único requeriría incrustarlos en el binario (p. ej. con `rust-embed`).
- Windows (nativo): la terminal y los agentes se lanzan con `powershell.exe` (sin shell de
  login ni `exec`; el `PATH` del sistema se hereda) y al cerrarlos se mata el árbol de
  procesos con `taskkill /T`. El home se toma de `USERPROFILE` si falta `HOME`. Los límites
  de RAM/CPU por tarea (systemd) solo existen en Linux. El soporte se ha escrito y probado
  con tests de lógica en Linux, pero **no se ha ejecutado aún sobre un Windows real**: los
  agentes (`gentle-shell`, `pi`, `claude`) deben estar en el `PATH` de PowerShell.

## Desarrollo

Guía de la arquitectura y del código en [ARCHITECTURE.md](ARCHITECTURE.md); reglas del proyecto en [AGENTS.md](AGENTS.md). Comprobaciones:

```bash
cargo fmt --all -- --check
cargo clippy --features ssr --all-targets -- -D warnings
cargo clippy --features hydrate --target wasm32-unknown-unknown --lib -- -D warnings
cargo test --features ssr
PLAYWRIGHT_MODULE=/ruta/node_modules/playwright ./end2end/run_smoke.sh   # UI con agente falso
```
