# Desktop app (Electron)

> 🇪🇸 [Versión en castellano](es/desktop.md) · [← README](../README.md)

For people who do not want to deal with a terminal, `electron/` wraps the same server in a desktop window and `make electron-build` builds an installer for the platform it runs on (Linux: AppImage, macOS: `.dmg`, Windows: NSIS `.exe`). There is no separate "mobile server" to build: the installer already contains the same `iworkflow` binary, so a phone connects to the server running inside the app (see [Using it from your phone](#using-it-from-your-phone)).

### Build the installer

Prerequisites (once): the ones of [Production](production.md), `git` and Node.js 22+ with npm. Cross-building is not supported: build each installer on its own platform.

```bash
make electron-build
```

It runs, in order:

1. `make dist`: release build, leaving the binary and `site/` in `dist/`.
2. Copies both into `electron/resources/server/` (outside the asar archive, so the binary stays executable).
3. `npm ci` and `electron-builder` for the current platform.
4. Writes the installer to `dist/electron/`: `iworkflow-<version>-linux-x86_64.AppImage`, `iworkflow-<version>-mac-arm64.dmg` (or `mac-x64`) or `iworkflow-<version>-win-x64.exe`.

Other forms:

```bash
make electron-build SKIP_SERVER_BUILD=1   # reuse the server already in dist/
make electron-build DIST_DIR=/some/dir    # use another server package / output location
make electron-dev                         # open the window on dist/ without building an installer
```

On macOS the `.dmg` is built for the architecture of the Mac that builds it (Apple Silicon or Intel); it needs the Xcode Command Line Tools. On Windows run it from **Git Bash** (with GNU make installed).

### Run it

```bash
chmod +x dist/electron/iworkflow-*.AppImage
./dist/electron/iworkflow-*.AppImage          # add --appimage-extract-and-run if FUSE is missing
```

On macOS open the `.dmg` and drag **iworkflow** to Applications. On Windows run the installer and start **iworkflow** from the Start menu. The installer has a page to choose the data folder: if it is not the default `%USERPROFILE%\.iworkflow`, it is stored as the `IWF_HOME` user environment variable (the app also reads it from the registry, so it applies from the first launch). Silent installs and updates leave an existing `IWF_HOME` untouched.

### Using it from your phone

The server inside the app is the regular one, so phone access works exactly as in [Access from other devices](configuration.md#access-from-other-devices):

1. In the app open **Settings → Network access** and turn it on.
2. **Quit the app completely** (tray icon → **Quit and stop server**, or **File → Quit**) and start it again. Closing only the window is not enough: the server keeps running and reads this setting only at startup. It now listens on `0.0.0.0:9000`.
3. Scan the **QR code** on the same card with the phone (same local network, or the same [Tailscale](mobile.md#developing-from-your-phone-with-tailscale) tailnet). It opens already signed in with the access token.

You can also pin the address before launching, e.g. `IWORKFLOW_ADDR=0.0.0.0:9000 ./iworkflow-*.AppImage`; then the saved setting is ignored.

### Behaviour

- The app starts the bundled server (`--no-open`) and shows it in a window. Nothing about the server changes: same data folder (`IWF_HOME`) and database, same `IWORKFLOW_ADDR`, same access token. The window signs itself in when a token is required.
- The server listens on `127.0.0.1:9000` by default; change the port in **Settings → Network access**. The app reads the address the server announces at startup, so it follows the saved port. If the port is busy the app shows an error; free it or choose another one.
- **Closing the window only hides it**, so a phone connected over the local network or Tailscale keeps working. Use the tray icon (**Quit and stop server**) or **File → Quit** to stop the server. On macOS the icon lives in the menu bar and clicking the Dock icon shows the window again (**iworkflow → Quit** or Cmd+Q stops the server). On a Linux desktop without tray support, closing the window quits the app.
- External links open in the system browser; the window itself only shows the local server.
- `git` (and the agent CLIs you use) must be installed and on the `PATH`; the installer does not bundle them. With network access on, Windows and macOS ask once to allow the app through the firewall. On macOS the app adds `/opt/homebrew/bin` and `/usr/local/bin` to its `PATH`, since apps opened from Finder do not inherit your shell's.
- The installers are not code-signed, so Windows SmartScreen warns on first run. On macOS the app is only ad-hoc signed and not notarized: Gatekeeper blocks the first launch, so right-click the app → **Open** (or run `xattr -dr com.apple.quarantine /Applications/iworkflow.app`).

### Updates

The app checks the latest GitHub release 15 seconds after start and every 6 hours, and from the tray menu (**Check for updates**). Nothing is installed without asking:

- **Windows (NSIS) and Linux (AppImage)**: it offers the new version, downloads it (only the changed blocks when possible), verifies the sha512 listed in `latest*.yml` and, after you accept, stops the server, replaces the app and starts it again (`electron-updater`). A Linux build that is not an AppImage only notifies.
- **macOS**: it only notifies and opens the release page, because macOS refuses to self-install an app without a valid code signature and the app is only signed ad hoc. Download the new `.dmg` and replace the app.
- `IWF_UPDATE_CHECK=off` disables the check. A private repository needs `GH_TOKEN` in the environment; a public one needs nothing. Development runs (`electron .`) never check.

### Release workflow

`.github/workflows/release.yml` builds the server archives and, on the same Linux, macOS (Apple Silicon and Intel) and Windows runners, the desktop installers: it runs the launcher tests (`npm test` in `electron/`), then `make electron-build` over the server it just compiled, and uploads `iworkflow-desktop-<platform>`. Pushing a `v*` tag publishes everything in a GitHub release; it can also be started by hand (*Run workflow*). The installer version comes from `electron/package.json`: change it with `make bump-version VERSION=x.y.z`. The workflow also uploads `latest-linux.yml` / `latest.yml` (and the `.blockmap` files) next to the installers: they are what `electron-updater` reads, so a release without them cannot update Windows or Linux installs. macOS has none.
