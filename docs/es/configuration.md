# Configuración

> 🇬🇧 [English version](../configuration.md) · [← README](../../README.es.md)

## Variables de entorno

Variables de entorno (se admite `.env`, ver `.env.example`):

| Variable | Por defecto |
|---|---|
| `IWF_HOME` | `~/.iworkflow`. Carpeta de datos; todo lo siguiente se deriva de ella y no se puede cambiar por separado: `workspaces.db` (SQLite, migraciones automáticas), `worktrees/`, `attachments/` (imágenes adjuntas a las tareas; se borran cuando la tarea termina), `agents-files/` (biblioteca de `AGENTS.md`), `cursor-sessions/` y `agy-sessions/` (id de sesión ACP de cada tarea, para reanudarla), `models/` (modelo de voz del dictado; `make model` lo descarga), `tls/` y `logs/` |
| `IWF_GENTLE_SHELL_BIN` | `gentle-shell` (se lanza como `--mode rpc` con `GENTLE_SHELL_INTERACTIVE_HOST=1`) |
| `IWF_PI_BIN` | `pi` (runtime `pi`, se lanza como `--mode rpc`) |
| `IWF_CLAUDE_BIN` | `claude` (runtime `claude`: Claude Code con `-p` y el protocolo stream-json; se elige en el selector de runtime del menú lateral; acepta ediciones de ficheros, pide confirmación para el resto y lee el `AGENTS.md` del proyecto) |
| `IWF_CURSOR_BIN` | `agent` (runtime `cursor`: Cursor CLI en modo servidor ACP, `agent acp`; requiere haber iniciado sesión con `agent login`) |
| `IWF_AGY_BIN` | `agy` (runtime `agy`: Antigravity CLI en modo servidor ACP, `agy acp`) |
| `IWF_GENTLE_PI_CONFIG_HOME` | `~/.pi/gentle-ai` (contiene `profiles.json` y `models.json`) |
| `IWF_TLS` | sin definir o `auto`: HTTPS está siempre activo, con un certificado autofirmado que se crea en `tls/` dentro de la carpeta de datos y se reutiliza (ver [HTTPS](#https)). `off` sirve HTTP plano; solo para tests y CI, registra un aviso. Cualquier otro valor es un error |
| `IWF_TLS_PORT` | sin definir: el puerto de escucha habitual. Cambia el puerto de escucha (1024–65535); HTTPS se sirve ahí, p. ej. `9000` → `https://host:9000` |
| `IWF_TLS_CERT` / `IWF_TLS_KEY` | Ficheros PEM de tu propio certificado y de su clave, que se usan en lugar del autofirmado; define los dos o ninguno |
| `IWF_ENGRAM_URL` | sin definir: desactivado. URL base de la API HTTP local de Engram, por ejemplo `http://127.0.0.1:7437`. Si se define con una URL http o https válida, activa la memoria de Engram de cada workspace (ver [Memoria de Engram](integrations.md#memoria-de-engram)). Un valor no válido la deja desactivada y registra un aviso al arrancar |
| `IWF_CLAUDE_CREDENTIALS` | `~/.claude/.credentials.json`. Credenciales OAuth de Claude Code, que lee la [barra de uso](workspace.md#límites-de-uso) |
| `IWF_CLAUDE_USAGE_URL` | sin definir: el endpoint real de uso de Anthropic. Sustituye al endpoint de uso de la suscripción de Claude |
| `IWF_CODEX_SESSIONS_DIR` | `~/.codex/sessions`. Donde Codex escribe sus ficheros de sesión; se leen para los límites de Codex |


## Carpeta de datos

Para mover la carpeta de datos: abre **Configuración → Recursos**, escribe la nueva carpeta absoluta en la tarjeta **Carpeta de datos** y pulsa *Mover carpeta*. El servidor se reinicia, copia todo (base de datos, worktrees, adjuntos, modelos…), comprueba la copia, reescribe las rutas guardadas y los worktrees de git, cambia a la carpeta nueva y solo entonces borra la antigua. Se niega mientras una tarea se está iniciando, ejecutando o finalizando (ojo: Claude y pi retoman la conversación por carpeta, así que las tareas que ya se iniciaron empiezan una conversación nueva tras el movimiento; las de Cursor y agy conservan la suya), y también si la carpeta nueva no está vacía, no se puede escribir, está anidada con la antigua o no tiene espacio suficiente. Si algo falla, la carpeta antigua queda intacta y en uso, y la tarjeta explica el motivo. La elección se guarda en un fichero puntero de una sola línea en la carpeta de configuración del usuario (`~/.config/iworkflow/data-home` en Linux, `~/Library/Application Support/iworkflow/data-home` en macOS, `%APPDATA%\iworkflow\data-home` en Windows), fuera de la carpeta de datos, así que las actualizaciones y las reinstalaciones lo conservan. `IWF_HOME` (entorno o `.env`) tiene prioridad sobre el puntero; mientras esté definida, la tarjeta es de solo lectura. También puedes definir `IWF_HOME` tú mismo (en Windows, el instalador lo pregunta y lo guarda como variable de entorno de usuario) tras copiar allí el antiguo `~/.iworkflow`. En Windows, la carpeta personal se toma de `USERPROFILE` cuando falta `HOME`.

## Logs

El servidor escribe en stderr y, a la vez, en `$IWF_HOME/logs/iworkflow.log` (por defecto `~/.iworkflow/logs/iworkflow.log`). La ubicación del fichero es fija; estas variables lo ajustan:

| Variable | Por defecto | Efecto |
|---|---|---|
| `IWF_LOG_LEVEL` | `info` | `debug` registra también cada petición HTTP y más detalle; cualquier otro valor deja `info`. |
| `IWF_LOG_MAX_MB` | `10` | Tamaño en MB a partir del cual se rota el fichero de log. Un valor menor que 1 o no numérico vuelve al valor por defecto. |
| `IWF_LOG_BACKUPS` | `5` | Ficheros rotados que se conservan, comprimidos con gzip; los más antiguos se borran. Misma regla para los valores no válidos. |

La línea de arranque con el enlace de acceso contiene el token: trata el log como información sensible.

## Dictado por voz

El campo Prompt de los formularios de crear y editar tarea tiene un botón de micrófono. Grabas, escuchas la grabación y la confirmas; el servidor la transcribe con [sherpa-onnx](https://github.com/k2-fsa/sherpa-onnx) y el modelo Qwen3-ASR 0.6B, y el texto se añade al prompt como un párrafo nuevo. El navegador necesita un origen seguro (HTTPS o `localhost`) para abrir el micrófono, y las grabaciones se limitan a 60 segundos.

1. `make model` descarga el modelo ya convertido (unos 840 MB) en `$IWF_HOME/models/` (por defecto `~/.iworkflow/models/`) y lo verifica con las sumas sha256 fijadas en `asr-model.sha256`.
2. El reconocedor de voz es el único código cgo del proyecto, tras el build tag `sherpa`. `make build`, `make dev` y `make dist` lo activan (`GO_TAGS=sherpa`); `make build GO_TAGS=` genera un binario sin CGO, en el que el dictado responde "no disponible". `make dist` copia además las librerías compartidas de sherpa-onnx del sistema actual junto al binario (`lib/` en Linux y macOS, la misma carpeta en Windows). Solo se transcribe una grabación a la vez; una segunda petición recibe `429` y el usuario la reintenta.

## HTTPS

El servidor sirve solo HTTPS, en su puerto de escucha (por ejemplo `https://127.0.0.1:9000`). Está siempre activo. Los navegadores solo exponen el micrófono (dictado por voz) en un contexto seguro, así que el micrófono funciona también desde otros dispositivos una vez que su navegador acepta el certificado.

```bash
make dev                       # https://127.0.0.1:3000
make run                       # igual, tras make dist
```

Sin `IWF_TLS_CERT` ni `IWF_TLS_KEY`, el servidor crea la primera vez un certificado autofirmado en `$IWF_HOME/tls/`, válido para `localhost`, `127.0.0.1` y la IP de red de la máquina. Se reutiliza hasta que le queden 30 días o cambie la IP, y entonces se recrea en el siguiente arranque. El navegador avisa de un certificado no fiable la primera vez: acéptalo una vez por dispositivo.

`IWF_TLS=off` sirve HTTP plano en su lugar. Es solo para tests y CI; el servidor registra un aviso al arrancar así.

`IWF_TLS_PORT` cambia el puerto de escucha (1024–65535). Hay un único listener: HTTPS se sirve en ese puerto (`9000` → `https://host:9000`). **Configuración → Acceso desde la red** muestra el enlace y el QR de HTTPS. `--addr` / `IWORKFLOW_ADDR` fijan la dirección de escucha.

## Runtimes de agente

El selector de runtime del menú lateral admite `gentle-shell`, `pi`, `claude`, `cursor` y `agy`. La elección se fija al arrancar cada tarea.

| Runtime | Binario | Capacidades |
|---|---|---|
| `gentle-shell` / `pi` | `IWF_GENTLE_SHELL_BIN` / `IWF_PI_BIN` | todas (steer, follow-up, diálogos, reanudar, modelo, thinking, subagentes) |
| `claude` | `IWF_CLAUDE_BIN` | diálogos, reanudar, modelo y thinking |
| `cursor` | `IWF_CURSOR_BIN` | diálogos, reanudar y modelo (la lista de modelos se obtiene del propio agente por ACP). Las ediciones de ficheros se aceptan; el resto pide confirmación |
| `agy` | `IWF_AGY_BIN` | diálogos, reanudar, modelo (obtenido por ACP) y thinking. Las ediciones de ficheros se aceptan; el resto pide confirmación |

## Argumentos del ejecutable

Al arrancar, `iworkflow` abre la interfaz web en el navegador (`https://<host>:<puerto>` del servidor; si escucha en `0.0.0.0`, se usa `127.0.0.1`).

| Argumento | Efecto |
|---|---|
| `--no-open` | Arranca solo el servidor, sin abrir el navegador. |
| `--addr HOST:PORT` | Dirección de escucha (también la variable `IWORKFLOW_ADDR`). La fija: se ignoran los ajustes de red guardados. Por defecto `127.0.0.1:3000` (`:9000` cuando está empaquetado). |
| `--site-root DIR` | Carpeta con los recursos estáticos. Por defecto, `site/` junto al ejecutable; si no existe, `web/static`. |
| `-h`, `--help` | Muestra la ayuda. |

Ejemplos: `./iworkflow --no-open`, `make run ARGS=--no-open`, `make dev ARGS=--no-open`.

En desarrollo (`make dev`), el servidor escucha solo en `127.0.0.1`.

## Acceso desde otros dispositivos

Por defecto, el servidor escucha solo en `127.0.0.1` (este equipo). Para alcanzarlo desde el móvil, activa **Configuración → Acceso desde la red** y reinicia el servidor: pasará a escuchar en `0.0.0.0` con el mismo puerto, y la tarjeta muestra la dirección que hay que abrir en el móvil. La elección se guarda en la base de datos. La misma tarjeta permite cambiar el **puerto del servidor** (1024–65535; se guarda en la base de datos y se aplica en el siguiente arranque; un servidor empaquetado usa 9000 por defecto). Si `--addr` o `IWORKFLOW_ADDR` están definidos, tienen prioridad y ninguno de los dos ajustes surte efecto. La tarjeta muestra un **código QR** y un selector con las interfaces de red del equipo (por ejemplo, ethernet y Tailscale, cada una con su IP): elige la que el móvil pueda alcanzar y escanea el código. Con el token exigido, iworkflow se abre ya autenticado; sin él, el código contiene solo la dirección.

Siempre que el servidor sea accesible desde la red (por este ajuste, o por un `--addr` que no sea loopback, como el `0.0.0.0:9000` de `make dist` / `make run`), exige por defecto un **token de acceso**:

- El QR y el enlace de la tarjeta llevan el token (`https://<ip>:<puerto>/?token=…`) y siguen a la interfaz seleccionada. Al abrirlo, el token se guarda en una cookie y se redirige a una URL limpia, así que no queda en la barra de direcciones ni en el historial. Sin la cookie, el servidor muestra una página de acceso donde pegar el token.
- Al arrancar, el servidor escribe ese enlace en el log, que es la forma de obtenerlo en una máquina sin navegador.
- **Generar un token nuevo** en la tarjeta invalida el anterior al instante; los dispositivos que lo usaban deben volver a autenticarse.
- La exigencia es opcional: desmarca **Exigir el token de acceso** (se aplica en el siguiente arranque) para servir a la red sin él. En ese caso, cualquiera que llegue al puerto puede ejecutar comandos con tu usuario, así que úsalo solo en una red de total confianza.
- Un servidor solo en loopback (el valor por defecto) nunca pide el token.
- La línea de log de arranque con el enlace contiene el token: trata los logs del servidor como información sensible.
- Las escrituras (`POST`, etc.) que el navegador etiqueta con un `Origin` distinto del `Host` al que van se rechazan con 403, de modo que otra web no puede cambiar estos ajustes a través de tu navegador. Por eso, un reverse proxy debe reenviar el `Host` original.

El token es un secreto compartido. Su conexión usa HTTPS con un certificado autofirmado que ninguna autoridad avala, así que en una red no fiable es preferible una red privada como Tailscale (ver más abajo) o tu propio certificado (`IWF_TLS_CERT` / `IWF_TLS_KEY`).
