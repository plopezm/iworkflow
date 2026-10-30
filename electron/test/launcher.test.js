'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const {
  DEFAULT_SITE_ADDR,
  RESTART_EXIT_CODE,
  isRestartExit,
  parseSiteAddr,
  serverBinaryName,
  buildServerEnv,
  serverArgs,
  extractAccessToken,
  extractListenAddr,
  windowUrl,
  isTrustedUrl,
} = require('../src/launcher');

test('defaults to the server default address when LEPTOS_SITE_ADDR is unset', () => {
  assert.equal(DEFAULT_SITE_ADDR, '127.0.0.1:9000');
  assert.deepEqual(parseSiteAddr({}), { host: '127.0.0.1', port: 9000 });
});

test('reads an explicit LEPTOS_SITE_ADDR', () => {
  assert.deepEqual(parseSiteAddr({ LEPTOS_SITE_ADDR: '0.0.0.0:9000' }), { host: '0.0.0.0', port: 9000 });
  assert.deepEqual(parseSiteAddr({ LEPTOS_SITE_ADDR: '[::]:9100' }), { host: '::', port: 9100 });
});

test('rejects a malformed LEPTOS_SITE_ADDR', () => {
  assert.throws(() => parseSiteAddr({ LEPTOS_SITE_ADDR: 'nonsense' }), /LEPTOS_SITE_ADDR/);
  assert.throws(() => parseSiteAddr({ LEPTOS_SITE_ADDR: '127.0.0.1:99999' }), /LEPTOS_SITE_ADDR/);
});

test('binary name depends on the platform', () => {
  assert.equal(serverBinaryName('win32'), 'iworkflow.exe');
  assert.equal(serverBinaryName('linux'), 'iworkflow');
});

test('the server is started without opening a browser', () => {
  assert.deepEqual(serverArgs(), ['--no-open']);
});

test('server env never pins the listen address, so the saved network setting still applies', () => {
  const env = buildServerEnv({ PATH: '/usr/bin', HOME: '/home/u' }, 'linux');
  assert.equal(env.LEPTOS_SITE_ADDR, undefined);
  assert.equal(env.PATH, '/usr/bin');
  assert.equal(env.HOME, '/home/u');
  assert.equal(env.LEPTOS_ENV, 'PROD');
  assert.equal(env.NO_COLOR, '1');
});

test('server env keeps an address the user pinned and does not touch the database path', () => {
  const env = buildServerEnv({ LEPTOS_SITE_ADDR: '0.0.0.0:9000', IWF_DATABASE_URL: '/data/x.db' }, 'linux');
  assert.equal(env.LEPTOS_SITE_ADDR, '0.0.0.0:9000');
  assert.equal(env.IWF_DATABASE_URL, '/data/x.db');
  const untouched = buildServerEnv({}, 'linux');
  assert.equal(untouched.IWF_DATABASE_URL, undefined);
});

test('on macOS the server PATH gains the Homebrew directories a Finder launch lacks', () => {
  const env = buildServerEnv({ PATH: '/usr/bin:/bin' }, 'darwin');
  assert.equal(env.PATH, '/usr/bin:/bin:/opt/homebrew/bin:/usr/local/bin');
  const present = buildServerEnv({ PATH: '/opt/homebrew/bin:/usr/bin' }, 'darwin');
  assert.equal(present.PATH, '/opt/homebrew/bin:/usr/bin:/usr/local/bin');
  assert.equal(buildServerEnv({}, 'darwin').PATH, '/opt/homebrew/bin:/usr/local/bin');
});

test('other platforms keep PATH untouched', () => {
  assert.equal(buildServerEnv({ PATH: '/usr/bin' }, 'linux').PATH, '/usr/bin');
  assert.equal(buildServerEnv({ PATH: 'C:\\bin' }, 'win32').PATH, 'C:\\bin');
});

test('extracts the access token from the startup link', () => {
  const token = 'a'.repeat(64);
  const line = `INFO iworkflow: acceso desde la red protegido con token; enlace: http://192.168.1.5:3000/?token=${token}`;
  assert.equal(extractAccessToken(line), token);
});

test('ignores lines without a valid token', () => {
  assert.equal(extractAccessToken('INFO iworkflow: escuchando en http://127.0.0.1:3000'), null);
  assert.equal(extractAccessToken('enlace: http://h:3000/?token=short'), null);
  assert.equal(extractAccessToken(`?token=${'z'.repeat(64)}`), null);
});

test('window url targets loopback for unspecified hosts and carries the token once', () => {
  assert.equal(windowUrl({ host: '0.0.0.0', port: 3000 }, null), 'http://127.0.0.1:3000/');
  assert.equal(windowUrl({ host: '::', port: 3000 }, null), 'http://127.0.0.1:3000/');
  assert.equal(windowUrl({ host: '127.0.0.1', port: 3000 }, 'f'.repeat(64)), `http://127.0.0.1:3000/?token=${'f'.repeat(64)}`);
});

test('only the local server origin is trusted inside the window', () => {
  const addr = { host: '0.0.0.0', port: 3000 };
  assert.equal(isTrustedUrl('http://127.0.0.1:3000/tasks', addr), true);
  assert.equal(isTrustedUrl('http://127.0.0.1:3001/', addr), false);
  assert.equal(isTrustedUrl('https://example.com/', addr), false);
  assert.equal(isTrustedUrl('not a url', addr), false);
});

test('reads the address the server announces in its startup log', () => {
  assert.deepEqual(extractListenAddr('INFO iworkflow: escuchando en http://127.0.0.1:9100'), { host: '127.0.0.1', port: 9100 });
  assert.deepEqual(extractListenAddr('INFO iworkflow: escuchando en http://0.0.0.0:9000'), { host: '0.0.0.0', port: 9000 });
  assert.deepEqual(extractListenAddr('INFO iworkflow: escuchando en http://[::1]:9000'), { host: '::1', port: 9000 });
  assert.deepEqual(extractListenAddr('time=2026-10-28T20:00:00.000+01:00 level=INFO msg=listening url=http://127.0.0.1:9100'), { host: '127.0.0.1', port: 9100 });
});

test('ignores log lines that are not the listen announcement', () => {
  assert.equal(extractListenAddr('INFO iworkflow: acceso desde la red; enlace: http://192.168.1.5:9000/?token=abc'), null);
  assert.equal(extractListenAddr('escuchando en http://127.0.0.1:99999'), null);
  assert.equal(extractListenAddr('hello'), null);
});

test('the server is told that the shell respawns it', () => {
  assert.equal(buildServerEnv({}, 'linux').IWF_SUPERVISED, '1');
});

test('only the restart exit code asks for a respawn', () => {
  assert.equal(RESTART_EXIT_CODE, 75);
  assert.equal(isRestartExit(75), true);
  assert.equal(isRestartExit(0), false);
  assert.equal(isRestartExit(1), false);
  assert.equal(isRestartExit(null), false);
});
