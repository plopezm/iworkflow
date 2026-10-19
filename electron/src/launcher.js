'use strict';

// Pure helpers for the Electron shell. Nothing here touches Electron or the filesystem so it
// can be tested with `node --test`.

const DEFAULT_SITE_ADDR = '127.0.0.1:3000';
const LOOPBACK_HOST = '127.0.0.1';
const TOKEN_LENGTH = 64;
const TOKEN_PATTERN = new RegExp(`[?&]token=([0-9a-fA-F]{${TOKEN_LENGTH}})(?![0-9a-zA-Z])`);
const UNSPECIFIED_HOSTS = new Set(['0.0.0.0', '::', '']);

/** Host and port the server will listen on, following the same default as the binary. */
function parseSiteAddr(env) {
  const raw = env.LEPTOS_SITE_ADDR || DEFAULT_SITE_ADDR;
  const match = /^(?:\[([^\]]*)\]|([^:]*)):(\d+)$/.exec(raw);
  const port = match ? Number(match[3]) : NaN;
  if (!match || port < 1 || port > 65535) {
    throw new Error(`Invalid LEPTOS_SITE_ADDR: ${raw}`);
  }
  return { host: match[1] ?? match[2], port };
}

function serverBinaryName(platform) {
  return platform === 'win32' ? 'iworkflow.exe' : 'iworkflow';
}

/** The window replaces the browser the server would otherwise open. */
function serverArgs() {
  return ['--no-open'];
}

/**
 * Environment for the server process. It deliberately leaves `LEPTOS_SITE_ADDR` and
 * `IWF_DATABASE_URL` alone: the saved network-access choice decides the bind address and the
 * database stays where it always was.
 */
function buildServerEnv(base) {
  return { ...base, LEPTOS_ENV: 'PROD', NO_COLOR: '1' };
}

/** Access token printed in the startup link, if the server demands one. */
function extractAccessToken(line) {
  const match = TOKEN_PATTERN.exec(line);
  return match ? match[1] : null;
}

function localHost(addr) {
  return UNSPECIFIED_HOSTS.has(addr.host) ? LOOPBACK_HOST : addr.host;
}

function originOf(addr) {
  const host = localHost(addr);
  return `http://${host.includes(':') ? `[${host}]` : host}:${addr.port}`;
}

/** URL the window loads; the one-shot `?token=` link makes the server set its session cookie. */
function windowUrl(addr, token) {
  return `${originOf(addr)}/${token ? `?token=${token}` : ''}`;
}

/** Only the local server origin may be shown inside the window. */
function isTrustedUrl(url, addr) {
  try {
    return new URL(url).origin === new URL(originOf(addr)).origin;
  } catch {
    return false;
  }
}

module.exports = {
  DEFAULT_SITE_ADDR,
  parseSiteAddr,
  serverBinaryName,
  serverArgs,
  buildServerEnv,
  extractAccessToken,
  windowUrl,
  isTrustedUrl,
};
