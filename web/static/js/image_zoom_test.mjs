// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';
import { isZoomed, setZoom, onClick } from './image_zoom.js';

function fakeImage() {
  const attrs = new Set();
  const img = {
    hasAttribute: (n) => attrs.has(n),
    setAttribute: (n) => attrs.add(n),
    removeAttribute: (n) => attrs.delete(n),
  };
  img.closest = (selector) => (selector === 'img[data-zoomable]' ? img : null);
  return img;
}

test('a click on a zoomable image toggles between fitted and real size', () => {
  const img = fakeImage();
  onClick({ target: img });
  assert.equal(isZoomed(img), true);
  onClick({ target: img });
  assert.equal(isZoomed(img), false);
});

test('a click elsewhere changes nothing', () => {
  const img = fakeImage();
  onClick({ target: { closest: () => null } });
  onClick({ target: null });
  assert.equal(isZoomed(img), false);
});

test('setZoom sets and clears the state', () => {
  const img = fakeImage();
  setZoom(img, true);
  assert.equal(isZoomed(img), true);
  setZoom(img, false);
  assert.equal(isZoomed(img), false);
});
