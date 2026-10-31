# iworkflow

> 🇬🇧 English version: [README.md](README.md)

**iworkflow es un ADE (Agentic Development Environment)**: una aplicación web autoalojada para lanzar, seguir y revisar varios agentes de código con IA en paralelo. En lugar de editar archivos tú mismo, gestionas **tareas**: cada una ejecuta un agente en su propio git worktree y revisas el diff antes de que nada llegue a tu rama.

<p align="center">
  <img src="docs/media/task-graph.gif" alt="Grafo de tareas de iworkflow: las tareas encadenadas esperan a sus predecesoras y, al integrar una, arrancan las siguientes solas" width="900" />
  <br />
  <sub>Encadena tareas en un tablero; al integrar una, las que dependen de ella arrancan solas.</sub>
</p>

> **Estado:** temprano y en desarrollo activo. Se ejecuta en tu propia máquina, no tiene cuentas de usuario (solo un token opcional para acceso por red) y es *source-available* (PolyForm Shield 1.0.0), no open source aprobado por la OSI. Consulta [Arranque rápido](#arranque-rápido) y [Licencia](#licencia).

## Características

- **Agentes en paralelo, un solo lugar**: lanza y dirige `gentle-shell`, `pi`, `claude` o `cursor` a la vez.
- **Aislamiento por diseño**: cada tarea tiene su propio **git worktree**, así que los agentes no se pisan.
- **Revisar, no rehacer**: al terminar un agente lees su diff y lo integras (`merge --no-ff`) o lo descartas.
- **Grafo de tareas**: encadena tareas en un tablero; una tarea arranca sola cuando sus predecesoras están integradas.
- **Integraciones**: GitLab / GitHub (MR/PR y estado de CI) y Jira Server / Data Center.
- **Mobile-first**: consulta el progreso, responde a un agente e integra desde el móvil (con Tailscale).
- **Espacio de trabajo con pestañas**: tablero, tarea, diff, terminal y editor de archivos en pestañas arrastrables y divisibles.

iworkflow no impone ningún flujo: cada agente usa la configuración y los plugins que ya tengas instalados y el `AGENTS.md` del proyecto. Internamente, los agentes se manejan por RPC (o ACP / stream-json, según el runtime). Está construido con Go (`net/http` + `chi`), SSR con `templ` y `htmx`, Tailwind CSS, `sqlc` y SQLite (`modernc.org/sqlite`).

## Índice

- [Diseñada para el móvil](#diseñada-para-el-móvil)
- [Arranque rápido](#arranque-rápido)
- [Espacio de trabajo con pestañas](#espacio-de-trabajo-con-pestañas)
- [Tablero por workspace](#tablero-por-workspace)
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

Requisitos: `git`, Go (1.23 o superior), `templ`, `tailwindcss` y al menos un CLI de agente instalado y con sesión iniciada (`gentle-shell`, `pi`, `claude` o `agent` para Cursor).

```bash
git clone https://github.com/plopezm/iworkflow.git
cd iworkflow
make dev                    # http://127.0.0.1:9000
```

1. **Workspaces** → registra una carpeta con git (necesita al menos un commit).
2. **Tareas** → elige workspace, escribe el prompt y lanza.
3. En el detalle de la tarea sigues al agente en vivo y puedes:
   - enviar mensajes (prompt si está esperando; *steer* o *follow-up* si está trabajando),
   - responder diálogos del agente (`ask_user_question`, confirmaciones…),
   - abortar la ejecución en curso.
4. **Finalizar y revisar** → cierra el agente, commitea lo pendiente y muestra el diff.
5. **Merge** (integra con `--no-ff` y borra el worktree) o **Descartar**.

> **Seguridad:** iworkflow solo se protege con un token compartido sobre HTTP y lanza agentes que ejecutan comandos con tu usuario. No lo expongas a redes no confiables; consulta [Producción](#producción) y [Tailscale](#desarrollar-desde-el-móvil-con-tailscale). El desarrollo escucha en `127.0.0.1:3000`; el script de producción en `0.0.0.0:9000`.

## Espacio de trabajo con pestañas

La interfaz es un espacio de trabajo tipo IDE: cada pantalla (tablero, tarea, diff, terminal, ajustes, ficheros…) se abre como **pestaña** dentro de grupos que puedes dividir arrastrando. Las pestañas se pueden reordenar arrastrándolas dentro de su barra, moverlas a otro grupo o a una división nueva.

- **Explorador de archivos y editor** (con resaltado de sintaxis) sobre un workspace o sobre el worktree de una tarea. En el panel de la tarea, sección **Revisión → Explorar archivos**.
- Los ficheros y carpetas modificados, añadidos, sin seguimiento, borrados o en conflicto se colorean según `git status`.
- Mientras editas, el margen izquierdo marca las líneas añadidas, modificadas y borradas respecto a `HEAD` (como en VS Code).
- Al **editar una tarea**, los límites de CPU y memoria están en *Opciones avanzadas*.
- **Proyectos relacionados:** las pantallas de crear y editar tarea listan los demás workspaces en *Proyectos relacionados*. La tarea guarda los que marques; al arrancar, su agente recibe sus rutas y la indicación de no modificarlas, puede leer sus ficheros y, si existen, consultar el codegraph de cada proyecto (`.codegraph/`) y su memoria engram (solo si `engram` está instalado). Claude además recibe las carpetas con `--add-dir`, y sus herramientas de edición (`Edit`, `Write`, `NotebookEdit`) tienen denegado escribir en ellas. Los comandos de shell no están aislados, así que no es una garantía absoluta. Al escribir `@` en el prompt se completan ficheros del proyecto actual y de los marcados, con la forma `@<proyecto>:<ruta>` (los espacios del nombre pasan a guiones).

## Tablero por workspace

Cada workspace tiene su propio **tablero**: un lienzo de nodos de tarea unidos por flechas (`/board/<workspace>`; `/` abre el primer workspace por nombre). Al pulsar un workspace en la barra lateral se abre su tablero, y los chips de workspace de la parte superior del tablero permiten cambiar de uno a otro. El menú del elemento de la barra lateral conserva **Ver ficheros** (el árbol del proyecto), **Abrir terminal** y **Gestionar workspaces**.

- **Grafo de dependencias:** arrastra desde el puerto derecho de un nodo hasta otra tarea para que esta espere a la primera. Selecciona una flecha para borrarla. Las autoaristas, duplicados, ciclos, flechas entre workspaces y flechas hacia una tarea que ya no está en *Por hacer* se rechazan con un mensaje.
- **Disparador al integrar:** una tarea con predecesoras arranca sola, sobre su rama base configurada, cuando **todas** están `merged`. Una tarea *Por hacer* sin predecesoras se inicia a mano con **Iniciar**. Las flechas se dibujan atenuadas mientras la predecesora está pendiente, con una línea discontinua en movimiento mientras trabaja y en verde continuo cuando se integra. (Las condiciones de arista aún no están implementadas; el modelo deja sitio para ellas.)
- **Tareas anidadas y en paralelo:** *Añadir tarea siguiente* crea una tarea que depende de la seleccionada; *Añadir tarea en paralelo* crea una con las mismas predecesoras. Ambas se crean en *Por hacer*.
- **Lienzo:** arrastra el fondo para desplazarte, usa la rueda para hacer zoom y **Ajustar vista** para encuadrar todo. Las posiciones de los nodos se guardan; las tareas sin posición guardada se colocan automáticamente por capas.
- **Panel de tareas sueltas:** a la izquierda se listan las tareas sin flechas ni posición guardada (todas las creadas con *Nueva tarea* nacen aquí) con sus acciones habituales (Iniciar, Editar, Borrar, Abrir, Integrar…), de modo que se pueden ejecutar por separado sin ninguna cadena. Arrastra una tarjeta al lienzo, o pulsa **Añadir al grafo**, para convertirla en nodo; las tareas de *Añadir siguiente / en paralelo* van directas al grafo. **Quitar del grafo** en un nodo borra todas sus flechas y su posición guardada (de forma atómica) y la devuelve a este panel; las sucesoras que se quedan sin predecesoras pasan a ser tareas sueltas normales. El panel se pliega y muestra su contador; en pantallas de menos de 768 px pasa a ser un selector **Sueltas (N) / Grafo**.

## Ciclo de vida de una tarea

<p align="center">
  <img src="docs/media/task-review.gif" alt="Revisión de una tarea en iworkflow: lanzar un agente, seguirlo en vivo, revisar su diff e integrarlo" width="800" />
  <br />
  <sub>Una tarea de principio a fin: lanzar, seguir al agente en vivo, revisar el diff, integrar. (Interfaz en inglés.)</sub>
</p>

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
| `--addr DIR:PUERTO` | Dirección de escucha (también la variable `IWORKFLOW_ADDR`). La fija: se ignoran los ajustes de red guardados. Por defecto `127.0.0.1:3000` (`:9000` empaquetado). |
| `--site-root RUTA` | Carpeta con los recursos estáticos. Por defecto `site/` junto al ejecutable, o `web/static`. |
| `-h`, `--help` | Muestra la ayuda. |

Ejemplos: `./iworkflow --no-open`, `make run ARGS=--no-open`, `make dev ARGS=--no-open`.

En desarrollo (`make dev`) el servidor escucha solo en `127.0.0.1`.

### Acceso desde otros dispositivos

Por defecto el servidor escucha solo en `127.0.0.1` (este equipo). Para usarlo desde el móvil, activa **Configuración → Acceso desde la red** y reinicia el servidor: pasa a escuchar en `0.0.0.0` con el mismo puerto, y la tarjeta muestra la dirección que abrir en el móvil. La elección se guarda en la base de datos. La misma tarjeta permite cambiar el **puerto del servidor** (1024–65535, se guarda en la base de datos y se aplica en el próximo arranque; un servidor empaquetado usa 9000 por defecto). Si `--addr` o `IWORKFLOW_ADDR` está definida, tiene prioridad y ninguno de los dos ajustes surte efecto. La tarjeta muestra un **código QR** y un selector con las interfaces de red del equipo (por ejemplo ethernet y Tailscale, cada una con su IP): elige la que el móvil pueda alcanzar y escanea el código. Con el token exigido, iworkflow se abre ya autenticado; sin él, el código contiene solo la dirección.

Siempre que el servidor sea accesible desde la red (este ajuste, o un `--addr` que no sea loopback, como el `0.0.0.0:9000` de `make dist` / `make run`) exige por defecto un **token de acceso**:

- El QR y el enlace de la tarjeta llevan el token (`http://<ip>:<puerto>/?token=…`) y siguen a la interfaz seleccionada. Al abrirlo se guarda en una cookie y se redirige a una URL limpia, así que no queda en la barra de direcciones ni en el historial. Sin la cookie, el servidor muestra una página de acceso donde pegar el token.
- Al arrancar, el servidor escribe ese enlace en el log, que es la forma de obtenerlo en una máquina sin navegador.
- **Generar un token nuevo** en la tarjeta invalida el anterior al instante; los dispositivos que lo usaban deben autenticarse de nuevo.
- La exigencia es opcional: desmarca **Exigir el token de acceso** (se aplica en el siguiente arranque) para servir a la red sin él. Entonces cualquiera que llegue al puerto puede ejecutar comandos con tu usuario, así que úsalo solo en una red de total confianza.
- Un servidor solo en loopback (el valor por defecto) nunca pide el token.
- La línea de log de arranque con el enlace contiene el token: trata los logs del servidor como sensibles.
- Las escrituras (`POST`, etc.) que el navegador etiqueta con un `Origin` distinto del `Host` al que van se rechazan con 403, de modo que otra web no puede cambiar estos ajustes a través de tu navegador. Un reverse proxy debe por tanto reenviar el `Host` original.

El token es un secreto compartido sobre HTTP sin cifrar: en una red no fiable puede interceptarse. Es preferible una red privada como Tailscale (ver más abajo).

## Producción

Requisitos: Go, GNU make, `templ`, `sqlc` y la CLI de Tailwind CSS (los mismos que `make generate`).

El Makefile compila y ejecuta la app en modo release:

```bash
make dist     # 1. compila en release y empaqueta en dist/
make run      # 2. arranca lo ya empaquetado
make prod     # ambos pasos seguidos (dist + run)
```

Flujo habitual: ejecuta `make dist` una vez (y de nuevo tras cada actualización) y `make run` cada vez que quieras arrancar el servidor. `ARGS` se pasa al binario (p. ej. `ARGS=--no-open`, ver [Argumentos del ejecutable](#argumentos-del-ejecutable)). Por defecto escucha en `http://0.0.0.0:9000`; se cambia con `SITE_ADDR`:

```bash
make run SITE_ADDR=127.0.0.1:3000 ARGS=--no-open
make dist DIST_DIR=/opt/iworkflow     # empaquetar en otra ruta
```

`make dist` regenera templ, sqlc y CSS, compila el binario `iworkflow` sin símbolos y copia
la carpeta estática `site/` en `dist/`. Ambos son necesarios.

`make run` arranca el binario con `--addr` y `--site-root` desde el directorio actual, así que el `.env` de ese directorio
también se aplica. Variables de make:

| Variable | Por defecto |
|---|---|
| `DIST_DIR` | `dist/` en la raíz del repo |
| `SITE_ADDR` | `0.0.0.0:9000` (todas las interfaces) |
| `ARGS` | ninguno (flags del binario) |

> **Seguridad:** la app lanza agentes que ejecutan comandos con tu usuario. Su única
> protección es el token de acceso (ver [Acceso desde otros dispositivos](#acceso-desde-otros-dispositivos)),
> que se exige siempre que el bind no sea loopback salvo que lo desactives, y viaja por HTTP sin
> cifrar. Restringe el acceso con firewall, red privada (Tailscale/WireGuard) o un
> reverse proxy con TLS. Para limitarlo a la máquina local:
> `make run SITE_ADDR=127.0.0.1:3000`.

Para desplegar en otra máquina basta con copiar `dist/` y lanzarlo con
`./iworkflow --addr 0.0.0.0:9000`
desde dentro de esa carpeta.

## Aplicación de escritorio (Electron)

Para quien no quiera usar la terminal, `electron/` envuelve el mismo servidor en una ventana de escritorio y `make electron-build` construye un instalador para la plataforma en la que se ejecuta (Linux: AppImage, macOS: `.dmg`, Windows: `.exe` NSIS). No hay un «servidor para el móvil» que compilar aparte: el instalador ya contiene el mismo binario `iworkflow`, así que el móvil se conecta al servidor que corre dentro de la app (ver [Usarlo desde el móvil](#usarlo-desde-el-móvil)).

### Construir el instalador

Requisitos (una vez): los de [Producción](#producción), `git` y Node.js 22+ con npm. No hay compilación cruzada: construye cada instalador en su propia plataforma.

```bash
make electron-build
```

Ejecuta, en orden:

1. `make dist`: build release, dejando el binario y `site/` en `dist/`.
2. Copia ambos a `electron/resources/server/` (fuera del archivo asar, para que el binario siga siendo ejecutable).
3. `npm ci` y `electron-builder` para la plataforma actual.
4. Escribe el instalador en `dist/electron/`: `iworkflow-<versión>-linux-x86_64.AppImage`, `iworkflow-<versión>-mac-arm64.dmg` (o `mac-x64`) o `iworkflow-<versión>-win-x64.exe`.

Otras formas:

```bash
make electron-build SKIP_SERVER_BUILD=1   # reutiliza el servidor ya presente en dist/
DIST_DIR=/otra/ruta make electron-build   # usa otro paquete de servidor / ubicación de salida
make electron-dev                         # abre la ventana sobre dist/ sin construir instalador
```

En macOS el `.dmg` se genera para la arquitectura del Mac que lo compila (Apple Silicon o Intel) y requiere las Xcode Command Line Tools. En Windows ejecútalo desde **Git Bash** (con GNU make instalado).

### Ejecutarlo

```bash
chmod +x dist/electron/iworkflow-*.AppImage
./dist/electron/iworkflow-*.AppImage          # añade --appimage-extract-and-run si falta FUSE
```

En macOS abre el `.dmg` y arrastra **iworkflow** a Aplicaciones. En Windows ejecuta el instalador y abre **iworkflow** desde el menú Inicio.

### Usarlo desde el móvil

El servidor de la app es el normal, así que el acceso desde el móvil funciona igual que en [Acceso desde otros dispositivos](#acceso-desde-otros-dispositivos):

1. En la app abre **Configuración → Acceso desde la red** y actívalo.
2. **Cierra la app del todo** (icono de la bandeja → **Quit and stop server**, o **File → Quit**) y vuelve a abrirla. Cerrar solo la ventana no basta: el servidor sigue en marcha y lee este ajuste únicamente al arrancar. Ahora escucha en `0.0.0.0:9000`.
3. Escanea con el móvil el **código QR** de la misma tarjeta (misma red local, o el mismo tailnet de [Tailscale](#desarrollar-desde-el-móvil-con-tailscale)). Se abre ya autenticado con el token de acceso.

También puedes fijar la dirección antes de lanzar, p. ej. `IWORKFLOW_ADDR=0.0.0.0:9000 ./iworkflow-*.AppImage`; entonces se ignora el ajuste guardado.

### Comportamiento

- La app arranca el servidor incluido (`--no-open`) y lo muestra en una ventana. El servidor no cambia: misma base de datos (`~/.iworkflow/workspaces.db`), mismo `IWORKFLOW_ADDR`, mismo token de acceso. La ventana entra sola cuando se exige token.
- Por defecto el servidor escucha en `127.0.0.1:9000`; cambia el puerto en **Configuración → Acceso desde la red**. La app lee la dirección que el servidor anuncia al arrancar, así que sigue al puerto guardado. Si el puerto está ocupado la app muestra un error; libéralo o elige otro.
- **Cerrar la ventana solo la oculta**, así que un móvil conectado por la red local o Tailscale sigue funcionando. Para detener el servidor usa el icono de la bandeja (**Quit and stop server**) o **File → Quit**. En macOS el icono está en la barra de menús y pulsar el icono del Dock vuelve a mostrar la ventana (**iworkflow → Quit** o Cmd+Q detiene el servidor). En un escritorio Linux sin bandeja, cerrar la ventana cierra la app.
- Los enlaces externos se abren en el navegador del sistema; la ventana solo muestra el servidor local.
- `git` (y los CLI de agentes que uses) deben estar instalados y en el `PATH`; el instalador no los incluye. Con el acceso desde la red activo, Windows y macOS piden una vez permitir la app en el cortafuegos. En macOS la app añade `/opt/homebrew/bin` y `/usr/local/bin` a su `PATH`, porque las apps abiertas desde Finder no heredan el de tu shell.
- Los instaladores no están firmados, así que Windows SmartScreen avisa en la primera ejecución. En macOS la app solo lleva firma ad hoc y no está notarizada: Gatekeeper bloquea el primer arranque, así que pulsa con el botón derecho → **Abrir** (o ejecuta `xattr -dr com.apple.quarantine /Applications/iworkflow.app`).

### Actualizaciones

La app consulta el último release de GitHub 15 segundos después de arrancar, cada 6 horas y desde el menú de la bandeja (**Check for updates**). No instala nada sin preguntar:

- **Windows (NSIS) y Linux (AppImage)**: ofrece la nueva versión, la descarga (solo los bloques cambiados cuando es posible), verifica el sha512 de `latest*.yml` y, tras aceptar, para el servidor, reemplaza la app y la vuelve a abrir (`electron-updater`). Un build de Linux que no sea AppImage solo avisa.
- **macOS**: solo avisa y abre la página del release, porque macOS no autoinstala una app sin firma de código válida y la app solo lleva firma ad hoc. Descarga el nuevo `.dmg` y reemplaza la app.
- `IWF_UPDATE_CHECK=off` desactiva la comprobación. Un repositorio privado necesita `GH_TOKEN` en el entorno; uno público no necesita nada. Las ejecuciones de desarrollo (`electron .`) nunca comprueban.

### Workflow de release

`.github/workflows/release.yml` construye los archivos del servidor y, en los mismos runners de Linux, macOS (Apple Silicon e Intel) y Windows, los instaladores de escritorio: ejecuta los tests del lanzador (`npm test` en `electron/`), después `make electron-build` sobre el servidor que acaba de compilar, y sube `iworkflow-desktop-<plataforma>`. Al subir un tag `v*` se publica todo en una release de GitHub; también puede lanzarse a mano (*Run workflow*). La versión del instalador sale de `electron/package.json`: cámbiala con `make bump-version VERSION=x.y.z`. El workflow sube también `latest-linux.yml` / `latest.yml` (y los `.blockmap`) junto a los instaladores: son lo que lee `electron-updater`, así que un release sin ellos no puede actualizar instalaciones de Windows ni Linux. macOS no tiene.

## Desarrollar desde el móvil con Tailscale

[Tailscale](https://tailscale.com) crea una red privada (tailnet) entre tus dispositivos, de modo que el móvil alcanza el servidor que corre en tu ordenador sin abrir puertos a internet. Es importante porque iworkflow solo se protege con un token compartido sobre HTTP y lanza agentes que ejecutan comandos con tu usuario.

1. Instala Tailscale en el ordenador que ejecuta iworkflow y en el móvil (App Store / Google Play) e inicia sesión con la **misma cuenta** en ambos.
2. Arranca el servidor escuchando en la interfaz de Tailscale (o en todas, que es el valor por defecto de `make dist` / `make run`):

   ```bash
   make run                                        # bind a 0.0.0.0:9000
   # o solo accesible en esta máquina y, vía Tailscale Serve, en la tailnet (ver abajo)
   make run SITE_ADDR=127.0.0.1:3000 ARGS=--no-open
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
make test                                      # go test -p 2 -race ./...
make vet                                       # go vet ./...
make lint                                      # golangci-lint run
make e2e                                       # suite completa e2e Playwright en navegador (o FILTER=<name>)
make electron-build                            # empaquetar app de escritorio (dist/electron)
make electron-dev                              # abrir ventana de Electron en desarrollo
```

## Licencia

Copyright 2026 Pablo López (plopezm). Distribuido bajo la [licencia PolyForm Shield 1.0.0](LICENSE).

Puedes usar, modificar y distribuir iworkflow libremente, también dentro de empresas y para trabajo comercial, pero no usarlo para ofrecer un producto que compita con él (por ejemplo, revenderlo u ofrecerlo como servicio alojado). Es una licencia *source-available*, no una licencia open source aprobada por la OSI.

Los recursos de terceros incluidos mantienen su propia licencia: xterm.js (MIT) y highlight.js (BSD-3-Clause) en `public/vendor/`, y las fuentes Instrument Serif y Martian Mono (SIL OFL 1.1) en `public/fonts/`. Todas las dependencias de Rust usan licencias permisivas (MIT, Apache-2.0, BSD, Zlib, Unicode…).

## Apoya el proyecto

<p align="center">
  <a href="https://buymeacoffee.com/plopezm" target="_blank" rel="noopener">
    <img src="https://img.buymeacoffee.com/button-api/?text=Buy%20me%20a%20coffee&emoji=%E2%98%95&slug=plopezm&button_colour=FFDD00&font_colour=000000&font_family=Cookie&outline_colour=000000&coffee_colour=ffffff" alt="Buy me a coffee" height="50" />
  </a>
</p>

Si iworkflow te ahorra tiempo, puedes invitarme a un café en **[buymeacoffee.com/plopezm](https://buymeacoffee.com/plopezm)**. ¡Gracias! ☕
