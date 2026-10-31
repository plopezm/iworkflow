import assert from 'node:assert/strict';
import { test } from 'node:test';
import { draggedHeight } from './composer_resize.js';

test('dragging down grows the textarea by the pointer travel', () => {
  assert.equal(draggedHeight(100, 200, 260), 160);
});

test('dragging up shrinks it but never under the minimum', () => {
  assert.equal(draggedHeight(100, 200, 150), 72);
  assert.equal(draggedHeight(300, 200, 100, 72), 200);
});
