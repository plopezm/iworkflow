# iworkflow

> 🇬🇧 English version: [README.md](README.md)

<p align="center">
  <a href="https://buymeacoffee.com/plopezm" target="_blank" rel="noopener">
    <img src="https://img.buymeacoffee.com/button-api/?text=Buy%20me%20a%20coffee&emoji=%E2%98%95&slug=plopezm&button_colour=FFDD00&font_colour=000000&font_family=Cookie&outline_colour=000000&coffee_colour=ffffff" alt="Buy me a coffee" height="50" />
  </a>
</p>

**iworkflow es un ADE (Agentic Development Environment)**: un entorno de desarrollo potenciado por IA, concebido como una evolución del IDE tradicional. En lugar de escribir todo el código manualmente, orquestas agentes autónomos de IA que actúan como colaboradores activos. El flujo de trabajo principal deja de ser la edición de archivos y pasa a ser la **gestión de tareas**: lanzas, coordinas y revisas el trabajo de varios agentes a la vez.

- **Orquestación de agentes**: una superficie unificada para lanzar, seguir y dirigir varios agentes en paralelo (`gentle-shell`, `pi`, `claude`, `cursor`).
- **Espacios de trabajo aislados**: cada tarea corre en su propio **git worktree**, de modo que los agentes en paralelo no sobrescriben el trabajo de los demás.
- **Revisar, no rehacer**: al terminar un agente revisas su diff y decides si integrarlo (`merge --no-ff`) o descartarlo. Los agentes pueden ejecutar pruebas y las tareas se conectan con GitLab/GitHub (MR/PR y CI) y Jira.
- **Cambio de rol del desarrollador**: de "hacerlo todo" a **orquestar y revisar** lo que ejecutan los agentes.

La app no impone ningún flujo: cada agente usa la configuración y los plugins que tengas instalados y el `AGENTS.md` del proyecto. Internamente, los agentes se manejan por RPC (o ACP / stream-json, según el runtime).

## Índice

