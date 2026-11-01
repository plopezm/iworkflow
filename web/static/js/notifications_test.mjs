// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';
import { canShow, shouldRequest } from './notifications.js';
import { isSaveShortcut } from './code.js';

test('only an undecided permission triggers the prompt', () => {
  assert.equal(shouldRequest('default'), true);
  assert.equal(shouldRequest('granted'), false);
  assert.equal(shouldRequest('denied'), false);
  assert.equal(shouldRequest(undefined), false, 'no Notification API');
});

test('only a granted permission allows showing a notification', () => {
  assert.equal(canShow('granted'), true);
  assert.equal(canShow('default'), false);
  assert.equal(canShow('denied'), false);
});

test('Ctrl+S and Cmd+S save, other keys do not', () => {
  assert.equal(isSaveShortcut({ ctrlKey: true, metaKey: false, key: 's' }), true);
  assert.equal(isSaveShortcut({ ctrlKey: false, metaKey: true, key: 'S' }), true);
  assert.equal(isSaveShortcut({ ctrlKey: false, metaKey: false, key: 's' }), false);
  assert.equal(isSaveShortcut({ ctrlKey: true, metaKey: false, key: 'a' }), false);
});

test('only tasks that were not already waiting raise a notification', async () => {
  const { newlyNeedingAttention } = await import('./notifications.js');
  const rows = [
    { id: 'a', title: 'Task A', body: 'Needs attention' },
    { id: 'b', title: 'Task B', body: 'Needs attention' },
  ];
  assert.deepEqual(newlyNeedingAttention(new Set(['a']), rows), [rows[1]]);
  assert.deepEqual(newlyNeedingAttention(new Set(['a', 'b']), rows), []);
  assert.deepEqual(newlyNeedingAttention(new Set(), []), []);
});
