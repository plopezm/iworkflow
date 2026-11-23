// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';
import { onSearchEscape, setupSidebarSearch, toggleSidebarSearch } from './sidebar_search.js';

// A sidebar stand-in: the search wrapper (hidden by default), its input and the icon button that controls it.
function fakeSidebar({ query = '' } = {}) {
  const focused = [];
  const input = {
    id: 'board-sidebar-filter',
    value: query,
    focus() { focused.push('input'); },
    closest: (selector) => (selector === '[data-search-panel]' ? panel : null),
  };
  const toggle = {
    attrs: { 'aria-expanded': 'false', 'aria-controls': 'board-sidebar-search' },
    getAttribute(name) { return this.attrs[name] ?? null; },
    setAttribute(name, value) { this.attrs[name] = String(value); },
    focus() { focused.push('toggle'); },
  };
  const panel = {
    id: 'board-sidebar-search',
    hidden: true,
    querySelector: (selector) => (selector === 'input' ? input : null),
  };
  const root = {
    getElementById: (id) => (id === panel.id ? panel : null),
    querySelectorAll: (selector) => (selector === '[data-search-toggle]' ? [toggle] : []),
  };
  return { root, input, toggle, panel, focused };
}

// The click the document receives on the icon button, and on anything else.
function clickOn(target) {
  return { target: { closest: (selector) => (selector === '[data-search-toggle]' ? target : null) } };
}

function keyOn(target, key) {
  return { key, target, prevented: false, preventDefault() { this.prevented = true; } };
}

test('the icon opens a closed search, shows the wrapper and focuses the input', () => {
  const sidebar = fakeSidebar();

  assert.equal(toggleSidebarSearch(sidebar.toggle, sidebar.root), true);

  assert.equal(sidebar.panel.hidden, false);
  assert.equal(sidebar.toggle.getAttribute('aria-expanded'), 'true');
  assert.deepEqual(sidebar.focused, ['input']);
});

test('the icon closes an open search whose input is empty', () => {
  const sidebar = fakeSidebar();
  toggleSidebarSearch(sidebar.toggle, sidebar.root);
  sidebar.focused.length = 0;

  assert.equal(toggleSidebarSearch(sidebar.toggle, sidebar.root), true);

  assert.equal(sidebar.panel.hidden, true);
  assert.equal(sidebar.toggle.getAttribute('aria-expanded'), 'false');
  assert.deepEqual(sidebar.focused, [], 'closing does not move the focus');
});

test('the icon does not close a search while a query narrows the list', () => {
  const sidebar = fakeSidebar();
  toggleSidebarSearch(sidebar.toggle, sidebar.root);
  sidebar.input.value = 'api';

  assert.equal(toggleSidebarSearch(sidebar.toggle, sidebar.root), false);

  assert.equal(sidebar.panel.hidden, false, 'the wrapper stays open, the query stays visible');
  assert.equal(sidebar.toggle.getAttribute('aria-expanded'), 'true');
});

test('a toggle whose wrapper is missing does nothing and does not throw', () => {
  const sidebar = fakeSidebar();
  const orphan = { getAttribute: () => 'no-such-panel', setAttribute() {}, focus() {} };

  assert.equal(toggleSidebarSearch(orphan, sidebar.root), false);
  assert.equal(sidebar.panel.hidden, true);
});

test('Escape in an empty search input closes the search and returns the focus to the icon', () => {
  const sidebar = fakeSidebar();
  toggleSidebarSearch(sidebar.toggle, sidebar.root);
  sidebar.focused.length = 0;
  const event = keyOn(sidebar.input, 'Escape');

  assert.equal(onSearchEscape(event, sidebar.root), true);

  assert.equal(sidebar.panel.hidden, true);
  assert.equal(sidebar.toggle.getAttribute('aria-expanded'), 'false');
  assert.deepEqual(sidebar.focused, ['toggle']);
  assert.equal(event.prevented, true);
});

test('Escape leaves a search with a query open, and other keys or elements are ignored', () => {
  const sidebar = fakeSidebar();
  toggleSidebarSearch(sidebar.toggle, sidebar.root);
  sidebar.input.value = 'api';

  assert.equal(onSearchEscape(keyOn(sidebar.input, 'Escape'), sidebar.root), false);
  assert.equal(sidebar.panel.hidden, false);

  sidebar.input.value = '';
  assert.equal(onSearchEscape(keyOn(sidebar.input, 'a'), sidebar.root), false);
  assert.equal(onSearchEscape(keyOn({ id: 'other', value: '', closest: () => null }, 'Escape'), sidebar.root), false);
  assert.equal(sidebar.panel.hidden, false, 'only Escape in the search input closes it');
});

test('setupSidebarSearch wires the icon click and the Escape key of the whole document', () => {
  const sidebar = fakeSidebar();
  const listeners = {};
  const doc = {
    ...sidebar.root,
    addEventListener: (type, listener) => { listeners[type] = listener; },
  };
  setupSidebarSearch(doc);

  listeners.click(clickOn(sidebar.toggle));
  assert.equal(sidebar.panel.hidden, false, 'a click on the icon opens the search');

  listeners.keydown(keyOn(sidebar.input, 'Escape'));
  assert.equal(sidebar.panel.hidden, true, 'Escape in the empty input closes it');

  listeners.click({ target: { closest: () => null } });
  assert.equal(sidebar.panel.hidden, true, 'a click elsewhere changes nothing');
});