- [Diseñada para el móvil](#diseñada-para-el-móvil)
- [Arranque rápido](#arranque-rápido)
- [Espacio de trabajo con pestañas](#espacio-de-trabajo-con-pestañas)
- [Ciclo de vida de una tarea](#ciclo-de-vida-de-una-tarea)
- [Integración con GitLab y GitHub](#integración-con-gitlab-y-github)
- [Integración con Jira (Server / Data Center)](#integración-con-jira-server--data-center)
- [Perfiles y modelos de agentes](#perfiles-y-modelos-de-agentes)
- [Configuración](#configuración)
  - [Runtimes de agente](#runtimes-de-agente)
  - [Argumentos del ejecutable](#argumentos-del-ejecutable)
- [Producción](#producción)
- [Desarrollar desde el móvil con Tailscale](#desarrollar-desde-el-móvil-con-tailscale)
- [Desarrollo](#desarrollo)
- [Licencia](#licencia)
- [Apoya el proyecto](#apoya-el-proyecto)

## Diseñada para el móvil

iworkflow está pensada **primero para el móvil**. Como tu trabajo es lanzar, seguir y revisar agentes (no escribir código), encaja bien en un teléfono:

- La interfaz es responsive: en pantallas pequeñas el menú lateral pasa a ser un panel desplegable desde una barra superior y el espacio de trabajo con pestañas se adapta al viewport.
- El agente sigue ejecutándose en tu máquina mientras estás fuera. Desde el móvil consultas el progreso, respondes a las preguntas del agente, envías un mensaje, revisas el diff e integras o descartas.
- En iOS (Safari → Compartir → *Añadir a pantalla de inicio*) y Android (Chrome → menú → *Añadir a pantalla de inicio*) puedes fijarla como acceso directo. Por ahora se abre en el navegador; aún no incluye un manifiesto de aplicación web, así que no es una app independiente a pantalla completa.

El servidor corre en tu ordenador y el móvil es solo un navegador, así que necesitas una forma segura de alcanzarlo: consulta [Desarrollar desde el móvil con Tailscale](#desarrollar-desde-el-móvil-con-tailscale).

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

> **Seguridad:** iworkflow no tiene autenticación y lanza agentes que ejecutan comandos con tu usuario. No lo expongas a redes no confiables; consulta [Producción](#producción) y [Tailscale](#desarrollar-desde-el-móvil-con-tailscale). El desarrollo escucha en `127.0.0.1:3000`; el script de producción en `0.0.0.0:9000`.

## Espacio de trabajo con pestañas

La interfaz es un espacio de trabajo tipo IDE: cada pantalla (tablero, tarea, diff, terminal, ajustes, ficheros…) se abre como **pestaña** dentro de grupos que puedes dividir arrastrando. Las pestañas se pueden reordenar arrastrándolas dentro de su barra, moverlas a otro grupo o a una división nueva.

- **Explorador de archivos y editor** (con resaltado de sintaxis) sobre un workspace o sobre el worktree de una tarea. En el panel de la tarea, sección **Revisión → Explorar archivos**.
- Los ficheros y carpetas modificados, añadidos, sin seguimiento, borrados o en conflicto se colorean según `git status`.
- Mientras editas, el margen izquierdo marca las líneas añadidas, modificadas y borradas respecto a `HEAD` (como en VS Code).
- Al **editar una tarea**, los límites de CPU y memoria están en *Opciones avanzadas*.

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

## Integración con GitLab y GitHub

Desde cada tarea puedes crear un merge request (GitLab) o pull request (GitHub), ver el CI y mezclarlo. Un workspace puede ser un repositorio de cualquiera de los dos: el proveedor se detecta por el host del remoto `origin` y se aplica la configuración de ese proveedor.

1. En **Configuración** hay una tarjeta por proveedor.
   - **GitLab**: URL de tu instancia (p. ej. `https://gitlab.miempresa.local`) y un token personal con scope `api`.
   - **GitHub**: `https://github.com` (propuesta por defecto) o la URL de tu GitHub Enterprise, y un token personal (scope `repo`; fine-grained: Pull requests y Contents en lectura/escritura, Actions en lectura).
   Usa "Probar conexión" para validarlos. Si tu certificado es de una CA interna o autofirmado, marca "Aceptar certificado TLS no confiable".
2. El proyecto se deduce del remoto `origin` (https, ssh o scp). Su host debe coincidir con la URL de un proveedor configurado; si no coincide con ninguno se te avisa, y si es github.com sin configurar se te pide configurar GitHub. En GitHub el repositorio es `propietario/repositorio`.
3. En el panel de la tarea, pestaña **PR / MR**: crear el MR/PR (rama de la tarea → rama base), ver su estado y el último pipeline (GitLab) o workflow (GitHub) con sus jobs, y **Mezclar** (pide confirmación).

Notas:
- La app **nunca hace `git push`**: publica tú la rama antes de crear el MR; si no está en `origin`, se te avisa.
- El token se guarda en claro en la base de datos local (`~/.iworkflow/workspaces.db`) y nunca se envía al navegador. Dejarlo vacío al editar conserva el guardado, salvo que cambies la URL: entonces hay que volver a introducirlo, para no enviarlo a otro servidor.
- Las redirecciones HTTP están desactivadas para no filtrar el token a otro host.

## Integración con Jira (Server / Data Center)

La pestaña **Jira** conecta con un Jira on premise usando la URL del servidor y un token de acceso personal (PAT, `Authorization: Bearer`, API REST v2). Al guardar, la conexión se valida contra Jira; el token se guarda en la base de datos local y nunca vuelve al navegador (dejarlo vacío al editar conserva el guardado).

- **Importar**: «Sincronizar» trae las tareas que cumplen el JQL de la conexión (por defecto `assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC`, máximo 500). El estado local (nueva / convertida / ignorada) se conserva entre sincronizaciones; las tareas nuevas que ya no devuelve Jira se retiran.
- **Gestionar**: lista con búsqueda y filtros, detalle con descripción y comentarios, y escritura hacia Jira: cambiar el estado (transiciones del workflow) y comentar. Ignorar/restaurar es solo local.
- **Convertir**: el botón «Convertir en tarea» abre un formulario con el título, el prompt y la prioridad propuestos desde la tarea de Jira (la prioridad se mapea del nombre de Jira); eliges workspace y rama base, editas lo que quieras y se crea la tarea (opcionalmente iniciada). La tarea de Jira queda marcada como convertida y enlazada.
- Si el servidor usa un certificado autofirmado o de una CA interna que el sistema no conoce, marca «Aceptar certificado no verificado» en la conexión.

## Perfiles y modelos de agentes

Esta funcionalidad es la integración con [Gentle AI](https://github.com/Gentleman-Programming/gentle-ai), el arnés de agentes que hay detrás de `gentle-shell` y su paquete gentle-pi. Los perfiles y el enrutado de modelos por agente son conceptos de Gentle AI: iworkflow no define los suyos, lee y escribe los mismos ficheros de configuración (`profiles.json`, `models.json`), así que los cambios hechos aquí los ve la TUI de Gentle AI y viceversa. Instala y configura Gentle AI primero; sin él estas pantallas no tienen nada que gestionar.

Dos pantallas equivalentes a los comandos de gentle-pi, sobre los mismos ficheros:

| Pantalla | Equivale a | Qué hace |
|---|---|---|
| `/profiles` | `/gentle:profiles` | Crear, duplicar, renombrar, eliminar, importar y exportar perfiles (`profiles.json`), editar modelo y esfuerzo por agente y crear roles de agente nuevos. |
| `/models` | `/gentle:models` | Editar el routing global (`models.json`): modelo y esfuerzo por agente. |

Los perfiles se eligen en el desplegable de ajustes del agente, que lee el mismo `profiles.json`. Estas pantallas no activan un perfil de forma global; solo lo gestionan. El bloqueo de escritura evita pisadas dentro de un proceso, no frente a otro proceso (p. ej. el TUI de gentle-pi) editando a la vez.

## Configuración

Variables de entorno (admite `.env`, ver `.env.example`):

| Variable | Por defecto |
|---|---|
| `IWF_DATABASE_URL` | `~/.iworkflow/workspaces.db` (SQLite, migraciones automáticas) |
| `IWF_WORKTREES_DIR` | `~/.iworkflow/worktrees` |
| `IWF_GENTLE_SHELL_BIN` | `gentle-shell` (se lanza como `--mode rpc` con `GENTLE_SHELL_INTERACTIVE_HOST=1`) |
| `IWF_PI_BIN` | `pi` (runtime `pi`, se lanza como `--mode rpc`) |
| `IWF_CLAUDE_BIN` | `claude` (runtime `claude`: Claude Code con `-p` y protocolo stream-json; se elige en el selector de runtime del menú lateral; acepta ediciones de ficheros, pide confirmación para el resto y lee el `AGENTS.md` del proyecto) |
| `IWF_CURSOR_BIN` | `agent` (runtime `cursor`: Cursor CLI en modo servidor ACP, `agent acp`; requiere haber iniciado sesión con `agent login`) |
| `IWF_CURSOR_SESSIONS_DIR` | `cursor-sessions` junto a la base de datos (guarda el id de sesión ACP de cada tarea para reanudarla) |
| `IWF_ATTACHMENTS_DIR` | `attachments` junto a la base de datos (imágenes adjuntas a las tareas; se borran al terminar la tarea) |
| `IWF_AGENTS_FILES_DIR` | `agents-files` junto a la base de datos (biblioteca de `AGENTS.md`) |
| `IWF_GENTLE_PI_CONFIG_HOME` | `~/.pi/gentle-ai` (contiene `profiles.json` y `models.json`) |
| `IWF_PI_AGENTS_DIR` | `~/.pi/agent/agents` (un `<rol>.md` por rol de agente) |

Los directorios por defecto de datos (`attachments`, `agents-files`, `cursor-sessions`) cuelgan de la carpeta de la base de datos: si cambias `IWF_DATABASE_URL`, se mueven con ella. En Windows el home se toma de `USERPROFILE` si falta `HOME`.

### Runtimes de agente

El selector de runtime del menú lateral admite `gentle-shell`, `pi`, `claude` y `cursor`. La elección se fija al arrancar cada tarea.

| Runtime | Binario | Capacidades |
|---|---|---|
| `gentle-shell` / `pi` | `IWF_GENTLE_SHELL_BIN` / `IWF_PI_BIN` | todas (steer, follow-up, diálogos, reanudar, modelo, thinking, subagentes) |
| `claude` | `IWF_CLAUDE_BIN` | diálogos, reanudar, modelo y thinking |
| `cursor` | `IWF_CURSOR_BIN` | diálogos, reanudar y modelo (la lista de modelos se obtiene del propio agente por ACP). Las ediciones de ficheros se aceptan; el resto pide confirmación |

### Argumentos del ejecutable

Al arrancar, `iworkflow` abre la web en el navegador (`http://<host>:<puerto>` del servidor; si escucha en `0.0.0.0` se usa `127.0.0.1`).

| Argumento | Efecto |
|---|---|
| `--no-open` | Arranca solo el servidor, sin abrir el navegador. |
| `-h`, `--help` | Muestra la ayuda. |

Ejemplos: `./iworkflow --no-open`, `./scripts/prod.sh run --no-open`, `cargo leptos watch -- --no-open`.

En desarrollo (`cargo leptos watch`) el servidor escucha solo en `127.0.0.1`.

## Producción

Requisitos: la toolchain de Rust, `cargo-leptos` (`cargo install cargo-leptos --locked`) y el target `wasm32-unknown-unknown` (`rustup target add wasm32-unknown-unknown`).

`scripts/prod.sh` compila y ejecuta la app en modo release:

```bash
./scripts/prod.sh build     # 1. compila en release y empaqueta en dist/
./scripts/prod.sh run       # 2. arranca lo ya empaquetado
./scripts/prod.sh           # ambos pasos seguidos (build + run)
```

Flujo habitual: ejecuta `build` una vez (y de nuevo tras cada actualización) y `run` cada vez que quieras arrancar el servidor. Los argumentos tras `run` se pasan al binario (p. ej. `--no-open`, ver [Argumentos del ejecutable](#argumentos-del-ejecutable)). Por defecto escucha en `http://0.0.0.0:9000`; se cambia con `LEPTOS_SITE_ADDR`:

```bash
LEPTOS_SITE_ADDR=127.0.0.1:3000 ./scripts/prod.sh run --no-open
DIST_DIR=/opt/iworkflow ./scripts/prod.sh build     # empaquetar en otra ruta
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
| `LEPTOS_SITE_ADDR` | `0.0.0.0:9000` (todas las interfaces) |

> **Seguridad:** la app no tiene autenticación y lanza agentes que ejecutan comandos
> con tu usuario. Con el bind a `0.0.0.0`, cualquiera que alcance el puerto puede
> usarla. Restringe el acceso con firewall, red privada (Tailscale/WireGuard) o un
> reverse proxy con autenticación y TLS. Para limitarlo a la máquina local:
> `LEPTOS_SITE_ADDR=127.0.0.1:3000 ./scripts/prod.sh run`.

Para desplegar en otra máquina basta con copiar `dist/` y lanzarlo con
`LEPTOS_SITE_ROOT=site LEPTOS_ENV=PROD LEPTOS_SITE_ADDR=0.0.0.0:3000 ./iworkflow`
desde dentro de esa carpeta.

## Desarrollar desde el móvil con Tailscale

[Tailscale](https://tailscale.com) crea una red privada (tailnet) entre tus dispositivos, de modo que el móvil alcanza el servidor que corre en tu ordenador sin abrir puertos a internet. Es importante porque iworkflow no tiene autenticación y lanza agentes que ejecutan comandos con tu usuario.

1. Instala Tailscale en el ordenador que ejecuta iworkflow y en el móvil (App Store / Google Play) e inicia sesión con la **misma cuenta** en ambos.
2. Arranca el servidor escuchando en la interfaz de Tailscale (o en todas, que es el valor por defecto de `scripts/prod.sh`):

   ```bash
   ./scripts/prod.sh run                                        # bind a 0.0.0.0:9000
   # o solo accesible en esta máquina y, vía Tailscale Serve, en la tailnet (ver abajo)
   LEPTOS_SITE_ADDR=127.0.0.1:3000 ./scripts/prod.sh run --no-open
   ```

3. Obtén el nombre o la IP del ordenador en la tailnet:

   ```bash
   tailscale ip -4        # p. ej. 100.101.102.103
   tailscale status       # muestra el nombre MagicDNS, p. ej. mipc.nombre-tailnet.ts.net
   ```

4. En el móvil abre `http://<ip-tailnet>:9000` o `http://mipc.nombre-tailnet.ts.net:9000`.

**Recomendado: HTTPS con Tailscale Serve.** Mantén el bind en `127.0.0.1` y deja que Tailscale lo exponga solo dentro de tu tailnet, con certificado válido:

```bash
tailscale serve --bg 3000
# https://mipc.nombre-tailnet.ts.net  (solo accesible desde tus dispositivos de la tailnet)
```

Los WebSockets del terminal se aceptan cuando el host de `Origin` coincide con la cabecera `Host`, lo que ocurre si usas la misma dirección en el navegador.

Consejos:

- **No** uses Tailscale Funnel: publicaría la app en todo internet.
- Limita con ACLs de Tailscale quién puede llegar a la máquina si la tailnet es compartida.
- El ordenador debe seguir encendido y el servidor en marcha; usa `tmux` o un servicio de usuario de systemd para sesiones largas.

## Desarrollo

Guía de la arquitectura y del código en [ARCHITECTURE.es.md](ARCHITECTURE.es.md); reglas del proyecto en [AGENTS.md](AGENTS.md). Comprobaciones:

```bash
cargo fmt --all -- --check
cargo clippy --features ssr --all-targets -- -D warnings
cargo clippy --features hydrate --target wasm32-unknown-unknown --lib -- -D warnings
cargo test --features ssr
PLAYWRIGHT_MODULE=/ruta/node_modules/playwright ./end2end/run_smoke.sh   # UI con agente falso
```

## Licencia

Copyright 2026 Pablo López (plopezm). Distribuido bajo la [licencia PolyForm Shield 1.0.0](LICENSE).

Los recursos de terceros incluidos mantienen su propia licencia: xterm.js (MIT) y highlight.js (BSD-3-Clause) en `public/vendor/`, y las fuentes Instrument Serif y Martian Mono (SIL OFL 1.1) en `public/fonts/`. Todas las dependencias de Rust usan licencias permisivas (MIT, Apache-2.0, BSD, Zlib, Unicode…).

## Apoya el proyecto

Si iworkflow te ahorra tiempo, puedes invitarme a un café en **[buymeacoffee.com/plopezm](https://buymeacoffee.com/plopezm)**. ¡Gracias! ☕
