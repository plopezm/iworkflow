// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';
import { isCurrentPath } from './sidebar_active.js';

test('a link is current only when its path is the focused page', () => {
  assert.equal(isCurrentPath('/settings', '/settings'), true);
  assert.equal(isCurrentPath('/settings', '/workspaces'), false);
});

test('the home board does not match every page by prefix', () => {
  assert.equal(isCurrentPath('/', '/settings'), false);
  assert.equal(isCurrentPath('/', '/'), true);
});

test('no focused page or no link path marks nothing', () => {
  assert.equal(isCurrentPath('/settings', ''), false);
  assert.equal(isCurrentPath('', '/settings'), false);
  assert.equal(isCurrentPath(null, undefined), false);
});
