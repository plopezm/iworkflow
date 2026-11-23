// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';
import { carryPromptAnswer } from './group_prompt.js';

function element(field) {
  return { getAttribute: (name) => (name === 'data-prompt-field' ? field : null) };
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
