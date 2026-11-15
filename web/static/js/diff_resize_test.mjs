// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';
import { STORAGE_KEY } from './diff_resize.js';
import { draggedWidth } from './panel_resize.js';

test('the diff tree grows when the pointer moves right and keeps the panel limits', () => {
  assert.equal(draggedWidth('left', 256, 300, 340, 1600), 296);
  assert.equal(draggedWidth('left', 256, 300, 0, 1600), 200);
});

test('the width is stored under its own key, apart from the side panels', () => {
  assert.equal(STORAGE_KEY, 'iwf.diff.tree.width');
});
