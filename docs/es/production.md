# Producción

> 🇬🇧 [English version](../production.md) · [← README](../../README.es.md)

Requisitos previos: Go, GNU make, `templ`, `sqlc` y la CLI de Tailwind CSS (los mismos que `make generate`).

El Makefile compila y ejecuta la aplicación en modo release:

```bash
make dist     # 1. compila en release y empaqueta en dist/
make run      # 2. arranca lo que ya está empaquetado
make prod     # ambos pasos seguidos (dist + run)
```

Flujo habitual: ejecuta `make dist` una vez (y de nuevo tras cada actualización) y `make run` cada vez que quieras arrancar el servidor. `ARGS` se pasa al binario (p. ej. `ARGS=--no-open`; ver [Argumentos del ejecutable](configuration.md#argumentos-del-ejecutable)). Por defecto, el servidor escucha en `http://0.0.0.0:9000`; cámbialo con `SITE_ADDR`:

```bash
make run SITE_ADDR=127.0.0.1:3000 ARGS=--no-open
make dist DIST_DIR=/opt/iworkflow     # empaquetar en otra ruta
```

`make dist` regenera templ, sqlc y CSS, compila un binario `iworkflow` sin símbolos y copia
la carpeta estática `site/` en `dist/`. Ambos son necesarios.

`make run` arranca el binario con `--addr` y `--site-root` desde el directorio actual, así que el `.env` de ese directorio
también se aplica. Variables de make:

| Variable | Por defecto |
|---|---|
| `DIST_DIR` | `dist/` en la raíz del repositorio |
| `SITE_ADDR` | `0.0.0.0:9000` (todas las interfaces) |
| `ARGS` | ninguno (flags del binario) |

> **Seguridad:** la aplicación lanza agentes que ejecutan comandos con tu usuario. Su única
> protección es el token de acceso (ver [Acceso desde otros dispositivos](configuration.md#acceso-desde-otros-dispositivos)),
> que se exige siempre que el bind no sea loopback, salvo que lo desactives, y viaja por HTTP sin
> cifrar. Restringe el acceso con un firewall, una red privada (Tailscale/WireGuard) o un
> reverse proxy con TLS. Para limitarlo a la máquina local:
> `make run SITE_ADDR=127.0.0.1:3000`.

Para desplegarlo en otra máquina, basta con copiar `dist/` y lanzarlo con
`./iworkflow --addr 0.0.0.0:9000`
desde dentro de esa carpeta.
