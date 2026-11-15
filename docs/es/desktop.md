# Aplicación de escritorio (Electron)

> 🇬🇧 [English version](../desktop.md) · [← README](../../README.es.md)

Para quien no quiera trabajar con una terminal, `electron/` envuelve el mismo servidor en una ventana de escritorio, y `make electron-build` construye un instalador para la plataforma en la que se ejecuta (Linux: AppImage; macOS: `.dmg`; Windows: `.exe` NSIS). No hay un «servidor móvil» que compilar aparte: el instalador ya contiene el mismo binario `iworkflow`, así que el móvil se conecta al servidor que se ejecuta dentro de la aplicación (ver [Usarlo desde el móvil](#usarlo-desde-el-móvil)).

### Construir el instalador

Requisitos (una sola vez): los de [Producción](production.md), `git` y Node.js 22+ con npm. No hay compilación cruzada: construye cada instalador en su propia plataforma.

```bash
make electron-build
```

Ejecuta, en orden:

1. `make dist`: build de release, dejando el binario y `site/` en `dist/`.
2. Copia ambos a `electron/resources/server/` (fuera del archivo asar, para que el binario siga siendo ejecutable).
3. `npm ci` y `electron-builder` para la plataforma actual.
4. Escribe el instalador en `dist/electron/`: `iworkflow-<versión>-linux-x86_64.AppImage`, `iworkflow-<versión>-mac-arm64.dmg` (o `mac-x64`) o `iworkflow-<versión>-win-x64.exe`.

Otras formas:

```bash
make electron-build SKIP_SERVER_BUILD=1   # reutiliza el servidor que ya está en dist/
make electron-build DIST_DIR=/some/dir    # usa otro paquete de servidor / otra ubicación de salida
make electron-dev                         # abre la ventana sobre dist/ sin construir un instalador
```

En macOS, el `.dmg` se genera para la arquitectura del Mac que lo compila (Apple Silicon o Intel) y requiere las Xcode Command Line Tools. En Windows, ejecútalo desde **Git Bash** (con GNU make instalado).

### Ejecutarlo

```bash
chmod +x dist/electron/iworkflow-*.AppImage
./dist/electron/iworkflow-*.AppImage          # añade --appimage-extract-and-run si falta FUSE
```

En macOS, abre el `.dmg` y arrastra **iworkflow** a Aplicaciones. En Windows, ejecuta el instalador y abre **iworkflow** desde el menú Inicio. El instalador tiene una página para elegir la carpeta de datos: si no es la carpeta por defecto `%USERPROFILE%\.iworkflow`, se guarda como variable de entorno de usuario `IWF_HOME` (la aplicación también la lee del registro, así que se aplica desde el primer arranque). Las instalaciones silenciosas y las actualizaciones no tocan un `IWF_HOME` existente.

### Usarlo desde el móvil

El servidor de la aplicación es el normal, así que el acceso desde el móvil funciona igual que en [Acceso desde otros dispositivos](configuration.md#acceso-desde-otros-dispositivos):

1. En la aplicación, abre **Configuración → Acceso desde la red** y actívalo.
2. **Cierra la aplicación del todo** (icono de la bandeja → **Quit and stop server**, o **File → Quit**) y vuelve a abrirla. Cerrar solo la ventana no basta: el servidor sigue en marcha y lee este ajuste únicamente al arrancar. Ahora escucha en `0.0.0.0:9000`.
3. Escanea con el móvil el **código QR** de la misma tarjeta (misma red local, o el mismo tailnet de [Tailscale](mobile.md)). Se abre ya autenticado con el token de acceso.

También puedes fijar la dirección antes de lanzar, p. ej. `IWORKFLOW_ADDR=0.0.0.0:9000 ./iworkflow-*.AppImage`; en ese caso se ignora el ajuste guardado.

### Comportamiento

- La aplicación arranca el servidor incluido (`--no-open`) y lo muestra en una ventana. El servidor no cambia: misma carpeta de datos (`IWF_HOME`) y misma base de datos, mismo `IWORKFLOW_ADDR`, mismo token de acceso. La ventana se autentica sola cuando se exige token.
- Por defecto, el servidor escucha en `127.0.0.1:9000`; cambia el puerto en **Configuración → Acceso desde la red**. La aplicación lee la dirección que el servidor anuncia al arrancar, así que sigue al puerto guardado. Si el puerto está ocupado, la aplicación muestra un error; libéralo o elige otro.
- **Cerrar la ventana solo la oculta**, así que un móvil conectado por la red local o por Tailscale sigue funcionando. Para detener el servidor, usa el icono de la bandeja (**Quit and stop server**) o **File → Quit**. En macOS, el icono está en la barra de menús y, al pulsar el icono del Dock, vuelve a mostrarse la ventana (**iworkflow → Quit** o Cmd+Q detienen el servidor). En un escritorio Linux sin soporte de bandeja, cerrar la ventana cierra la aplicación.
- Los enlaces externos se abren en el navegador del sistema; la ventana solo muestra el servidor local.
- `git` (y los CLI de los agentes que uses) deben estar instalados y en el `PATH`; el instalador no los incluye. Con el acceso desde la red activo, Windows y macOS piden una vez permitir la aplicación en el cortafuegos. En macOS, la aplicación añade `/opt/homebrew/bin` y `/usr/local/bin` a su `PATH`, porque las aplicaciones abiertas desde Finder no heredan el de tu shell.
- Los instaladores no están firmados digitalmente, así que Windows SmartScreen avisa en la primera ejecución. En macOS, la aplicación solo tiene firma ad hoc y no está notarizada: Gatekeeper bloquea el primer arranque, así que pulsa con el botón derecho → **Abrir** (o ejecuta `xattr -dr com.apple.quarantine /Applications/iworkflow.app`).

### Actualizaciones

La aplicación consulta la última release de GitHub 15 segundos después de arrancar, cada 6 horas y desde el menú de la bandeja (**Check for updates**). No instala nada sin preguntar:

- **Windows (NSIS) y Linux (AppImage)**: ofrece la nueva versión, la descarga (solo los bloques modificados cuando es posible), verifica el sha512 indicado en `latest*.yml` y, tras aceptar, detiene el servidor, reemplaza la aplicación y la vuelve a arrancar (`electron-updater`). Un build de Linux que no sea AppImage solo avisa.
- **macOS**: solo avisa y abre la página de la release, porque macOS no instala por sí mismo una aplicación sin firma de código válida, y la aplicación solo tiene firma ad hoc. Descarga el nuevo `.dmg` y reemplaza la aplicación.
- `IWF_UPDATE_CHECK=off` desactiva la comprobación. Un repositorio privado necesita `GH_TOKEN` en el entorno; uno público no necesita nada. Las ejecuciones de desarrollo (`electron .`) nunca comprueban actualizaciones.

### Workflow de release

`.github/workflows/release.yml` construye los archivos del servidor y, en los mismos runners de Linux, macOS (Apple Silicon e Intel) y Windows, los instaladores de escritorio: ejecuta los tests del lanzador (`npm test` en `electron/`), después `make electron-build` sobre el servidor que acaba de compilar, y sube `iworkflow-desktop-<plataforma>`. Al subir un tag `v*`, todo se publica en una release de GitHub; también puede lanzarse a mano (*Run workflow*). La versión del instalador sale de `electron/package.json`: cámbiala con `make bump-version VERSION=x.y.z`. El workflow sube también `latest-linux.yml` / `latest.yml` (y los `.blockmap`) junto a los instaladores: son lo que lee `electron-updater`, así que un release sin ellos no puede actualizar las instalaciones de Windows ni de Linux. macOS no los tiene.
