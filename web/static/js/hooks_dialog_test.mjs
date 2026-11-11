// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';
import { dialogsToOpen, dismissPathOf } from './hooks_dialog.js';

test('only the dialogs under the root that are not open yet are opened', () => {
  const closed = { open: false };
  const opened = { open: true };
  const root = { querySelectorAll: () => [closed, opened] };
  assert.deepEqual(dialogsToOpen(root), [closed]);
});

test('no root gives no dialogs to open', () => {
  assert.deepEqual(dialogsToOpen(null), []);
  assert.deepEqual(dialogsToOpen({}), []);
});

test('the dismiss path is the view the server gave the dialog', () => {
  assert.equal(dismissPathOf({ dataset: { dismissPath: '/hooks/library' } }), '/hooks/library');
  assert.equal(dismissPathOf({ dataset: {} }), '');
});
