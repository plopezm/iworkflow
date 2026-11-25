// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';
import { deletableRows, syncAll, toggleAll } from './branch_select_all.js';

function fakeList(states) {
  const rows = states.map((disabled) => ({ checked: false, disabled }));
  const all = { checked: false, indeterminate: false };
  const root = {
    querySelectorAll: () => rows,
    querySelector: () => all,
  };
  return { root, rows, all };
}

test('select all ticks only the rows that can be deleted', () => {
  const { root, rows } = fakeList([false, true, false]);
  toggleAll(true, root);
  assert.deepEqual(rows.map((r) => r.checked), [true, false, true]);
  assert.equal(deletableRows(root).length, 2);
  toggleAll(false, root);
  assert.deepEqual(rows.map((r) => r.checked), [false, false, false]);
});

test('the select-all box is ticked when every deletable row is, mixed when some are', () => {
  const { root, rows, all } = fakeList([false, true, false]);
  rows[0].checked = true;
  syncAll(root);
  assert.equal(all.checked, false);
  assert.equal(all.indeterminate, true);
  rows[2].checked = true;
  syncAll(root);
  assert.equal(all.checked, true);
  assert.equal(all.indeterminate, false);
});

test('a list with nothing to delete leaves the box clear', () => {
  const { root, all } = fakeList([true]);
  syncAll(root);
  assert.equal(all.checked, false);
  assert.equal(all.indeterminate, false);
});
