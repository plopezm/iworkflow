// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';
import { actionValues, addressFor, clampRatio, dropAfter, ratioAt, reconcilePanes } from './dock.js';

test('data-dock-* attributes become the form fields of an action', () => {
  const dataset = { dockAction: 'move_tab', dockTab: '7', dockGroup: '2', dockZone: 'left' };
  assert.deepEqual(actionValues(dataset), { tab: '7', group: '2', zone: 'left' });
});

test('the action attribute itself is never sent as a field', () => {
  assert.equal('action' in actionValues({ dockAction: 'split_group' }), false);
});

test('a dropped tab lands after its target on the right half only', () => {
  const rect = { left: 100, width: 80 };
  assert.equal(dropAfter(170, rect), true);
  assert.equal(dropAfter(140, rect), false, 'exactly the middle counts as before');
  assert.equal(dropAfter(120, rect), false);
});

test('dividers never go below 12% or above 88%', () => {
  assert.equal(clampRatio(0), 0.12);
  assert.equal(clampRatio(1), 0.88);
  assert.equal(clampRatio(0.5), 0.5);
});

test('a divider position becomes a clamped fraction of its container', () => {
  assert.equal(ratioAt(150, 100, 200), 0.25);
  assert.equal(ratioAt(10, 100, 200), 0.12, 'dragged past the start');
});

test('a kept pane is re-attached when its tab still shows the same path (keep-alive)', () => {
  const { reattach, discard } = reconcilePanes(
    [{ id: '1', path: '/terminals' }, { id: '3', path: '/jira' }],
    [{ id: '1', path: '/terminals' }, { id: '2', path: '/models' }],
  );
  assert.deepEqual(reattach, ['1']);
  assert.deepEqual(discard, ['2']);
});

test('a replaced tab gets a fresh pane that loads its new content, not the old pane', () => {
  // replace keeps the tab id but changes its path: the old pane still shows the list.
  const { reattach, discard } = reconcilePanes(
    [{ id: '1', path: '/terminals/s1' }],
    [{ id: '1', path: '/terminals' }],
  );
  assert.deepEqual(reattach, []);
  assert.deepEqual(discard, ['1']);
});

test('no kept panes means nothing to re-attach', () => {
  assert.deepEqual(reconcilePanes([{ id: '1', path: '/a' }], []), { reattach: [], discard: [] });
});

test('the address bar shows the focused tab path, and nothing for an empty group', () => {
  assert.equal(addressFor('/tasks/a'), '/tasks/a');
  assert.equal(addressFor(''), null);
});
