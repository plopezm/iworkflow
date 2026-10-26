// Run with: node --test web/static/js/
// Its own file: the document is set before the composer script is imported, which node runs once per process.
import test from 'node:test';
import assert from 'node:assert/strict';

test('importing the composer script installs nothing: the page that uses it does', async () => {
  let listeners = 0;
  globalThis.document = {
    addEventListener() { listeners += 1; },
    querySelectorAll: () => [],
  };

  const { installComposer } = await import('./composer.js');

  assert.equal(listeners, 0, 'the listeners are installed by the page, not by the import');
  delete globalThis.document;
  assert.equal(typeof installComposer, 'function');
});

test('installing the composer twice on one document keeps one listener per event', async () => {
  const counts = {};
  const doc = {
    addEventListener(type) { counts[type] = (counts[type] ?? 0) + 1; },
    querySelectorAll: () => [],
  };
  const { installComposer } = await import('./composer.js');

  installComposer(doc);
  installComposer(doc);

  assert.deepEqual(Object.values(counts).filter((count) => count > 1), [], 'no event is handled twice');
  assert.ok(counts.keydown >= 1, 'the keys are handled');
});
