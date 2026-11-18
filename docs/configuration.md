# Configuration

> 🇪🇸 [Versión en castellano](es/configuration.md) · [← README](../README.md)

## Environment variables

Environment variables (`.env` is supported, see `.env.example`):

| Variable | Default |
|---|---|
| `IWF_HOME` | `~/.iworkflow`. Data folder; everything below is derived from it and cannot be set separately: `workspaces.db` (SQLite, automatic migrations), `worktrees/`, `attachments/` (images attached to tasks; deleted when the task ends), `agents-files/` (`AGENTS.md` library), `cursor-sessions/` and `agy-sessions/` (each task's ACP session id, to resume it), `models/` (speech model for voice dictation; `make model` downloads it), `tls/` and `logs/` |
| `IWF_GENTLE_SHELL_BIN` | `gentle-shell` (launched as `--mode rpc` with `GENTLE_SHELL_INTERACTIVE_HOST=1`) |
| `IWF_PI_BIN` | `pi` (`pi` runtime, launched as `--mode rpc`) |
| `IWF_CLAUDE_BIN` | `claude` (`claude` runtime: Claude Code with `-p` and the stream-json protocol; chosen in the runtime selector of the side menu; accepts file edits, asks for confirmation for the rest and reads the project's `AGENTS.md`) |
| `IWF_CURSOR_BIN` | `agent` (`cursor` runtime: Cursor CLI in ACP server mode, `agent acp`; requires having signed in with `agent login`) |
| `IWF_AGY_BIN` | `agy` (`agy` runtime: Antigravity CLI in ACP server mode, `agy acp`) |
| `IWF_GENTLE_PI_CONFIG_HOME` | `~/.pi/gentle-ai` (contains `profiles.json` and `models.json`) |
| `IWF_TLS` | `auto`, the only mode: HTTPS is always on, with a self-signed certificate created in `tls/` in the data folder and reused (see [HTTPS](#https)). `off` serves plain HTTP; tests and CI only, logs a warning |
| `IWF_TLS_PORT` | unset: the usual listen port. Overrides the listen port (1024–65535); HTTPS is served there, e.g. `9000` → `https://host:9000` |
| `IWF_TLS_CERT` / `IWF_TLS_KEY` | PEM files of your own certificate and key, used instead of the self-signed one; set both or neither |
| `IWF_ENGRAM_URL` | unset: off. Base URL of the local Engram HTTP API, for example `http://127.0.0.1:7437`. When set to a valid http or https URL, it turns on the Engram memory of each workspace (see [Engram memory](integrations.md#engram-memory)). An invalid value keeps it off and logs one warning at startup |
| `IWF_CLAUDE_CREDENTIALS` | `~/.claude/.credentials.json`. OAuth credentials of Claude Code, read by the [usage bar](workspace.md#usage-limits) |
| `IWF_CLAUDE_USAGE_URL` | unset: the real Anthropic usage endpoint. Overrides the Claude subscription usage endpoint |
| `IWF_CODEX_SESSIONS_DIR` | `~/.codex/sessions`. Where Codex writes its session files, read for the Codex limits |


## Data folder

Moving the data folder: open **Settings → Resources**, type the new absolute folder in the **Data folder** card and press *Move folder*. The server restarts, copies everything (database, worktrees, attachments, models...), checks the copy, repoints the stored paths and the git worktrees, switches to the new folder and only then deletes the old one. It refuses while a task is starting, running or finishing (note: Claude and pi resume a conversation by folder, so tasks that already started begin a new conversation after the move; Cursor and agy tasks keep theirs), and if the new folder is not empty, not writable, nested with the old one or too small. If anything fails the old folder stays untouched and in use, and the card shows why. The choice is stored in a one-line pointer file in the per-user configuration folder (`~/.config/iworkflow/data-home` on Linux, `~/Library/Application Support/iworkflow/data-home` on macOS, `%APPDATA%\iworkflow\data-home` on Windows), outside the data folder, so updates and reinstalls keep it. `IWF_HOME` (environment or `.env`) wins over the pointer; while it is set the card is read-only. Alternatively set `IWF_HOME` yourself (on Windows the installer asks for it and stores it as a user environment variable) after copying the old `~/.iworkflow` there. On Windows the home directory is taken from `USERPROFILE` when `HOME` is missing.

## Logging

The server writes to stderr and, at the same time, to `$IWF_HOME/logs/iworkflow.log` (default `~/.iworkflow/logs/iworkflow.log`). The file location is fixed; these variables tune it:

| Variable | Default | Effect |
|---|---|---|
| `IWF_LOG_LEVEL` | `info` | `debug` also logs every HTTP request and extra detail; any other value keeps `info`. |
| `IWF_LOG_MAX_MB` | `10` | Size in MB at which the log file is rotated. A value below 1 or not a number falls back to the default. |
| `IWF_LOG_BACKUPS` | `5` | Rotated files kept, gzip-compressed; older ones are deleted. Same fallback as above. |

The startup line with the access link contains the token, so treat the log as sensitive.

## Voice dictation

The prompt field of the create and edit task forms has a microphone button. Record, listen to the recording, confirm it and the server transcribes it with [sherpa-onnx](https://github.com/k2-fsa/sherpa-onnx) and the Qwen3-ASR 0.6B model; the text is appended to the prompt as a new paragraph. The browser needs a secure origin (HTTPS or `localhost`) to open the microphone, and recordings are limited to 60 seconds.

1. `make model` downloads the pre-converted model (about 840 MB) into `$IWF_HOME/models/` (default `~/.iworkflow/models/`) and verifies it against the sha256 sums pinned in `asr-model.sha256`.
2. The speech recognizer is the only cgo code in the project, behind the `sherpa` build tag. `make build`, `make dev` and `make dist` enable it (`GO_TAGS=sherpa`); `make build GO_TAGS=` gives a CGO-free binary where dictation answers "not available". `make dist` also copies the sherpa-onnx shared libraries of the current OS next to the binary (`lib/` on Linux and macOS, the same folder on Windows). Only one transcription runs at a time; a second request gets `429` and is retried by the user.

## HTTPS

The server serves only HTTPS, on its listen port (for example `https://127.0.0.1:9000`). It is always on. Browsers expose the microphone (voice dictation) only in a secure context, so the microphone works from other devices once their browser accepts the certificate.

```bash
make dev                       # https://127.0.0.1:3000
make run                       # same, from make dist
```

Without `IWF_TLS_CERT` and `IWF_TLS_KEY`, the server creates a self-signed certificate on first use in `$IWF_HOME/tls/`, valid for `localhost`, `127.0.0.1` and the machine's network IP. It is reused until it nears expiry (30 days) or the IP changes, and then recreated at the next start. The browser warns about an untrusted certificate the first time: accept it once per device.

`IWF_TLS=off` serves plain HTTP instead. It is for tests and CI only; the server logs a warning when it starts that way.

`IWF_TLS_PORT` overrides the listen port (1024–65535). There is one listener only: HTTPS is served on that port (`9000` → `https://host:9000`). **Settings → Network access** shows the HTTPS link and QR code. `--addr` / `IWORKFLOW_ADDR` set the listen address.

## Agent runtimes

The runtime selector in the side menu supports `gentle-shell`, `pi`, `claude`, `cursor` and `agy`. The choice is fixed when each task starts.

| Runtime | Binary | Capabilities |
|---|---|---|
| `gentle-shell` / `pi` | `IWF_GENTLE_SHELL_BIN` / `IWF_PI_BIN` | all (steer, follow-up, dialogs, resume, model, thinking, subagents) |
| `claude` | `IWF_CLAUDE_BIN` | dialogs, resume, model and thinking |
| `cursor` | `IWF_CURSOR_BIN` | dialogs, resume and model (the model list is obtained from the agent itself over ACP). File edits are accepted; everything else asks for confirmation |
| `agy` | `IWF_AGY_BIN` | dialogs, resume, model (obtained over ACP) and thinking. File edits are accepted; everything else asks for confirmation |

## Executable arguments

On startup, `iworkflow` opens the web UI in the browser (`https://<host>:<port>` of the server; if it listens on `0.0.0.0`, `127.0.0.1` is used).

| Argument | Effect |
|---|---|
| `--no-open` | Starts only the server, without opening the browser. |
| `--addr HOST:PORT` | Listen address (also the `IWORKFLOW_ADDR` variable). Pins the address: the saved network settings are ignored. Default `127.0.0.1:3000` (`:9000` when packaged). |
| `--site-root DIR` | Folder with the static assets. Default: `site/` next to the executable, else `web/static`. |
| `-h`, `--help` | Shows the help. |

Examples: `./iworkflow --no-open`, `make run ARGS=--no-open`, `make dev ARGS=--no-open`.

In development (`make dev`) the server listens only on `127.0.0.1`.

## Access from other devices

By default the server listens only on `127.0.0.1` (this computer). To reach it from your phone, turn on **Settings → Network access** and restart the server: it then listens on `0.0.0.0` with the same port, and the card shows the address to open on the phone. The choice is stored in the database. The same card lets you change the **server port** (1024–65535, saved in the database and applied on the next start; a packaged server defaults to 9000). If `--addr` or `IWORKFLOW_ADDR` is set, it always wins and neither setting has an effect. The card shows a **QR code** and a selector with the machine's network interfaces (for example ethernet and Tailscale, each with its IP): pick the one your phone can reach and scan the code. With the token required, iworkflow opens already signed in; without it, the code just holds the plain address.

Whenever the server is reachable from the network (this setting, or a non-loopback `--addr` such as the `0.0.0.0:9000` of `make dist` / `make run`) it demands an **access token** by default:

- The QR code and the link on the card carry the token (`https://<ip>:<port>/?token=…`) and follow the selected interface. Opening it stores the token in a cookie and redirects to a clean URL, so it does not stay in the address bar or the history. Without the cookie, the server shows a login page where you can paste the token.
- At startup the server logs that link, which is how you get it on a machine without a browser.
- **Generate a new token** on the card invalidates the old one at once; devices using it must sign in again.
- The requirement is optional: untick **Require the access token** (applies on the next start) to serve the network without it. Then anyone who can reach the port can run commands as your user, so use it only on a network you fully trust.
- Loopback-only servers (the default) never ask for the token.
- The startup log line with the link contains the token: treat the server logs as sensitive.
- Writes (`POST`, etc.) that a browser labels with an `Origin` different from the `Host` it targeted are rejected with 403, so another website cannot change these settings through your browser. A reverse proxy must therefore forward the original `Host` header.

The token is a shared secret. Its connection uses HTTPS with a self-signed certificate that no authority vouches for, so on an untrusted network prefer a private network such as Tailscale (see below) or your own certificate (`IWF_TLS_CERT` / `IWF_TLS_KEY`).
