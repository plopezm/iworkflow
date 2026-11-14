// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  collectEntries,
  filterEntries,
  isApplePlatform,
  isComposing,
  isPaletteShortcut,
  isTerminalTarget,
  paletteShortcutLabel,
  setupCommandPalette,
} from './command_palette.js';

const entries = [
  { href: '/settings', label: 'Settings' },
  { href: '/board/1', label: 'Backend' },
  { href: '/tasks/9', label: 'Fix the login bug' },
  { href: '/terminals', label: 'Terminals' },
];

test('an empty query keeps every entry in order', () => {
  assert.deepEqual(filterEntries(entries, '').map((e) => e.href), entries.map((e) => e.href));
  assert.deepEqual(filterEntries(entries, '   ').length, 4);
});

test('the query matches any part of the label, ignoring case and accents', () => {
  assert.deepEqual(filterEntries(entries, 'LOGIN').map((e) => e.href), ['/tasks/9']);
  assert.deepEqual(filterEntries([{ href: '/a', label: 'Navegación' }], 'navegacion').length, 1);
});

test('labels that start with the query come before the ones that only contain it', () => {
  const found = filterEntries(
    [{ href: '/tasks/1', label: 'Fix the login bug' }, { href: '/board/2', label: 'Login page' }],
    'login',
  );
  assert.deepEqual(found.map((e) => e.href), ['/board/2', '/tasks/1']);
});

test('a query that matches nothing returns no entries', () => {
  assert.deepEqual(filterEntries(entries, 'zzz'), []);
});

test('Ctrl+K and Cmd+K open the palette, other keys do not', () => {
  assert.equal(isPaletteShortcut({ ctrlKey: true, metaKey: false, key: 'k' }), true);
  assert.equal(isPaletteShortcut({ ctrlKey: false, metaKey: true, key: 'K' }), true);
  assert.equal(isPaletteShortcut({ ctrlKey: false, metaKey: false, key: 'k' }), false);
  assert.equal(isPaletteShortcut({ ctrlKey: true, metaKey: false, key: 'j' }), false);
  assert.equal(isPaletteShortcut({ ctrlKey: true, metaKey: false, altKey: true, key: 'k' }), false);
});

test('the terminal keeps Ctrl+K (kill to end of line)', () => {
  const inside = { closest: (sel) => (sel.includes('.xterm') ? {} : null) };
  const outside = { closest: () => null };
  assert.equal(isTerminalTarget(inside), true);
  assert.equal(isTerminalTarget(outside), false);
  assert.equal(isTerminalTarget(null), false);
});

test('an Enter that confirms an IME composition is not a choice', () => {
  assert.equal(isComposing({ key: 'Enter', isComposing: true, keyCode: 13 }), true);
  assert.equal(isComposing({ key: 'Enter', isComposing: false, keyCode: 229 }), true);
  assert.equal(isComposing({ key: 'Enter', isComposing: false, keyCode: 13 }), false);
});

test('Apple platforms are detected from userAgentData or the legacy platform', () => {
  assert.equal(isApplePlatform({ userAgentData: { platform: 'macOS' }, platform: '' }), true);
  assert.equal(isApplePlatform({ platform: 'MacIntel' }), true);
  assert.equal(isApplePlatform({ platform: 'iPhone' }), true);
  assert.equal(isApplePlatform({ userAgentData: { platform: 'Windows' }, platform: 'Win32' }), false);
  assert.equal(isApplePlatform({}), false);
});

test('the shortcut hint is ⌘K on Apple platforms and Ctrl K elsewhere', () => {
  assert.equal(paletteShortcutLabel(true), '⌘K');
  assert.equal(paletteShortcutLabel(false), 'Ctrl K');
});

// A tiny fake document: just enough DOM for the palette wiring, so the keyboard and hint behaviour is
// exercised without a browser. The selector strings are the ones command_palette.js queries.
class FakeElement {
  constructor(tag = 'div') {
    this.tagName = tag;
    this.attributes = {};
    this.dataset = {};
    this.listeners = {};
    this.queries = {};
    this.ancestors = {};
    this.children = [];
    this.textContent = '';
    this.className = '';
    this.hidden = false;
    this.open = false;
    this.value = '';
    this.clicks = 0;
  }

  getAttribute(name) {
    return Object.hasOwn(this.attributes, name) ? this.attributes[name] : null;
  }

  setAttribute(name, value) {
    this.attributes[name] = String(value);
  }

  addEventListener(type, fn) {
    (this.listeners[type] ??= []).push(fn);
  }

