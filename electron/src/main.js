'use strict';

// Electron shell: starts the iworkflow server as a child process and shows it in a window.
// The server keeps its own network settings, so a phone can still reach it over the local
// network or Tailscale; closing the window only hides it and "Quit" stops the server.

const { app, BrowserWindow, Menu, Tray, dialog, nativeImage, shell } = require('electron');
const { spawn } = require('node:child_process');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const readline = require('node:readline');
const {
  buildServerEnv,
  extractAccessToken,
  extractListenAddr,
  isRestartExit,
  isTrustedUrl,
  parseSiteAddr,
  serverArgs,
  serverBinaryName,
  windowUrl,
} = require('./launcher');
const { startUpdater } = require('./updater');

const READY_TIMEOUT_MS = 60_000;
const POLL_INTERVAL_MS = 200;
const STOP_GRACE_MS = 5_000;
const LOG_TAIL_LINES = 20;
const ICON_PATH = path.join(__dirname, '..', 'build', 'icon.png');
const APP_TITLE = 'iworkflow';
// The macOS menu bar is shorter than a Linux/Windows tray.
const TRAY_ICON_SIZE = process.platform === 'darwin' ? 18 : 22;

let server = null;
let mainWindow = null;
let tray = null;
let quitting = false;
let accessToken = null;
// Address the window may show; it changes when a restart moves the server to another port.
let serverAddr = null;
const logTail = [];

/** Directory holding the server binary and its `site/` folder. */
function serverDir() {
  if (process.env.IWF_SERVER_DIR) return process.env.IWF_SERVER_DIR;
  return app.isPackaged
    ? path.join(process.resourcesPath, 'server')
    : path.join(__dirname, '..', '..', 'dist');
}

function rememberLog(line) {
  logTail.push(line);
  if (logTail.length > LOG_TAIL_LINES) logTail.shift();
}

function isPortFree(addr) {
  return new Promise((resolve) => {
    const probe = net.createServer();
    probe.once('error', () => resolve(false));
    probe.listen({ port: addr.port, host: addr.host === '' ? undefined : addr.host }, () => {
      probe.close(() => resolve(true));
    });
  });
}

function canConnect(addr) {
  return new Promise((resolve) => {
    const host = ['0.0.0.0', '::', ''].includes(addr.host) ? '127.0.0.1' : addr.host;
    const socket = net.connect({ port: addr.port, host });
    socket.once('connect', () => {
      socket.destroy();
      resolve(true);
    });
    socket.once('error', () => resolve(false));
  });
}

async function waitUntilReady(addr) {
  const deadline = Date.now() + READY_TIMEOUT_MS;
  while (Date.now() < deadline) {
    if (!server) throw new Error('The server stopped before it was ready.');
    if (await canConnect(addr)) return;
    await new Promise((resolve) => setTimeout(resolve, POLL_INTERVAL_MS));
  }
  throw new Error('The server did not start listening in time.');
}

function startServer(onListening) {
  const binary = path.join(serverDir(), serverBinaryName(process.platform));
  const child = spawn(binary, serverArgs(), {
    cwd: os.homedir(),
    env: buildServerEnv(process.env),
    stdio: ['ignore', 'pipe', 'pipe'],
    windowsHide: true,
  });
  server = child;
  const onLine = (line) => {
    rememberLog(line);
    console.log(`[server] ${line}`);
    accessToken = extractAccessToken(line) ?? accessToken;
    const announced = extractListenAddr(line);
    if (announced) onListening(announced);
  };
  readline.createInterface({ input: child.stdout }).on('line', onLine);
  readline.createInterface({ input: child.stderr }).on('line', onLine);
  child.once('error', (error) => rememberLog(String(error)));
  child.once('exit', (code, signal) => {
    server = null;
    if (!quitting && isRestartExit(code)) {
      void restartServer();
    } else if (!quitting) {
      fatal(`The server stopped unexpectedly (${signal ?? `code ${code}`}).`);
    }
  });
}

/** The server exited on purpose (settings changed): start it again and reload the window. */
async function restartServer() {
  try {
    accessToken = null;
    const addr = await startAndFindAddr();
    await waitUntilReady(addr);
    serverAddr = addr;
    if (mainWindow) await mainWindow.loadURL(windowUrl(addr, accessToken));
  } catch (error) {
    fatal(error instanceof Error ? error.message : String(error));
  }
}

function stopServer() {
  const child = server;
  if (!child) return Promise.resolve();
  return new Promise((resolve) => {
    const timer = setTimeout(() => child.kill('SIGKILL'), STOP_GRACE_MS);
    child.once('exit', () => {
      clearTimeout(timer);
      resolve();
    });
    child.kill();
  });
}

