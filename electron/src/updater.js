'use strict';

// Update flow of the desktop app. The releases of the GitHub repository are the source:
// `auto` installs through electron-updater (reads latest*.yml, checks the sha512, downloads and
// swaps the app), `notify` only points the user to the release page. Decisions live in
// update-policy.js. A private repository needs GH_TOKEN in the environment.

const { app, dialog, shell } = require('electron');
const { UPDATE_REPO, updateStrategy } = require('./update-policy');
const { createUpdateFlow } = require('./update-flow');

const FIRST_CHECK_DELAY_MS = 15_000;
const CHECK_INTERVAL_MS = 6 * 60 * 60 * 1000;

/**
 * Starts the periodic check and returns `{ enabled, checkNow }`.
 * `beforeInstall` must stop everything that would block the installer (the server process);
 * `recover` brings the app back (server included) when the installer could not be started.
 */
function startUpdater({ getWindow, beforeInstall, recover }) {
  const strategy = updateStrategy({ platform: process.platform, packaged: app.isPackaged, env: process.env });
  if (strategy === 'off') return { enabled: false, checkNow: async () => {} };

  const ask = (options) => {
    const parent = getWindow();
    const shown = { type: 'info', noLink: true, ...options };
    return parent && parent.isVisible() ? dialog.showMessageBox(parent, shown) : dialog.showMessageBox(shown);
  };

  const flow = createUpdateFlow({
    strategy,
    repo: UPDATE_REPO,
    appVersion: app.getVersion(),
    ask,
    openExternal: (url) => shell.openExternal(url),
    fetchImpl: fetch,
    ghToken: process.env.GH_TOKEN,
    // Loaded lazily so a notify-only build never touches the module.
    loadAutoUpdater: () => require('electron-updater').autoUpdater,
    beforeInstall,
    recover,
    log: (message) => console.error(message),
  });
  const checkNow = flow.checkNow;

  setTimeout(() => void checkNow(false), FIRST_CHECK_DELAY_MS).unref();
  setInterval(() => void checkNow(false), CHECK_INTERVAL_MS).unref();
  return { enabled: true, checkNow: () => checkNow(true) };
}

module.exports = { startUpdater };
