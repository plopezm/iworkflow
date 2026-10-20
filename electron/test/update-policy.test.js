'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const {
  UPDATE_REPO,
  latestReleaseApiUrl,
  isNewerVersion,
  parseLatestRelease,
  updateStrategy,
} = require('../src/update-policy');

test('windows and AppImage builds update themselves', () => {
  assert.equal(updateStrategy({ platform: 'win32', packaged: true, env: {} }), 'auto');
  assert.equal(
    updateStrategy({ platform: 'linux', packaged: true, env: { APPIMAGE: '/opt/iworkflow.AppImage' } }),
    'auto',
  );
});

test('macOS only notifies because the app is not signed with a certificate', () => {
  assert.equal(updateStrategy({ platform: 'darwin', packaged: true, env: {} }), 'notify');
});

test('a linux build that is not an AppImage cannot replace itself, so it only notifies', () => {
  assert.equal(updateStrategy({ platform: 'linux', packaged: true, env: {} }), 'notify');
});

test('development runs never check for updates', () => {
  assert.equal(updateStrategy({ platform: 'win32', packaged: false, env: {} }), 'off');
});

test('IWF_UPDATE_CHECK=off disables the check on every platform', () => {
  for (const value of ['off', 'OFF', '0', 'false']) {
    const env = { IWF_UPDATE_CHECK: value, APPIMAGE: '/x' };
    for (const platform of ['win32', 'linux', 'darwin']) {
      assert.equal(updateStrategy({ platform, packaged: true, env }), 'off', `${platform} ${value}`);
    }
  }
  assert.equal(updateStrategy({ platform: 'win32', packaged: true, env: { IWF_UPDATE_CHECK: 'on' } }), 'auto');
});

test('compares versions numerically, ignoring a leading v', () => {
  assert.equal(isNewerVersion('0.16.0', 'v0.17.0'), true);
  assert.equal(isNewerVersion('0.9.0', '0.10.0'), true);
  assert.equal(isNewerVersion('1.0.0', '0.99.99'), false);
  assert.equal(isNewerVersion('0.16.0', '0.16.0'), false);
  assert.equal(isNewerVersion('0.16.0', 'v0.16.1'), true);
});

test('a malformed or pre-release candidate is never offered', () => {
  assert.equal(isNewerVersion('0.16.0', 'nightly'), false);
  assert.equal(isNewerVersion('0.16.0', '0.17.0-rc.1'), false);
  assert.equal(isNewerVersion('garbage', '0.17.0'), false);
});

test('the latest release is read from the GitHub API', () => {
  assert.equal(UPDATE_REPO, 'plopezm/iworkflow');
  assert.equal(
    latestReleaseApiUrl(UPDATE_REPO),
    'https://api.github.com/repos/plopezm/iworkflow/releases/latest',
  );
  assert.deepEqual(
    parseLatestRelease({ tag_name: 'v0.17.0', html_url: 'https://github.com/x/y/releases/tag/v0.17.0', draft: false, prerelease: false }),
    { version: '0.17.0', url: 'https://github.com/x/y/releases/tag/v0.17.0' },
  );
});

test('drafts, pre-releases and unusable payloads yield no release', () => {
  assert.equal(parseLatestRelease({ tag_name: 'v1.0.0', html_url: 'u', draft: true }), null);
  assert.equal(parseLatestRelease({ tag_name: 'v1.0.0', html_url: 'u', prerelease: true }), null);
  assert.equal(parseLatestRelease({ html_url: 'u' }), null);
  assert.equal(parseLatestRelease(null), null);
});
