// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';
import { openFileLink, openPathURL, onClick, OPEN_PATH_ERROR_CLASS } from './file_links.js';

function fakeLink(path, taskId) {
  const classes = new Set();
  const attrs = new Map([['data-file-path', path]]);
  return {
    classList: { add: (c) => classes.add(c), remove: (c) => classes.delete(c), has: (c) => classes.has(c) },
    getAttribute: (n) => attrs.get(n) ?? null,
    setAttribute: (n, v) => attrs.set(n, v),
    removeAttribute: (n) => attrs.delete(n),
    closest: (selector) => (selector === '[data-task-id]' && taskId ? { dataset: { taskId } } : null),
    attrs,
  };
}

test('the endpoint names the task', () => {
  assert.equal(openPathURL('a b'), '/tasks/a%20b/open-path');
});

test('a click posts the path of the link to the task', async () => {
  const link = fakeLink('src/main.go:12', 't1');
  let call;
  const ok = await openFileLink(link, async (url, init) => { call = { url, init }; return { ok: true }; });
  assert.equal(ok, true);
  assert.equal(call.url, '/tasks/t1/open-path');
  assert.equal(call.init.method, 'POST');
  assert.equal(new URLSearchParams(call.init.body).get('path'), 'src/main.go:12');
});

test('a refused path marks the link with the reason', async () => {
  const link = fakeLink('/tmp/x.sh', 't1');
  const ok = await openFileLink(link, async () => ({ ok: false, text: async () => ' blocked \n' }));
  assert.equal(ok, false);
  assert.equal(link.classList.has(OPEN_PATH_ERROR_CLASS), true);
  assert.equal(link.attrs.get('title'), 'blocked');
});

test('a link outside a task does nothing', async () => {
  let called = false;
  const ok = await openFileLink(fakeLink('/a/b.go', null), async () => { called = true; return { ok: true }; });
  assert.equal(ok, false);
  assert.equal(called, false);
});

test('onClick keeps the browser on the page for a file link only', () => {
  let prevented = false;
  const link = fakeLink('/a/b.go', null);
  onClick({ target: { closest: () => link }, preventDefault: () => { prevented = true; } });
  assert.equal(prevented, true);
  prevented = false;
  onClick({ target: { closest: () => null }, preventDefault: () => { prevented = true; } });
  assert.equal(prevented, false);
});
