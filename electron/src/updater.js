'use strict';

// Update flow of the desktop app. The releases of the GitHub repository are the source:
// `auto` installs through electron-updater (reads latest*.yml, checks the sha512, downloads and
// swaps the app), `notify` only points the user to the release page. Decisions live in
// update-policy.js. A private repository needs GH_TOKEN in the environment.

const { app, dialog, shell } = require('electron');
const {
  UPDATE_REPO,
  isNewerVersion,
  latestReleaseApiUrl,
  parseLatestRelease,
  updateStrategy,
} = require('./update-policy');

const FIRST_CHECK_DELAY_MS = 15_000;
const CHECK_INTERVAL_MS = 6 * 60 * 60 * 1000;
const API_TIMEOUT_MS = 15_000;

/**
 * Starts the periodic check and returns `{ enabled, checkNow }`.
 * `beforeInstall` must stop everything that would block the installer (the server process).
 */
function startUpdater({ getWindow, beforeInstall }) {
  const strategy = updateStrategy({ platform: process.platform, packaged: app.isPackaged, env: process.env });
  if (strategy === 'off') return { enabled: false, checkNow: async () => {} };

  let busy = false;
  let declinedVersion = null;

  const ask = (options) => {
    const parent = getWindow();
    const shown = { type: 'info', noLink: true, ...options };
    return parent && parent.isVisible() ? dialog.showMessageBox(parent, shown) : dialog.showMessageBox(shown);
  };

  // Automatic checks stay quiet; only a check asked by the user reports "up to date" or errors.
  const report = (manual, message) => (manual ? ask({ message }) : Promise.resolve());

  async function notifyOnly(manual) {
    const headers = { Accept: 'application/vnd.github+json', 'User-Agent': 'iworkflow-desktop' };
    if (process.env.GH_TOKEN) headers.Authorization = `Bearer ${process.env.GH_TOKEN}`;
    const response = await fetch(latestReleaseApiUrl(UPDATE_REPO), {
      headers,
      signal: AbortSignal.timeout(API_TIMEOUT_MS),
    });
    if (!response.ok) throw new Error(`GitHub answered ${response.status}`);
    const release = parseLatestRelease(await response.json());
    if (!release || !isNewerVersion(app.getVersion(), release.version)) {
      return report(manual, `iworkflow ${app.getVersion()} is up to date.`);
    }
    if (!manual && declinedVersion === release.version) return;
    declinedVersion = release.version;
    const { response: choice } = await ask({
      message: `iworkflow ${release.version} is available.`,
      detail: 'Download the new installer from the release page and run it.',
      buttons: ['Open release page', 'Later'],
      defaultId: 0,
      cancelId: 1,
    });
    if (choice === 0) await shell.openExternal(release.url);
  }

  async function installAutomatically(manual) {
    // Loaded here so a notify-only build never touches the module.
    const { autoUpdater } = require('electron-updater');
    autoUpdater.autoDownload = false;
    autoUpdater.autoInstallOnAppQuit = false;
    const found = await autoUpdater.checkForUpdates();
    if (!found || !found.isUpdateAvailable) {
      return report(manual, `iworkflow ${app.getVersion()} is up to date.`);
    }
    const version = found.updateInfo.version;
    if (!manual && declinedVersion === version) return;
    declinedVersion = version;
    const { response: download } = await ask({
      message: `iworkflow ${version} is available.`,
      detail: 'It downloads in the background and asks before restarting.',
      buttons: ['Download', 'Later'],
      defaultId: 0,
      cancelId: 1,
    });
    if (download !== 0) return;
    await autoUpdater.downloadUpdate();
    const { response: restart } = await ask({
      message: `iworkflow ${version} is ready to install.`,
      detail: 'The app restarts, and the server restarts with it.',
      buttons: ['Restart now', 'Later'],
      defaultId: 0,
      cancelId: 1,
    });
    if (restart !== 0) return;
    await beforeInstall();
    autoUpdater.quitAndInstall(false, true);
  }

  async function checkNow(manual = false) {
    if (busy) return;
    busy = true;
    try {
      await (strategy === 'auto' ? installAutomatically(manual) : notifyOnly(manual));
    } catch (error) {
      console.error(`[update] ${error instanceof Error ? error.message : error}`);
      await report(manual, 'Could not check for updates. Try again later.');
    } finally {
      busy = false;
    }
  }

  setTimeout(() => void checkNow(false), FIRST_CHECK_DELAY_MS).unref();
  setInterval(() => void checkNow(false), CHECK_INTERVAL_MS).unref();
  return { enabled: true, checkNow: () => checkNow(true) };
}

module.exports = { startUpdater };
