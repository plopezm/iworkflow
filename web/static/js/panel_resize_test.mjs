// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';
import { clampWidth, draggedWidth, parseStoredWidth, steppedWidth, MIN_WIDTH, MAX_WIDTH } from './panel_resize.js';

test('a left panel grows when the pointer moves right, a right panel when it moves left', () => {
  assert.equal(draggedWidth('left', 300, 300, 340, 1600), 340);
  assert.equal(draggedWidth('right', 400, 900, 860, 1600), 440);
});

test('the width never goes under the minimum nor over the maximum or half the viewport', () => {
  assert.equal(clampWidth(50, 1600), MIN_WIDTH);
  assert.equal(clampWidth(5000, 4000), MAX_WIDTH);
  assert.equal(clampWidth(700, 1000), 500);
});

test('a stored width is read only when it is a number; anything else means no stored width', () => {
  assert.equal(parseStoredWidth('320'), 320);
  assert.equal(parseStoredWidth(null), null);
  assert.equal(parseStoredWidth('abc'), null);
  assert.equal(parseStoredWidth('-5'), null);
});

test('arrow keys move the edge toward the arrow, whatever the side', () => {
  assert.equal(steppedWidth('left', 300, 'ArrowRight', 1600), 316);
  assert.equal(steppedWidth('right', 300, 'ArrowLeft', 1600), 316);
  assert.equal(steppedWidth('left', 300, 'ArrowLeft', 1600), 284);
  assert.equal(steppedWidth('left', 300, 'Enter', 1600), null);
});