  dispatch(type, init = {}) {
    const event = {
      type,
      target: this,
      defaultPrevented: false,
      preventDefault() {
        this.defaultPrevented = true;
      },
      ...init,
    };
    for (const fn of this.listeners[type] ?? []) fn(event);
    return event;
  }

  querySelectorAll(selector) {
    const found = this.queries[selector];
    if (Array.isArray(found)) return found;
    return found ? [found] : [];
  }

  querySelector(selector) {
    return this.querySelectorAll(selector)[0] ?? null;
  }

  closest(selector) {
    return this.ancestors[selector] ?? null;
  }

  cloneNode() {
    const copy = new FakeElement(this.tagName);
    copy.textContent = this.textContent;
    return copy;
  }

  click() {
    this.clicks += 1;
  }

  replaceChildren(...nodes) {
    this.children = nodes;
  }

  append(...nodes) {
    this.children.push(...nodes);
  }

  scrollIntoView() {}

  focus() {}

  showModal() {
    this.open = true;
  }

  close() {
    this.open = false;
  }
}

function navLink(href, text) {
  const el = new FakeElement('a');
  el.setAttribute('href', href);
  el.textContent = text;
  return el;
}

function taskRow(id, title, href) {
  const row = new FakeElement('article');
  row.dataset.attentionTitle = title;
  row.setAttribute('data-attention-task', id);
  const link = navLink(href, 'open');
  link.ancestors['[data-attention-task]'] = row;
  row.queries['a[href]'] = link;
  return row;
}

function buildPage({ navLinks = [], rows = [], kbd = [] }) {
  const page = new FakeElement('document');
  const dialog = new FakeElement('dialog');
  const input = new FakeElement('input');
  const list = new FakeElement('ul');
  const empty = new FakeElement('p');
  empty.hidden = true;
  dialog.queries = {
    '[data-palette-input]': input,
    '[data-palette-list]': list,
    '[data-palette-empty]': empty,
  };
  page.queries = {
    '[data-app-shell] nav a[href]': navLinks,
    '[data-workspace-row]': [],
    '[data-attention-task]': rows,
    '[data-palette-kbd]': kbd,
    '[data-palette-open]': [],
  };
  page.getElementById = (id) => (id === 'command-palette' ? dialog : null);
  page.createElement = (tag) => new FakeElement(tag);
  return { page, dialog, input, list };
}

test('the entries come from the sidebar links and the attention rows, first link per path wins', () => {
  const settings = navLink('/settings', 'Settings');
  const duplicate = navLink('/settings', 'Ajustes');
  const row = taskRow('t1', 'Fix login', '/tasks/9');
  const { page } = buildPage({ navLinks: [settings, duplicate], rows: [row] });

  const found = collectEntries(page);

  assert.deepEqual(found.map((e) => [e.href, e.label]), [
    ['/settings', 'Settings'],
    ['/tasks/9', 'Fix login'],
  ]);
  assert.equal(found[0].link, settings);
});

test('an Enter during an IME composition does not choose the entry; a plain Enter does', () => {
  const settings = navLink('/settings', 'Settings');
  const { page, dialog, input } = buildPage({ navLinks: [settings] });
  setupCommandPalette(page, { platform: 'Linux x86_64' });

  page.dispatch('keydown', { key: 'k', ctrlKey: true, metaKey: false, altKey: false, shiftKey: false });
  assert.equal(dialog.open, true);
  assert.equal(input.value, '');

  dialog.dispatch('keydown', { key: 'Enter', isComposing: true, keyCode: 229 });
  dialog.dispatch('keydown', { key: 'Enter', isComposing: false, keyCode: 229 });
  assert.equal(settings.clicks, 0, 'composition Enter must not click the link');
  assert.equal(dialog.open, true);

  dialog.dispatch('keydown', { key: 'Enter', isComposing: false, keyCode: 13 });
  assert.equal(settings.clicks, 1);
  assert.equal(dialog.open, false);
});

test('the sidebar shortcut hint shows ⌘K on Apple platforms and keeps Ctrl K elsewhere', () => {
  const apple = new FakeElement('kbd');
  apple.textContent = 'Ctrl K';
  setupCommandPalette(buildPage({ kbd: [apple] }).page, { userAgentData: { platform: 'macOS' }, platform: '' });
  assert.equal(apple.textContent, '⌘K');

  const other = new FakeElement('kbd');
  other.textContent = 'Ctrl K';
  setupCommandPalette(buildPage({ kbd: [other] }).page, { platform: 'Linux x86_64' });
  assert.equal(other.textContent, 'Ctrl K');
});
