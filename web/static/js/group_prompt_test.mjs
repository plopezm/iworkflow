// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';
import { carryPromptAnswer, setupGroupPrompt } from './group_prompt.js';

function element(field) {
  return { getAttribute: (name) => (name === 'data-prompt-field' ? field : null) };
}

// A document stand-in that records the htmx:configRequest listener setupGroupPrompt installs on the body.
function fakeRoot() {
  const listeners = {};
  return {
    body: { addEventListener: (type, listener) => { listeners[type] = listener; } },
    dispatch: (type, event) => listeners[type](event),
  };
}

// The event htmx fires before it sends a request; preventDefault cancels the request.
function configRequest(detail) {
  const event = { detail, target: null, defaultPrevented: false };
  event.preventDefault = () => { event.defaultPrevented = true; };
  return event;
}

test('the prompt answer becomes the form field the element names', () => {
  const detail = { headers: { 'HX-Prompt': 'Backend' }, parameters: { color: 'red' } };

  assert.equal(carryPromptAnswer(detail, element('name')), true);
  assert.deepEqual(detail.parameters, { color: 'red', name: 'Backend' });
});

test('a request without a prompt answer is left as it is', () => {
  const detail = { headers: {}, parameters: { name: 'kept' } };

  assert.equal(carryPromptAnswer(detail, element('name')), false);
  assert.deepEqual(detail.parameters, { name: 'kept' });
});

test('an element without a field is left as it is', () => {
  const detail = { headers: { 'HX-Prompt': 'Backend' }, parameters: {} };

  assert.equal(carryPromptAnswer(detail, element(null)), false);
  assert.equal(carryPromptAnswer(detail, null), false);
  assert.deepEqual(detail.parameters, {});
});

test('a blank prompt answer cancels the request instead of posting an empty name', () => {
  for (const answer of ['', '   ', '\t \n']) {
    const root = fakeRoot();
    setupGroupPrompt(root);
    const detail = { headers: { 'HX-Prompt': answer }, parameters: {}, elt: element('name') };
    const event = configRequest(detail);

    root.dispatch('htmx:configRequest', event);

    assert.equal(event.defaultPrevented, true, `answer ${JSON.stringify(answer)} must cancel the request`);
    assert.deepEqual(detail.parameters, {}, 'no name is posted');
  }
});

test('a non-blank prompt answer is sent with the name and not cancelled', () => {
  const root = fakeRoot();
  setupGroupPrompt(root);
  const detail = { headers: { 'HX-Prompt': '  Backend ' }, parameters: {}, elt: element('name') };
  const event = configRequest(detail);

  root.dispatch('htmx:configRequest', event);

  assert.equal(event.defaultPrevented, false);
  assert.deepEqual(detail.parameters, { name: '  Backend ' });
});

test('a blank answer of an element without a field is not cancelled: only the group prompts are', () => {
  const root = fakeRoot();
  setupGroupPrompt(root);
  const event = configRequest({ headers: { 'HX-Prompt': '' }, parameters: {}, elt: element(null) });

  root.dispatch('htmx:configRequest', event);

  assert.equal(event.defaultPrevented, false);
});

test('a request without a prompt answer is not cancelled', () => {
  const root = fakeRoot();
  setupGroupPrompt(root);
  const event = configRequest({ headers: {}, parameters: {}, elt: element('name') });

  root.dispatch('htmx:configRequest', event);

  assert.equal(event.defaultPrevented, false);
});
