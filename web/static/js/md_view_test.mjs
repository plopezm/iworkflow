import test from 'node:test';
import assert from 'node:assert/strict';
import { initMarkdownViews, normalizeMode, refreshPreview, setMode } from './md_view.js';

function fakeForm() {
  const classes = new Set();
  const preview = {
    innerHTML: '',
    textContent: '',
    dataset: { mdError: 'no se pudo' },
    classList: { add: (c) => classes.add(c), remove: (c) => classes.delete(c), has: (c) => classes.has(c) },
  };
  const field = { value: '# hola' };
  const buttons = ['code', 'rendered'].map((mode) => ({
    dataset: { mdModeSet: mode },
    pressed: null,
    setAttribute(name, value) {
      if (name === 'aria-pressed') this.pressed = value;
    },
  }));
  const form = {
    dataset: { mdForm: '/markdown/preview' },
    querySelector: (selector) => (selector === '[data-md-preview]' ? preview : selector === 'textarea[name="content"]' ? field : null),
    querySelectorAll: (selector) => (selector === '[data-md-mode-set]' ? buttons : []),
  };
  return { form, preview, field, buttons, classes };
}

function stubFetch(handler) {
  const calls = [];
  globalThis.fetch = (url, options) => {
    calls.push({ url, options });
    return handler(calls.length);
  };
  return calls;
}

const reply = (html) => Promise.resolve({ ok: true, status: 200, text: () => Promise.resolve(html) });

test('only "rendered" selects the rendered view', () => {
  assert.equal(normalizeMode('rendered'), 'rendered');
  assert.equal(normalizeMode('code'), 'code');
  assert.equal(normalizeMode(null), 'code');
  assert.equal(normalizeMode('x'), 'code');
});

test('the preview posts the current text and shows the answer', async () => {
  const { form, preview } = fakeForm();
  const calls = stubFetch(() => reply('<h1>hola</h1>'));
  await refreshPreview(form);
  assert.equal(calls[0].url, '/markdown/preview');
  assert.equal(calls[0].options.method, 'POST');
  assert.equal(calls[0].options.body.get('content'), '# hola');
  assert.equal(preview.innerHTML, '<h1>hola</h1>');
});

test('an older answer never replaces a newer one', async () => {
  const { form, preview } = fakeForm();
  let releaseFirst;
  stubFetch((n) => (n === 1 ? new Promise((resolve) => { releaseFirst = () => resolve(reply('<p>viejo</p>')); }) : reply('<p>nuevo</p>')));
  const first = refreshPreview(form);
  await refreshPreview(form);
  releaseFirst();
  await first;
  assert.equal(preview.innerHTML, '<p>nuevo</p>');
});

test('a failed preview says so and a later success clears the message', async () => {
  const { form, preview, classes } = fakeForm();
  const quiet = console.error;
  console.error = () => {};
  try {
    stubFetch(() => Promise.resolve({ ok: false, status: 500, text: () => Promise.resolve('') }));
    await refreshPreview(form);
  } finally {
    console.error = quiet;
  }
  assert.equal(preview.textContent, 'no se pudo');
  assert.ok(classes.has('md-preview-error'));

  stubFetch(() => reply('<p>ok</p>'));
  await refreshPreview(form);
  assert.equal(preview.innerHTML, '<p>ok</p>');
  assert.ok(!classes.has('md-preview-error'));
});

test('setMode marks the pressed button and loads the preview only for rendered', async () => {
  const { form, buttons } = fakeForm();
  const calls = stubFetch(() => reply('<p>x</p>'));
  setMode(form, 'code');
  assert.equal(form.dataset.mdMode, 'code');
  assert.deepEqual(buttons.map((b) => b.pressed), ['true', 'false']);
  assert.equal(calls.length, 0);
  setMode(form, 'rendered');
  assert.equal(form.dataset.mdMode, 'rendered');
  assert.deepEqual(buttons.map((b) => b.pressed), ['false', 'true']);
  assert.equal(calls.length, 1);
});

test('a form that arrives after a swap takes the remembered mode once', () => {
  globalThis.localStorage = { getItem: () => 'rendered', setItem: () => {} };
  const fresh = fakeForm();
  const settled = fakeForm();
  settled.form.dataset.mdMode = 'code';
  const calls = stubFetch(() => reply(''));
  initMarkdownViews({ querySelectorAll: () => [fresh.form, settled.form] });
  assert.equal(fresh.form.dataset.mdMode, 'rendered');
  assert.equal(settled.form.dataset.mdMode, 'code', 'a form that has a mode is left alone');
  assert.equal(calls.length, 1);
  delete globalThis.localStorage;
});
