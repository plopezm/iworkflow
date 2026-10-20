'use strict';

// The update flow with every side effect injected, so it can be tested without Electron.
// updater.js wires the real dialog, shell, fetch and electron-updater into it.

const { isNewerVersion, latestReleaseApiUrl, parseLatestRelease } = require('./update-policy');

const API_TIMEOUT_MS = 15_000;
const CHECK_TIMEOUT_MS = 60_000;

/** Rejects when `promise` does not settle within `ms`; the timer never keeps the process alive. */
function withTimeout(promise, ms, label) {
  let timer;
  const timeout = new Promise((_, reject) => {
    timer = setTimeout(() => reject(new Error(`${label} timed out after ${ms} ms`)), ms);
    timer.unref?.();
  });
  return Promise.race([promise, timeout]).finally(() => clearTimeout(timer));
}

/**
 * Returns `{ checkNow(manual) }`. Dependencies:
 * `strategy` ('auto' | 'notify'), `appVersion`, `ask(options)` (dialog, resolves `{ response }`),
 * `openExternal(url)`, `fetchImpl`, `ghToken`, `loadAutoUpdater()`, `beforeInstall()` (stops what
 * would block the installer), `recover()` (brings the app back when the install did not start),
 * `log(message)`, optional `checkTimeoutMs`.
 */
function createUpdateFlow(deps) {
  const { strategy, appVersion, ask, openExternal, fetchImpl, ghToken, loadAutoUpdater, beforeInstall, recover, log } =
    deps;
  const checkTimeoutMs = deps.checkTimeoutMs ?? CHECK_TIMEOUT_MS;
  let busy = false;
  let declinedVersion = null;

  const describe = (error) => (error instanceof Error ? error.message : String(error));
  // Automatic checks stay quiet; only a check asked by the user reports "up to date" or errors.
  const report = (manual, message) => (manual ? ask({ message }) : Promise.resolve());
  const upToDate = (manual) => report(manual, `iworkflow ${appVersion} is up to date.`);

  async function notifyOnly(manual) {
    const headers = { Accept: 'application/vnd.github+json', 'User-Agent': 'iworkflow-desktop' };
    if (ghToken) headers.Authorization = `Bearer ${ghToken}`;
    const response = await fetchImpl(latestReleaseApiUrl(deps.repo), {
      headers,
      signal: AbortSignal.timeout(API_TIMEOUT_MS),
    });
    if (!response.ok) throw new Error(`GitHub answered ${response.status}`);
    const release = parseLatestRelease(await response.json());
    if (!release || !isNewerVersion(appVersion, release.version)) return upToDate(manual);
    if (!manual && declinedVersion === release.version) return;
    declinedVersion = release.version;
    const { response: choice } = await ask({
      message: `iworkflow ${release.version} is available.`,
      detail: 'Download the new installer from the release page and run it.',
      buttons: ['Open release page', 'Later'],
      defaultId: 0,
      cancelId: 1,
    });
    if (choice === 0) await openExternal(release.url);
  }

  async function installAutomatically(manual) {
    const autoUpdater = loadAutoUpdater();
    autoUpdater.autoDownload = false;
    autoUpdater.autoInstallOnAppQuit = false;
    const found = await withTimeout(autoUpdater.checkForUpdates(), checkTimeoutMs, 'update check');
    if (!found || !found.isUpdateAvailable) return upToDate(manual);
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
    try {
      await autoUpdater.downloadUpdate();
    } catch (error) {
      // The user asked for this download, so a failure is never silent.
      log(`[update] ${describe(error)}`);
      await ask({ type: 'error', message: `Could not download iworkflow ${version}.`, detail: 'Try again later.' });
      return;
    }
    const { response: restart } = await ask({
      message: `iworkflow ${version} is ready to install.`,
      detail: 'The app restarts, and the server restarts with it.',
      buttons: ['Restart now', 'Later'],
      defaultId: 0,
      cancelId: 1,
    });
    if (restart !== 0) return;
    await beforeInstall();
    // The server is already stopped: if the installer fails, synchronously or through the
    // updater's `error` event, restart the app instead of leaving it half dead. A successful
    // install quits the app, so any error seen from here on means the install did not start.
    let recovered = false;
    const failInstall = (error) => {
      if (recovered) return;
      recovered = true;
      log(`[update] ${describe(error)}`);
      recover();
    };
    autoUpdater.on('error', failInstall);
    try {
      autoUpdater.quitAndInstall(false, true);
    } catch (error) {
      failInstall(error);
    }
  }

  async function checkNow(manual = false) {
    if (busy) return;
    busy = true;
    try {
      await (strategy === 'auto' ? installAutomatically(manual) : notifyOnly(manual));
    } catch (error) {
      log(`[update] ${describe(error)}`);
      await report(manual, 'Could not check for updates. Try again later.');
    } finally {
      busy = false;
    }
  }

  return { checkNow };
}

module.exports = { createUpdateFlow, withTimeout };
