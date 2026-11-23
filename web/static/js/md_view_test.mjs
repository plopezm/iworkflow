import test from 'node:test';
import assert from 'node:assert/strict';
import { normalizeMode } from './md_view.js';

test('only "rendered" selects the rendered view', () => {
  assert.equal(normalizeMode('rendered'), 'rendered');
  assert.equal(normalizeMode('code'), 'code');
  assert.equal(normalizeMode(null), 'code');
  assert.equal(normalizeMode('x'), 'code');
});
