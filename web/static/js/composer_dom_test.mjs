// Run with: node --test web/static/js/
// The message box driven through a small stand-in for the DOM: the keys, the file requests and the send button.
import test from 'node:test';
import assert from 'node:assert/strict';
import { installComposer } from './composer.js';

// A stand-in element: what the composer script reads and writes, nothing more.
function element(attrs = [], props = {}) {
  const node = {
    value: '',
    hidden: false,
    disabled: false,
    innerHTML: '',
    selectionStart: 0,
    files: [],
    dataset: {},
    parent: null,
    children: {},
    ...props,
    matches: (selector) => attrs.includes(selector),
    closest(selector) {
      for (let current = node; current; current = current.parent) {
        if (current.matches(selector)) return current;
      }
      return null;
    },
    querySelector: (selector) => node.children[selector] ?? null,
    querySelectorAll: () => [],
    setAttribute() {},
    append() {},
    focus() {},
    setSelectionRange() {},
    classList: { toggle() {} },
    replaceChildren() {},
  };
  return node;
}

// The composer script builds the reference chips with the document it runs in.
globalThis.document = { createElement: () => element() };

// A message box with its text, its send button, its file menu and a form that counts its submissions.
function messageBox() {
  const submissions = { count: 0 };
  const form = element([], { requestSubmit: () => { submissions.count += 1; } });
  const textarea = element(['[data-composer-input]']);
  const send = element(['[data-composer-send]'], { disabled: true });
  const menu = element(['[data-composer-menu]'], { hidden: true });
  const refs = element(['[data-composer-refs]'], { hidden: true });
  const images = element(['[data-image-input]']);
  const composer = element(['[data-composer]'], {
    dataset: { filesPath: '/tasks/t1/composer/files', commands: '[]' },
    children: {
      '[data-composer-input]': textarea,
      '[data-composer-send]': send,
      '[data-composer-menu]': menu,
      '[data-composer-refs]': refs,
      '[data-image-input]': images,
    },
  });
  textarea.parent = composer;
  textarea.form = form;
  form.parent = composer;
  for (const child of [send, menu, refs, images]) child.parent = composer;
  return { composer, textarea, send, menu, submissions };
}

// A document that records its listeners and dispatches events to them.
function documentWith(composers) {
  const listeners = {};
  return {
    addEventListener(type, listener) {
      (listeners[type] ??= []).push(listener);
    },
    querySelectorAll: (selector) => (selector === '[data-composer]' ? composers : []),
    dispatch(type, event) {
      for (const listener of listeners[type] ?? []) listener(event);
    },
    listenerCount: (type) => (listeners[type] ?? []).length,
  };
}

// Types the text into the box: the input event that the browser fires, then the key that ends the keystroke.
function type(doc, box, text) {
  box.textarea.value = text;
  box.textarea.selectionStart = text.length;
  doc.dispatch('input', { target: box.textarea });
  doc.dispatch('keyup', { key: 'a', target: box.textarea });
}

// The file requests of the test: each one waits until the test answers it, with a body and a status or with a failure.
function fileRequests() {
  const requests = [];
  globalThis.fetch = (url) => new Promise((resolve, reject) => {
    requests.push({
      url,
      answer: (html, status = 200) => resolve({ ok: status >= 200 && status < 300, status, text: async () => html }),
      fail: (error) => reject(error),
    });
  });
  return requests;
}

const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

test('Ctrl+Enter sends a message that has text', () => {
  const box = messageBox();
  const doc = documentWith([box.composer]);
  installComposer(doc);
  box.textarea.value = 'hola';
  box.send.disabled = false;

  doc.dispatch('keydown', { key: 'Enter', ctrlKey: true, target: box.textarea, preventDefault() {} });

  assert.equal(box.submissions.count, 1);
});

test('Ctrl+Enter does nothing while the send button is disabled', () => {
  const box = messageBox();
  const doc = documentWith([box.composer]);
  installComposer(doc);
  assert.equal(box.send.disabled, true, 'an empty box has nothing to send');

  doc.dispatch('keydown', { key: 'Enter', ctrlKey: true, target: box.textarea, preventDefault() {} });

  assert.equal(box.submissions.count, 0);
});

test('Ctrl+Enter does nothing while a send is in flight', () => {
  const box = messageBox();
  const doc = documentWith([box.composer]);
  installComposer(doc);
  box.textarea.value = 'hola';
  box.send.disabled = true; // htmx disables the button while the request is in flight

  doc.dispatch('keydown', { key: 'Enter', ctrlKey: true, target: box.textarea, preventDefault() {} });

  assert.equal(box.submissions.count, 0, 'a second send cannot start while the first one is on its way');
});

test('an @ query asks the server once, even though the input and the key both report it', async () => {
  const box = messageBox();
  const doc = documentWith([box.composer]);
  installComposer(doc);
  const requests = fileRequests();

  type(doc, box, '@a');

  assert.equal(requests.length, 1, 'one keystroke is one request');
  requests[0].answer('<li>a.txt</li>');
  await flush();
  assert.equal(box.menu.innerHTML, '<li>a.txt</li>');
});

test('a late answer for a query the user has already left is not shown', async () => {
  const box = messageBox();
  const doc = documentWith([box.composer]);
  installComposer(doc);
  const requests = fileRequests();

  type(doc, box, '@a');
  type(doc, box, '@ab');
  requests[1].answer('<li>ab.txt</li>');
  await flush();
  requests[0].answer('<li>a.txt</li>');
  await flush();

  assert.equal(box.menu.innerHTML, '<li>ab.txt</li>', 'the menu keeps the answer of the query on screen');
});

test('an answer that arrives after the @ trigger was left does not reopen the menu', async () => {
  const box = messageBox();
  const doc = documentWith([box.composer]);
  installComposer(doc);
  const requests = fileRequests();

  type(doc, box, '@a');
  type(doc, box, '');
  requests[0].answer('<li>a.txt</li>');
  await flush();

  assert.equal(box.menu.hidden, true);
  assert.equal(box.menu.innerHTML, '');
});

test('a failed file answer shows no suggestions and the next query still works', async () => {
  const box = messageBox();
  const doc = documentWith([box.composer]);
  installComposer(doc);
  const requests = fileRequests();

  type(doc, box, '@a');
  requests[0].answer('<p>internal server error</p>', 500);
  await flush();

  assert.equal(box.menu.innerHTML, '', 'the error body is not rendered as file names');
  assert.equal(box.menu.hidden, true);

  type(doc, box, '@ab');
  requests[1].answer('<li>ab.txt</li>');
  await flush();

  assert.equal(box.menu.innerHTML, '<li>ab.txt</li>');
  assert.equal(box.menu.hidden, false);
});

test('a file request that fails to reach the server closes the menu', async () => {
  const box = messageBox();
  const doc = documentWith([box.composer]);
  installComposer(doc);
  const requests = fileRequests();

  type(doc, box, '@a');
  requests[0].fail(new TypeError('Failed to fetch'));
  await flush();

  assert.equal(box.menu.hidden, true);
  assert.equal(box.menu.innerHTML, '');
});
