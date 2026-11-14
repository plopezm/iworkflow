// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';
import { filterEntries, isPaletteShortcut, isTerminalTarget } from './command_palette.js';

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
