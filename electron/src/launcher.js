'use strict';

// Pure helpers for the Electron shell. Nothing here touches Electron or the filesystem so it
// can be tested with `node --test`.

const DEFAULT_SITE_ADDR = '127.0.0.1:9000';
const LOOPBACK_HOST = '127.0.0.1';
const TOKEN_LENGTH = 64;
const TOKEN_PATTERN = new RegExp(`[?&]token=([0-9a-fA-F]{${TOKEN_LENGTH}})(?![0-9a-zA-Z])`);
const LISTEN_PATTERN = /(?:escuchando en |msg=listening url=)http:\/\/(?:\[([^\]]*)\]|([^:/\s]+)):(\d+)/;
/** Exit code with which the server asks its supervisor to start it again. */
const RESTART_EXIT_CODE = 75;
const MAC_EXTRA_PATHS = ['/opt/homebrew/bin', '/usr/local/bin'];
const UNSPECIFIED_HOSTS = new Set(['0.0.0.0', '::', '']);

/** Host and port the server will listen on, following the same default as the binary. */
function parseSiteAddr(env) {
  const raw = env.IWORKFLOW_ADDR || DEFAULT_SITE_ADDR;
  const match = /^(?:\[([^\]]*)\]|([^:]*)):(\d+)$/.exec(raw);
  const port = match ? Number(match[3]) : NaN;
  if (!match || port < 1 || port > 65535) {
    throw new Error(`Invalid IWORKFLOW_ADDR: ${raw}`);
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
 * Environment for the server process. It deliberately leaves `IWORKFLOW_ADDR` alone: the saved
 * network-access choice decides the bind address. `IWF_SUPERVISED` tells it that the shell
 * respawns it after a restart request, so it exits instead of starting a copy of itself.
 * `userIwfHome` is the `IWF_HOME` stored in the Windows user environment: the installer writes
 * it, but a process started by that same installer does not inherit it, so it fills the gap
 * when the process environment lacks one.
 */
function buildServerEnv(base, platform = process.platform, userIwfHome = '') {
  const env = { ...base, NO_COLOR: '1', IWF_SUPERVISED: '1' };
  if (!String(base.IWF_HOME ?? '').trim() && userIwfHome) env.IWF_HOME = userIwfHome;
  if (platform === 'darwin') env.PATH = withMacPaths(base.PATH);
  return env;
}

/** `IWF_HOME` value in the output of `reg query HKCU\\Environment /v IWF_HOME`, or '' when absent. */
function parseRegIwfHome(output) {
  const match = /^\s*IWF_HOME\s+REG_(?:EXPAND_)?SZ\s+(.+?)\s*$/m.exec(String(output ?? ''));
  return match ? match[1] : '';
}

/**
 * Apps launched from Finder inherit a minimal `PATH` without Homebrew, so the server would not
 * find `git` installed there. Append the standard locations that are missing.
 */
function withMacPaths(current) {
  const entries = (current ?? '').split(':').filter(Boolean);
  for (const dir of MAC_EXTRA_PATHS) if (!entries.includes(dir)) entries.push(dir);
  return entries.join(':');
}

/**
 * Address the server announced in its startup log, if the line is that announcement. The saved
 * port lives in the database, so this is the only reliable source when the environment is unset.
 */
function extractListenAddr(line) {
  const match = LISTEN_PATTERN.exec(line);
  if (!match) return null;
  const port = Number(match[3]);
  return port >= 1 && port <= 65535 ? { host: match[1] ?? match[2], port } : null;
}

/** Access token printed in the startup link, if the server demands one. */
function extractAccessToken(line) {
  const match = TOKEN_PATTERN.exec(line);
  return match ? match[1] : null;
}

/** Whether the line is progress of a data folder move, which makes the server start slowly on purpose. */
function isMoveProgress(line) {
  return /data folder move:/.test(line);
}

/**
 * A start is late when nothing happened for `timeoutMs`: since the launch, or since the last progress
 * line of a data folder move, whichever is later. Copying gigabytes legitimately takes minutes.
 */
function isStartTimedOut(now, startedAt, lastProgressAt, timeoutMs) {
  return now - Math.max(startedAt, lastProgressAt) >= timeoutMs;
}

/** Whether the server exited because the user asked to restart it. */
function isRestartExit(code) {
  return code === RESTART_EXIT_CODE;
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
  RESTART_EXIT_CODE,
  isRestartExit,
  parseSiteAddr,
  serverBinaryName,
  serverArgs,
  buildServerEnv,
  parseRegIwfHome,
  extractAccessToken,
  extractListenAddr,
  windowUrl,
  isTrustedUrl,
  isMoveProgress,
  isStartTimedOut,
};
