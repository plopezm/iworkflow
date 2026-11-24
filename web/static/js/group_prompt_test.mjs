// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';
import { setupGroupPrompt } from './group_prompt.js';

function element(question, field) {
  const attrs = { 'data-prompt-question': question, 'data-prompt-field': field };
  return { getAttribute: (name) => (attrs[name] === undefined ? null : attrs[name]) };
}

// A document stand-in that records the htmx listeners setupGroupPrompt installs on the body.
function fakeRoot() {
  const listeners = {};
  return {
    body: { addEventListener: (type, listener) => { listeners[type] = listener; } },
    dispatch: (type, event) => listeners[type](event),
  };
}

// The htmx:confirm event: preventDefault holds the request until detail.issueRequest is called.
function confirmEvent(elt) {
  const event = { detail: { elt, issued: 0, issueRequest() { this.issued += 1; } }, defaultPrevented: false };
  event.preventDefault = () => { event.defaultPrevented = true; };
  return event;
}

function configEvent(elt, parameters = {}) {
  return { detail: { elt, parameters, headers: {} } };
}

const tick = () => new Promise((resolve) => setTimeout(resolve, 0));

test('an element with a question holds its request and asks the user, never window.prompt', async () => {
  const root = fakeRoot();
  const asked = [];
  setupGroupPrompt(root, async (elt) => { asked.push(elt); return 'Backend'; });
  const elt = element('Group name', 'name');
  const event = confirmEvent(elt);

  root.dispatch('htmx:confirm', event);
  await tick();

  assert.equal(event.defaultPrevented, true);
  assert.deepEqual(asked, [elt]);
  assert.equal(event.detail.issued, 1, 'the request resumes once the user answers');
});

test('the answer becomes the form field the element names, only for that request', async () => {
  const root = fakeRoot();
  setupGroupPrompt(root, async () => 'Backend');
  const elt = element('Group name', 'name');
  root.dispatch('htmx:confirm', confirmEvent(elt));
  await tick();

  const first = configEvent(elt, { color: 'red' });
  root.dispatch('htmx:configRequest', first);
  assert.deepEqual(first.detail.parameters, { color: 'red', name: 'Backend' });

  const second = configEvent(elt, { color: 'red' });
  root.dispatch('htmx:configRequest', second);
  assert.deepEqual(second.detail.parameters, { color: 'red' }, 'the answer is consumed by its request');
});

test('a cancelled dialog sends nothing', async () => {
  const root = fakeRoot();
  setupGroupPrompt(root, async () => null);
  const event = confirmEvent(element('Group name', 'name'));

  root.dispatch('htmx:confirm', event);
  await tick();

  assert.equal(event.detail.issued, 0);
});

test('a blank answer sends nothing instead of creating a group with an empty name', async () => {
  for (const answer of ['', '   ', '\t \n']) {
    const root = fakeRoot();
    setupGroupPrompt(root, async () => answer);
    const event = confirmEvent(element('Group name', 'name'));

    root.dispatch('htmx:confirm', event);
    await tick();

    assert.equal(event.detail.issued, 0, `answer ${JSON.stringify(answer)} must not send a request`);
  }
});

test('an element without a question is left to htmx', async () => {
  const root = fakeRoot();
  let asked = false;
  setupGroupPrompt(root, async () => { asked = true; return 'x'; });
  const event = confirmEvent(element(null, null));

  root.dispatch('htmx:confirm', event);
  await tick();

  assert.equal(event.defaultPrevented, false);
  assert.equal(asked, false);
  const config = configEvent(element(null, null), { name: 'kept' });
  root.dispatch('htmx:configRequest', config);
  assert.deepEqual(config.detail.parameters, { name: 'kept' });
});
