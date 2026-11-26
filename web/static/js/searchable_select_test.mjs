// Run with: node --test web/static/js/
// The filter and the keyboard stepping of the searchable branch selects. The DOM wiring is exercised in the browser by
// end2end/go/branch_search.e2e.cjs; here the pure rules that decide what the dropdown shows and which option is active,
// and the enhance lifecycle (idempotency, one change listener per select across rebuilds) on a minimal fake DOM.
import test from 'node:test';
import assert from 'node:assert/strict';
import { enhance, filterOptions, nextActive } from './searchable_select.js';

// The options of a branch select as the script reads them: the label is the visible text, the value is the branch name.
const entries = [
  { index: 0, value: '', label: 'Workspace branch (main)' },
  { index: 1, value: 'origin/develop', label: 'origin/develop' },
  { index: 2, value: 'origin/feature/Login-Form', label: 'Login form' },
  { index: 3, value: 'origin/release/1.0', label: 'Release 1.0' },
];

test('an empty or blank query keeps every option in the select order', () => {
  assert.deepEqual(filterOptions(entries, '').map((e) => e.index), [0, 1, 2, 3]);
  assert.deepEqual(filterOptions(entries, '   ').map((e) => e.index), [0, 1, 2, 3]);
  assert.deepEqual(filterOptions(entries, undefined).map((e) => e.index), [0, 1, 2, 3]);
});

test('the filter is a case-insensitive substring over the label', () => {
  assert.deepEqual(filterOptions(entries, 'DEVEL').map((e) => e.value), ['origin/develop']);
  assert.deepEqual(filterOptions(entries, 'login').map((e) => e.value), ['origin/feature/Login-Form']);
  assert.deepEqual(filterOptions(entries, 'Release 1').map((e) => e.value), ['origin/release/1.0']);
});

test('the filter also matches the branch name when the label shows something else', () => {
  assert.deepEqual(filterOptions(entries, 'feature/login').map((e) => e.value), ['origin/feature/Login-Form']);
  assert.deepEqual(filterOptions(entries, 'origin/').map((e) => e.index), [1, 2, 3]);
});

test('surrounding spaces in the query are ignored', () => {
  assert.deepEqual(filterOptions(entries, '  develop  ').map((e) => e.index), [1]);
});

test('a query with no match leaves an empty list, so the empty-state line can show', () => {
  assert.deepEqual(filterOptions(entries, 'zzz'), []);
});

test('the filter keeps the select index of every option it returns', () => {
  const kept = filterOptions(entries, 'o');
  assert.deepEqual(kept.map((e) => e.index), kept.map((e) => entries.indexOf(e)));
  assert.equal(kept.find((e) => e.value === 'origin/release/1.0').index, 3);
});

test('the filter does not change the entries it is given', () => {
  const before = JSON.stringify(entries);
  filterOptions(entries, 'dev');
  assert.equal(JSON.stringify(entries), before);
});

test('ArrowDown moves to the next option and wraps from the last to the first', () => {
  assert.equal(nextActive(0, 1, 3), 1);
  assert.equal(nextActive(1, 1, 3), 2);
  assert.equal(nextActive(2, 1, 3), 0);
});

test('ArrowUp moves to the previous option and wraps from the first to the last', () => {
  assert.equal(nextActive(2, -1, 3), 1);
  assert.equal(nextActive(0, -1, 3), 2);
});

test('without a highlight the first step down picks the first option and the first step up the last', () => {
  assert.equal(nextActive(-1, 1, 3), 0);
  assert.equal(nextActive(-1, -1, 3), 2);
});

test('a highlight that no longer exists after a filter restarts from the top', () => {
  assert.equal(nextActive(5, 1, 3), 0);
  assert.equal(nextActive(5, -1, 3), 2);
});

test('an empty list has no highlight', () => {
  assert.equal(nextActive(-1, 1, 0), -1);
  assert.equal(nextActive(0, -1, 0), -1);
});

// A minimal DOM: just what enhance() touches while it builds the combobox around a select.
class FakeElement {
  constructor(tag, attrs = {}) {
    this.tag = tag;
    this.attrs = { ...attrs };
    this.children = [];
    this.parentNode = null;
    this.listeners = {};
    this.classList = { add: () => {} };
    this.options = [];
    this.selectedIndex = 0;
    this.value = '';
  }
  setAttribute(name, value) { this.attrs[name] = value; }
  getAttribute(name) { return name in this.attrs ? this.attrs[name] : null; }
  hasAttribute(name) { return name in this.attrs; }
  removeAttribute(name) { delete this.attrs[name]; }
  matches(selector) { return this.tag === 'select' && selector === 'select[data-searchable]' && 'data-searchable' in this.attrs; }
  append(...nodes) { nodes.forEach((node) => { node.parentNode = this; this.children.push(node); }); }
  insertBefore(node, ref) {
    node.parentNode = this;
    this.children.splice(this.children.indexOf(ref), 0, node);
  }
  remove() { if (this.parentNode) this.parentNode.children.splice(this.parentNode.children.indexOf(this), 1); this.parentNode = null; }
  get previousElementSibling() {
    if (!this.parentNode) return null;
    return this.parentNode.children[this.parentNode.children.indexOf(this) - 1] ?? null;
  }
  addEventListener(type, handler) { (this.listeners[type] ??= []).push(handler); }
  fire(type) { (this.listeners[type] ?? []).forEach((handler) => handler({ type })); }
  select() {}
}

const fakeDoc = { createElement: (tag) => new FakeElement(tag) };

function fakeSelect(labels, selected = 0) {
  const form = new FakeElement('form');
  const select = new FakeElement('select', { 'data-searchable': '', 'data-search-placeholder': 'Filter…', 'data-search-empty': 'No match' });
  select.options = labels.map((label) => ({ textContent: label }));
  select.selectedIndex = selected;
  form.append(select);
  return { form, select };
}

const comboboxesOf = (form) => form.children.filter((node) => node.hasAttribute('data-combobox'));

test('enhance is idempotent: a second pass keeps the same combobox', () => {
  const { form, select } = fakeSelect(['main', 'develop']);
  const first = enhance(select, fakeDoc);
  const second = enhance(select, fakeDoc);

  assert.equal(second, first);
  assert.equal(comboboxesOf(form).length, 1);
  assert.equal(first.input.value, 'main', 'the input shows the label of the selected option');
});

test('a rebuilt combobox leaves one change listener on the select and its input still follows the select', () => {
  const { form, select } = fakeSelect(['main', 'develop']);
  const before = enhance(select, fakeDoc);
  // A morph or swap drops the combobox but keeps the select.
  before.wrapper.remove();
  const after = enhance(select, fakeDoc);

  assert.notEqual(after, before, 'the combobox was rebuilt');
  assert.equal(comboboxesOf(form).length, 1);
  assert.equal(select.listeners.change.length, 1, 'rebuilds never add a second change listener');

  select.selectedIndex = 1;
  select.fire('change');
  assert.equal(after.input.value, 'develop', 'the rebuilt input follows the select');
  assert.equal(before.input.value, 'main', 'the discarded input is not touched');
});
