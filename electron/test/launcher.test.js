'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const {
  DEFAULT_SITE_ADDR,
  parseSiteAddr,
  serverBinaryName,
  buildServerEnv,
  serverArgs,
  extractAccessToken,
  windowUrl,
  isTrustedUrl,
} = require('../src/launcher');

test('defaults to the server default address when LEPTOS_SITE_ADDR is unset', () => {
  assert.equal(DEFAULT_SITE_ADDR, '127.0.0.1:3000');
  assert.deepEqual(parseSiteAddr({}), { host: '127.0.0.1', port: 3000 });
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
  const env = buildServerEnv({ PATH: '/usr/bin', HOME: '/home/u' });
  assert.equal(env.LEPTOS_SITE_ADDR, undefined);
  assert.equal(env.PATH, '/usr/bin');
  assert.equal(env.HOME, '/home/u');
  assert.equal(env.LEPTOS_ENV, 'PROD');
  assert.equal(env.NO_COLOR, '1');
});

test('server env keeps an address the user pinned and does not touch the database path', () => {
  const env = buildServerEnv({ LEPTOS_SITE_ADDR: '0.0.0.0:9000', IWF_DATABASE_URL: '/data/x.db' });
  assert.equal(env.LEPTOS_SITE_ADDR, '0.0.0.0:9000');
  assert.equal(env.IWF_DATABASE_URL, '/data/x.db');
  const untouched = buildServerEnv({});
  assert.equal(untouched.IWF_DATABASE_URL, undefined);
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
