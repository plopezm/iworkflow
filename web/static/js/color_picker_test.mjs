import test from 'node:test';
import assert from 'node:assert/strict';

import { applyCustomColor } from './color_picker.js';

function fakeSwatch() {
  const style = new Map();
  const radio = { value: '', checked: false, dispatchEvent() {} };
  const swatch = { style: { setProperty: (k, v) => style.set(k, v) }, querySelector: () => radio };
  const input = { value: '', closest: () => swatch };
  return { input, radio, style };
}

test('a picked color becomes the selected value of the custom swatch', () => {
  const { input, radio, style } = fakeSwatch();
  input.value = '#A1B2C3';
  assert.equal(applyCustomColor(input), true);
  assert.equal(radio.value, '#a1b2c3');
  assert.equal(radio.checked, true);
  assert.equal(style.get('--ws'), '#a1b2c3');
});

test('a value that is not #rrggbb leaves the swatch untouched', () => {
  const { input, radio } = fakeSwatch();
  input.value = 'red';
  assert.equal(applyCustomColor(input), false);
  assert.equal(radio.value, '');
  assert.equal(radio.checked, false);
});
