// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  buildRegex,
  findMatches,
  lineOf,
  stepIndex,
  counterText,
  saveFilter,
  loadFilter,
  saveJump,
  takeJump,
  pickCurrent,
  scrollTopFor,
  textRangeAt,
} from './file_search.js';

const flags = (over = {}) => ({ re: false, cs: false, ww: false, ...over });

function fakeStorage() {
  const data = new Map();
  return {
    getItem: (k) => (data.has(k) ? data.get(k) : null),
    setItem: (k, v) => data.set(k, String(v)),
    removeItem: (k) => data.delete(k),
  };
}

test('buildRegex: literal escapes metacharacters and ignores case by default', () => {
  const re = buildRegex('a.b(', flags());
  assert.ok(re.test('xA.B(y'));
  assert.ok(!buildRegex('a.b', flags()).test('axb'));
});

test('buildRegex: regex mode, case and whole word', () => {
  assert.ok(buildRegex('a.b', flags({ re: true })).test('axb'));
  assert.ok(!buildRegex('Foo', flags({ cs: true })).test('foo'));
  assert.ok(!buildRegex('foo', flags({ ww: true })).test('foobar'));
  assert.ok(buildRegex('foo', flags({ ww: true })).test('a foo b'));
});

test('buildRegex: empty text or a broken regex gives null', () => {
  assert.equal(buildRegex('', flags()), null);
  assert.equal(buildRegex('(', flags({ re: true })), null);
});

test('findMatches: offsets over the whole text, no empty matches, matches per line anchors', () => {
  const text = 'foo bar\nFoo\nxfoo';
  const found = findMatches(text, buildRegex('foo', flags()));
  assert.deepEqual(found, [
    { start: 0, end: 3 },
    { start: 8, end: 11 },
    { start: 13, end: 16 },
  ]);
  assert.deepEqual(findMatches('abc', buildRegex('x*', flags({ re: true }))), []);
  assert.deepEqual(findMatches('a\nb', buildRegex('^b$', flags({ re: true }))), [{ start: 2, end: 3 }]);
});

test('findMatches: caps the number of matches', () => {
  const found = findMatches('x'.repeat(50), buildRegex('x', flags()), 10);
  assert.equal(found.length, 10);
});

test('findMatches: null regex finds nothing', () => {
  assert.deepEqual(findMatches('abc', null), []);
});

test('lineOf: 1-based line of an offset', () => {
  const text = 'a\nbb\nccc';
  assert.equal(lineOf(text, 0), 1);
  assert.equal(lineOf(text, 2), 2);
  assert.equal(lineOf(text, 5), 3);
});

test('stepIndex wraps around in both directions', () => {
  assert.equal(stepIndex(0, 3, 1), 1);
  assert.equal(stepIndex(2, 3, 1), 0);
  assert.equal(stepIndex(0, 3, -1), 2);
  assert.equal(stepIndex(0, 0, 1), -1);
});

test('counterText shows "n / total" or the empty marker', () => {
  assert.equal(counterText(2, 17), '3 / 17');
  assert.equal(counterText(-1, 0), '0 / 0');
});

test('filter handoff: save, load and clear with an empty query', () => {
  const store = fakeStorage();
  saveFilter(store, 'scope1', { q: 'foo', re: true, cs: false, ww: true });
  assert.deepEqual(loadFilter(store, 'scope1'), { q: 'foo', re: true, cs: false, ww: true });
  assert.equal(loadFilter(store, 'other'), null);
  saveFilter(store, 'scope1', { q: '', re: false, cs: false, ww: false });
  assert.equal(loadFilter(store, 'scope1'), null);
});

test('loadFilter ignores corrupt storage', () => {
  const store = fakeStorage();
  store.setItem('iwf-content-search:s', '{nope');
  assert.equal(loadFilter(store, 's'), null);
});

test('jump handoff is consumed once and only for its own path', () => {
  const store = fakeStorage();
  saveJump(store, 's', { path: 'src/a.go', line: 42 });
  assert.equal(takeJump(store, 's', 'other.go'), null);
  assert.deepEqual(takeJump(store, 's', 'src/a.go'), { path: 'src/a.go', line: 42 });
  assert.equal(takeJump(store, 's', 'src/a.go'), null);
});

test('pickCurrent: the first match on the jump line, else the first match', () => {
  const text = 'foo\nbar\nfoo bar foo';
  const found = findMatches(text, buildRegex('foo', flags()));
  assert.equal(pickCurrent(text, found, 3), 1);
  assert.equal(pickCurrent(text, found, 99), 0);
  assert.equal(pickCurrent(text, found, 0), 0);
  assert.equal(pickCurrent(text, [], 3), -1);
});

test('scrollTopFor centres the line, never below zero', () => {
  assert.equal(scrollTopFor({ line: 1, lineHeight: 20, padTop: 12, viewport: 400 }), 0);
  assert.equal(scrollTopFor({ line: 100, lineHeight: 20, padTop: 12, viewport: 400 }), 99 * 20 + 12 - 200 + 10);
});

test('textRangeAt maps text offsets onto text nodes', () => {
  const nodes = [{ length: 3, id: 'a' }, { length: 4, id: 'b' }];
  assert.deepEqual(textRangeAt(nodes, 1, 5), { startNode: 0, startOffset: 1, endNode: 1, endOffset: 2 });
  assert.deepEqual(textRangeAt(nodes, 3, 4), { startNode: 1, startOffset: 0, endNode: 1, endOffset: 1 });
  assert.equal(textRangeAt(nodes, 8, 9), null);
});
