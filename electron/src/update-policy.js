'use strict';

// Pure decisions of the update system, kept free of Electron so they can be unit tested.

const UPDATE_REPO = 'plopezm/iworkflow';
const DISABLED_VALUES = ['off', '0', 'false'];

/**
 * How this installation handles updates:
 * - `auto`: electron-updater downloads and installs the new version (Windows NSIS, Linux AppImage).
 * - `notify`: only tell the user and open the release page. macOS cannot self-install without a
 *   valid code signature (we sign ad hoc), and a Linux build outside an AppImage cannot replace itself.
 * - `off`: no check at all (development runs or `IWF_UPDATE_CHECK=off`).
 */
function updateStrategy({ platform, packaged, env }) {
  if (!packaged) return 'off';
  if (DISABLED_VALUES.includes((env.IWF_UPDATE_CHECK ?? '').toLowerCase())) return 'off';
  if (platform === 'win32') return 'auto';
  if (platform === 'linux' && env.APPIMAGE) return 'auto';
  return 'notify';
}

function latestReleaseApiUrl(repo) {
  return `https://api.github.com/repos/${repo}/releases/latest`;
}

/** `[major, minor, patch]` of a plain `x.y.z` version (leading `v` allowed), else null. */
function parseVersion(text) {
  const match = /^v?(\d+)\.(\d+)\.(\d+)$/.exec(String(text).trim());
  return match ? match.slice(1).map(Number) : null;
}

/** True only when `candidate` is a plain release version strictly greater than `current`. */
function isNewerVersion(current, candidate) {
  const have = parseVersion(current);
  const next = parseVersion(candidate);
  if (!have || !next) return false;
  for (let i = 0; i < 3; i += 1) {
    if (next[i] !== have[i]) return next[i] > have[i];
  }
  return false;
}

/** Reduces a GitHub "latest release" payload to what the notice needs, or null if unusable. */
function parseLatestRelease(payload) {
  if (!payload || payload.draft || payload.prerelease) return null;
  if (typeof payload.tag_name !== 'string' || typeof payload.html_url !== 'string') return null;
  return { version: payload.tag_name.replace(/^v/, ''), url: payload.html_url };
}

module.exports = {
  UPDATE_REPO,
  latestReleaseApiUrl,
  isNewerVersion,
  parseLatestRelease,
  updateStrategy,
};
