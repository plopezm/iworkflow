// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';
import { resetCreatedForm, setupGroupManager } from './group_manager.js';

function form(matches) {
  return { matches: (selector) => matches && selector === 'form[data-group-create]', reset() { this.resets = (this.resets || 0) + 1; } };
}

test('the create form is emptied once its request succeeded', () => {
  const created = form(true);
  assert.equal(resetCreatedForm({ target: created, detail: { successful: true } }), true);
  assert.equal(created.resets, 1);
});

test('a failed create keeps what the user typed', () => {
  const created = form(true);
  assert.equal(resetCreatedForm({ target: created, detail: { successful: false } }), false);
  assert.equal(created.resets, undefined);
});

test('requests of other elements are left alone', () => {
  const other = form(false);
  assert.equal(resetCreatedForm({ target: other, detail: { successful: true } }), false);
  assert.equal(other.resets, undefined);
  assert.equal(resetCreatedForm({ target: null, detail: { successful: true } }), false);
});

test('setupGroupManager listens to htmx:afterRequest on the body', () => {
  const listeners = {};
  setupGroupManager({ body: { addEventListener: (type, listener) => { listeners[type] = listener; } } });
  assert.equal(typeof listeners['htmx:afterRequest'], 'function');
});
