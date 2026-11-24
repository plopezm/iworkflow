# iworkflow

> 🇬🇧 English version: [README.md](README.md)

**iworkflow es un ADE (Agentic Development Environment)**: una aplicación web autoalojada para lanzar, seguir y revisar varios agentes de código con IA en paralelo. En lugar de editar ficheros tú mismo, gestionas **tareas**: cada una ejecuta un agente en su propio git worktree y revisas el diff antes de que nada llegue a tu rama.

<p align="center">
  <img src="docs/media/task-graph.gif" alt="Grafo de tareas de iworkflow: las tareas encadenadas esperan a sus predecesoras, y una tarea integrada arranca sus sucesoras automáticamente" width="900" />
  <br />
  <sub>Encadena tareas en un tablero; cuando una se integra, las que dependen de ella arrancan solas.</sub>
</p>

> **Estado:** temprano y en desarrollo activo. Se ejecuta en tu propia máquina, no tiene cuentas de usuario (solo un token de acceso opcional para la red) y es *source-available* (PolyForm Shield 1.0.0), no open source aprobado por la OSI. Consulta [Licencia](#licencia).

## Características

- **Agentes en paralelo, un solo lugar**: lanza y dirige `gentle-shell`, `pi`, `claude`, `cursor` o `agy` a la vez.
- **Aislamiento por diseño**: cada tarea tiene su propio **git worktree**, así que los agentes nunca se pisan.
- **Revisar, no rehacer**: cuando un agente termina, lees su diff y lo integras (`merge --no-ff`) o lo descartas.
- **Grafo de tareas**: encadena tareas en un tablero; una tarea arranca sola en cuanto sus predecesoras están integradas.
- **Límites de uso de un vistazo**: una barra de uso muestra las ventanas de cinco horas y semanal de Claude y Codex, y las tareas detenidas por un límite se reanudan cuando este se renueva.
- **Hooks de tareas**: ejecuta scripts o prompts de agente antes o después de que una tarea arranque o termine, por proyecto.
- **Integrado**: GitLab / GitHub (estado de MR/PR y CI), Jira Server / Data Center y memoria de Engram.
- **Mobile-first**: consulta el progreso, responde a una pregunta del agente e integra desde el móvil (a través de Tailscale).
- **Espacio de trabajo con pestañas, estilo IDE**: tablero, tarea, diff, terminal y editor de ficheros como pestañas arrastrables y divisibles, además de dictado por voz para los prompts.
- **Búsqueda por contenido**: escribe en el árbol del proyecto para dejar solo los ficheros con coincidencias (literal por defecto, con interruptores de mayúsculas, regex y palabra completa; ripgrep con respaldo en Go); al abrir una coincidencia, el fichero salta a ella, y `Ctrl+F`, `F3` / `Shift+F3` o las flechas recorren las coincidencias.
- **Aplicación de escritorio**: un instalador de Electron para Linux, macOS y Windows envuelve el mismo servidor.

iworkflow no impone ningún flujo de trabajo: cada agente usa la configuración y los plugins que ya tengas instalados, más el `AGENTS.md` del proyecto. Por debajo, los agentes se manejan por RPC (o ACP / stream-json, según el runtime). Está construido con Go (`net/http` + `chi`), SSR con `templ` y `htmx`, Tailwind CSS, `sqlc` y SQLite.

## Arranque rápido

```bash
git clone https://github.com/plopezm/iworkflow.git
cd iworkflow
make dev                    # https://127.0.0.1:3000 (autofirmado: acepta el aviso del navegador una vez)
```

Necesitas `git`, Go, `templ`, `tailwindcss` y al menos un CLI de agente instalado y con sesión iniciada. Después registra un workspace, escribe un prompt, sigue al agente en vivo, revisa el diff e intégralo. El recorrido completo está en [Arranque rápido](docs/es/getting-started.md).

> **Seguridad:** iworkflow solo se protege con un token de acceso opcional y lanza agentes que ejecutan comandos con tu usuario. No lo expongas a redes no confiables.

<p align="center">
  <img src="docs/media/task-review.gif" alt="Revisión de una tarea en iworkflow: lanzar un agente, seguirlo en vivo, revisar su diff e integrarlo" width="800" />
  <br />
  <sub>Una tarea de principio a fin: lanzar, seguir al agente en vivo, revisar el diff e integrar.</sub>
</p>

### Paquetes de escritorio (Wails, en curso)

`make wails-package` construye el shell de escritorio Wails (`cmd/iworkflow-desktop`) con el servidor preparado a su lado y escribe el paquete del sistema actual en `dist/wails/`. En Linux es un `.tar.gz`: descomprímelo y ejecuta `./iworkflow-desktop` (el servidor está en `server/`; `iworkflow.desktop` es una plantilla de lanzador cuyos `Exec` e `Icon` puedes apuntar a la carpeta extraída). El `.dmg` de macOS y el instalador NSIS de Windows aún no están verificados. Los instaladores de Electron de `make electron-build` siguen siendo la aplicación de escritorio distribuida.

Cada paquete escribe también el recurso de actualización `iworkflow-<version>-<goos>-<goarch>.tar.gz` (una carpeta superior con los archivos de la app que reemplaza el actualizador; `iworkflow-updater` está junto a `iworkflow-desktop`) y `SHA256SUMS` con el sha256 de cada archivo de la ejecución, que el actualizador comprueba antes de instalar. Los recursos de actualización de macOS y Windows aún no están verificados.

## Más información

| Tema | Qué encontrarás |
|---|---|
| [Arranque rápido](docs/es/getting-started.md) | Requisitos, primer workspace, primera tarea, revisión e integración |
| [Espacio de trabajo, tablero y ciclo de vida](docs/es/workspace.md) | Espacio con pestañas, grafo de tareas, estados de la tarea, [límites de uso](docs/es/workspace.md#límites-de-uso), [hooks de tareas](docs/es/workspace.md#hooks-de-tareas) |
| [Integraciones](docs/es/integrations.md) | GitLab y GitHub, Jira, perfiles y modelos de Gentle AI, memoria de Engram |
| [Configuración](docs/es/configuration.md) | Variables de entorno, carpeta de datos, logs, dictado por voz, HTTPS, runtimes de agente, acceso por red |
| [Producción](docs/es/production.md) | Build de release, arranque y despliegue del servidor |
| [Aplicación de escritorio](docs/es/desktop.md) | Instalador de Electron, actualizaciones y workflow de release |
| [Móvil y Tailscale](docs/es/mobile.md) | Usarlo desde el teléfono, de forma segura |
| [Desarrollo](docs/es/development.md) | Objetivos de make, tests y grabación de los GIFs |
| [Arquitectura](ARCHITECTURE.es.md) | Mapa del código: módulos, runtimes, persistencia, frontend |
| [Reglas del proyecto](AGENTS.md) | Reglas de código y de proceso para contribuidores y agentes |

## Licencia

Copyright 2026 Pablo López (plopezm). Distribuido bajo la [licencia PolyForm Shield 1.0.0](LICENSE).

Puedes usar, modificar y distribuir iworkflow libremente, también dentro de empresas y para trabajo comercial, pero no usarlo para ofrecer un producto que compita con él (por ejemplo, revenderlo u ofrecerlo como servicio alojado). Es una licencia *source-available*, no una licencia open source aprobada por la OSI.

Los recursos de terceros incluidos mantienen su propia licencia: xterm.js (MIT) y highlight.js (BSD-3-Clause) en `web/static/vendor/`, y las fuentes Instrument Serif y Martian Mono (SIL OFL 1.1) en `web/static/fonts/`. Las dependencias de Go se declaran en `go.mod`; las atribuciones están en [NOTICE](NOTICE).

## Apoya el proyecto

<p align="center">
  <a href="https://buymeacoffee.com/plopezm" target="_blank" rel="noopener">
    <img src="https://img.buymeacoffee.com/button-api/?text=Buy%20me%20a%20coffee&emoji=%E2%98%95&slug=plopezm&button_colour=FFDD00&font_colour=000000&font_family=Cookie&outline_colour=000000&coffee_colour=ffffff" alt="Invítame a un café" height="50" />
  </a>
</p>

Si iworkflow te ahorra tiempo, puedes invitarme a un café en **[buymeacoffee.com/plopezm](https://buymeacoffee.com/plopezm)**. ¡Gracias! ☕
