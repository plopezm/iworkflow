'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const { EventEmitter } = require('node:events');
const { createUpdateFlow, withTimeout } = require('../src/update-flow');

const RELEASE_URL = 'https://github.com/x/y/releases/tag/v0.17.0';

/** Builds a flow whose dialogs answer from `answers` (button index per ask, in order). */
function harness({ strategy, answers = [], autoUpdater, release, fetchFails = false, checkTimeoutMs }) {
  const calls = { asked: [], opened: [], recovered: 0, beforeInstall: 0, logs: [] };
  const queue = [...answers];
  const flow = createUpdateFlow({
    strategy,
    repo: 'x/y',
    appVersion: '0.16.0',
    ask: async (options) => {
      calls.asked.push(options);
      return { response: queue.length ? queue.shift() : 0 };
    },
    openExternal: async (url) => void calls.opened.push(url),
    fetchImpl: async () => {
      if (fetchFails) throw new Error('offline');
      return { ok: true, json: async () => release };
    },
    ghToken: undefined,
    loadAutoUpdater: () => autoUpdater,
    beforeInstall: async () => void (calls.beforeInstall += 1),
    recover: () => void (calls.recovered += 1),
    log: (message) => calls.logs.push(message),
    checkTimeoutMs,
  });
  return { flow, calls };
}

function fakeUpdater({ available = true, download, quitAndInstall } = {}) {
  const updater = new EventEmitter();
  updater.checkForUpdates = async () => ({ isUpdateAvailable: available, updateInfo: { version: '0.17.0' } });
  updater.downloadUpdate = download ?? (async () => {});
  updater.quitAndInstall = quitAndInstall ?? (() => {});
  return updater;
}

const NEWER = { tag_name: 'v0.17.0', html_url: RELEASE_URL, draft: false, prerelease: false };

test('an automatic check that finds nothing stays quiet, a manual one says so', async () => {
  const { flow, calls } = harness({ strategy: 'auto', autoUpdater: fakeUpdater({ available: false }) });
  await flow.checkNow(false);
  assert.equal(calls.asked.length, 0);
  await flow.checkNow(true);
  assert.equal(calls.asked[0].message, 'iworkflow 0.16.0 is up to date.');
});

test('a declined version is not offered again by automatic checks, but a manual check offers it', async () => {
  const { flow, calls } = harness({ strategy: 'auto', answers: [1, 1], autoUpdater: fakeUpdater() });
  await flow.checkNow(false);
  await flow.checkNow(false);
  assert.equal(calls.asked.length, 1);
  await flow.checkNow(true);
  assert.equal(calls.asked.length, 2);
});

test('accepting stops the server before the installer starts', async () => {
  const order = [];
  const updater = fakeUpdater({ quitAndInstall: () => order.push('install') });
  const { flow, calls } = harness({ strategy: 'auto', answers: [0, 0], autoUpdater: updater });
  const original = calls.beforeInstall;
  await flow.checkNow(true);
  assert.equal(original, 0);
  assert.equal(calls.beforeInstall, 1);
  assert.deepEqual(order, ['install']);
  assert.equal(calls.recovered, 0);
});

test('a download failure tells the user and never reaches the installer', async () => {
  const updater = fakeUpdater({
    download: async () => {
      throw new Error('disk full');
    },
    quitAndInstall: () => assert.fail('must not install'),
  });
  const { flow, calls } = harness({ strategy: 'auto', answers: [0], autoUpdater: updater });
  await flow.checkNow(false);
  assert.equal(calls.asked.at(-1).type, 'error');
  assert.equal(calls.beforeInstall, 0);
});

test('a synchronous installer failure recovers the app once', async () => {
  const updater = fakeUpdater({
    quitAndInstall: () => {
      throw new Error('spawn failed');
    },
  });
  const { flow, calls } = harness({ strategy: 'auto', answers: [0, 0], autoUpdater: updater });
  await flow.checkNow(true);
  assert.equal(calls.recovered, 1);
});

test('an asynchronous installer failure (updater error event) recovers the app once', async () => {
  const updater = fakeUpdater({ quitAndInstall: () => {} });
  const { flow, calls } = harness({ strategy: 'auto', answers: [0, 0], autoUpdater: updater });
  await flow.checkNow(true);
  assert.equal(calls.recovered, 0);
  updater.emit('error', new Error('installer exited'));
  updater.emit('error', new Error('again'));
  assert.equal(calls.recovered, 1);
});

test('a check that never answers times out, reports it and frees the next check', async () => {
  const updater = fakeUpdater();
  updater.checkForUpdates = () => new Promise(() => {});
  const { flow, calls } = harness({ strategy: 'auto', autoUpdater: updater, checkTimeoutMs: 20 });
  await flow.checkNow(true);
  assert.match(calls.logs[0], /timed out/);
  assert.equal(calls.asked.at(-1).message, 'Could not check for updates. Try again later.');
  updater.checkForUpdates = async () => ({ isUpdateAvailable: false });
  await flow.checkNow(true);
  assert.equal(calls.asked.at(-1).message, 'iworkflow 0.16.0 is up to date.');
});

test('notify mode opens the release page only when the user accepts', async () => {
  const accepted = harness({ strategy: 'notify', answers: [0], release: NEWER });
  await accepted.flow.checkNow(false);
  assert.deepEqual(accepted.calls.opened, [RELEASE_URL]);
  const later = harness({ strategy: 'notify', answers: [1], release: NEWER });
  await later.flow.checkNow(false);
  assert.deepEqual(later.calls.opened, []);
});

test('network errors are quiet for automatic checks and reported for manual ones', async () => {
  const { flow, calls } = harness({ strategy: 'notify', fetchFails: true });
  await flow.checkNow(false);
  assert.equal(calls.asked.length, 0);
  await flow.checkNow(true);
  assert.equal(calls.asked.length, 1);
});

test('withTimeout passes the result through and rejects a slow promise', async () => {
  assert.equal(await withTimeout(Promise.resolve(7), 50, 'x'), 7);
  await assert.rejects(withTimeout(new Promise(() => {}), 10, 'slow'), /slow timed out/);
});