function fatal(message) {
  if (quitting) return;
  quitting = true;
  dialog.showErrorBox(APP_TITLE, `${message}\n\n${logTail.join('\n')}`.trim());
  stopServer().finally(() => app.exit(1));
}

function showWindow() {
  if (!mainWindow) return;
  if (mainWindow.isMinimized()) mainWindow.restore();
  mainWindow.show();
  mainWindow.focus();
}

function createWindow(addr) {
  serverAddr = addr;
  mainWindow = new BrowserWindow({
    title: APP_TITLE,
    width: 1400,
    height: 900,
    show: false,
    icon: ICON_PATH,
    webPreferences: { contextIsolation: true, nodeIntegration: false, sandbox: true },
  });
  mainWindow.once('ready-to-show', () => mainWindow.show());
  // The server stays up for remote devices: closing the window only hides it.
  mainWindow.on('close', (event) => {
    if (!quitting && tray) {
      event.preventDefault();
      mainWindow.hide();
    }
  });
  mainWindow.on('closed', () => {
    mainWindow = null;
  });
  const keepLocal = (event, url) => {
    if (!isTrustedUrl(url, serverAddr)) {
      event.preventDefault();
      if (/^https?:\/\//.test(url)) shell.openExternal(url);
    }
  };
  mainWindow.webContents.on('will-navigate', keepLocal);
  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    if (/^https?:\/\//.test(url) && !isTrustedUrl(url, serverAddr)) shell.openExternal(url);
    return { action: 'deny' };
  });
  return mainWindow.loadURL(windowUrl(addr, accessToken));
}

function createTray(updater) {
  try {
    tray = new Tray(nativeImage.createFromPath(ICON_PATH).resize({ width: TRAY_ICON_SIZE, height: TRAY_ICON_SIZE }));
  } catch {
    tray = null;
    return;
  }
  tray.setToolTip(`${APP_TITLE} (server running)`);
  tray.setContextMenu(
    Menu.buildFromTemplate([
      { label: 'Open iworkflow', click: showWindow },
      ...(updater.enabled
        ? [{ label: 'Check for updates', click: () => void updater.checkNow() }]
        : []),
      { type: 'separator' },
      { label: 'Quit and stop server', click: () => app.quit() },
    ]),
  );
  tray.on('click', showWindow);
}

/** The address is pinned by the environment, or announced by the server once it is bound. */
async function startAndFindAddr() {
  if (process.env.LEPTOS_SITE_ADDR) {
    const addr = parseSiteAddr(process.env);
    if (!(await isPortFree(addr))) {
      throw new Error(
        `Port ${addr.port} is already in use (another iworkflow instance?). ` +
          'Close it, or set LEPTOS_SITE_ADDR to another address.',
      );
    }
    startServer(() => {});
    return addr;
  }
  return new Promise((resolve, reject) => {
    const timer = setTimeout(
      () => reject(new Error('The server did not announce its address in time.')),
      READY_TIMEOUT_MS,
    );
    startServer((announced) => {
      clearTimeout(timer);
      resolve(announced);
    });
  });
}

async function main() {
  const addr = await startAndFindAddr();
  await waitUntilReady(addr);
  const updater = startUpdater({
    getWindow: () => mainWindow,
    // The installer cannot replace files while the server binary is running.
    beforeInstall: async () => {
      quitting = true;
      await stopServer();
    },
    recover: () => {
      app.relaunch();
      app.exit(0);
    },
  });
  createTray(updater);
  await createWindow(addr);
}

if (!app.requestSingleInstanceLock()) {
  app.quit();
} else {
  app.on('second-instance', showWindow);
  // macOS: clicking the Dock icon while the window is hidden brings it back.
  app.on('activate', showWindow);
  // A plain SIGTERM/SIGINT skips `before-quit`; route it there so the server never outlives us.
  for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => app.quit());
  // Last resort if the process exits some other way.
  process.on('exit', () => {
    if (server) server.kill('SIGKILL');
  });
  app.on('before-quit', (event) => {
    if (quitting && !server) return;
    quitting = true;
    event.preventDefault();
    stopServer().finally(() => app.exit(0));
  });
  // Stay alive with no windows: the server is the product; the tray brings the window back.
  app.on('window-all-closed', () => {
    if (!tray) app.quit();
  });
  app.whenReady().then(() =>
    main().catch((error) => fatal(error instanceof Error ? error.message : String(error))),
  );
}
