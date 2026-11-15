# Production

> 🇪🇸 [Versión en castellano](es/production.md) · [← README](../README.md)

Prerequisites: Go, GNU make, `templ`, `sqlc` and the Tailwind CSS CLI (the same ones as `make generate`).

The Makefile builds and runs the app in release mode:

```bash
make dist     # 1. compile in release and package into dist/
make run      # 2. start what is already packaged
make prod     # both steps in a row (dist + run)
```

Typical flow: run `make dist` once (and again after every update), then `make run` whenever you want to start the server. `ARGS` is passed to the binary (e.g. `ARGS=--no-open`, see [Executable arguments](configuration.md#executable-arguments)). The server listens on `http://0.0.0.0:9000` by default; change it with `SITE_ADDR`:

```bash
make run SITE_ADDR=127.0.0.1:3000 ARGS=--no-open
make dist DIST_DIR=/opt/iworkflow     # package somewhere else
```

`make dist` regenerates templ, sqlc and CSS, builds a stripped `iworkflow` binary and copies
the static `site/` folder into `dist/`. Both are required.

`make run` starts the binary with `--addr` and `--site-root` from the current directory, so the `.env` in that directory
is applied too. Make variables:

| Variable | Default |
|---|---|
| `DIST_DIR` | `dist/` at the repo root |
| `SITE_ADDR` | `0.0.0.0:9000` (all interfaces) |
| `ARGS` | none (flags for the binary) |

> **Security:** the app launches agents that run commands as your user. Its only
> protection is the access token (see [Access from other devices](configuration.md#access-from-other-devices)),
> which is demanded whenever the bind is not loopback unless you switch it off, and it travels
> over plain HTTP. Restrict access with a firewall, a private network (Tailscale/WireGuard) or a
> reverse proxy with TLS. To limit it to the local machine:
> `make run SITE_ADDR=127.0.0.1:3000`.

To deploy on another machine, just copy `dist/` and launch it with
`./iworkflow --addr 0.0.0.0:9000`
from inside that folder.
