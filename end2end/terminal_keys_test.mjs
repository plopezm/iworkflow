// Run with: node --test end2end/terminal_keys_test.mjs
import test from 'node:test';
import assert from 'node:assert/strict';

globalThis.window = globalThis;
await import('../public/terminal.js');
const { sendKey } = window.iwfTerminal;

function fakeHandle({ applicationCursorKeysMode = false } = {}) {
  const term = {
    modes: { applicationCursorKeysMode },
    inputs: [],
    focused: 0,
    input(data, wasUserInput) { term.inputs.push([data, wasUserInput]); },
    focus() { term.focused += 1; },
  };
  return { term };
}

const NORMAL = { tab: '\t', ctrl_c: '\x03', up: '\x1b[A', down: '\x1b[B', right: '\x1b[C', left: '\x1b[D' };
const APPLICATION = { up: '\x1bOA', down: '\x1bOB', right: '\x1bOC', left: '\x1bOD' };

for (const [key, seq] of Object.entries(NORMAL)) {
  test(`${key} sends its escape sequence as user input and refocuses`, () => {
    const handle = fakeHandle();
    sendKey(handle, key);
    assert.deepEqual(handle.term.inputs, [[seq, true]]);
    assert.equal(handle.term.focused, 1);
  });
}

for (const [key, seq] of Object.entries(APPLICATION)) {
  test(`${key} honours application cursor mode`, () => {
    const handle = fakeHandle({ applicationCursorKeysMode: true });
    sendKey(handle, key);
    assert.deepEqual(handle.term.inputs, [[seq, true]]);
  });
}

test('tab and ctrl+c ignore application cursor mode', () => {
  const handle = fakeHandle({ applicationCursorKeysMode: true });
  sendKey(handle, 'tab');
  sendKey(handle, 'ctrl_c');
  assert.deepEqual(handle.term.inputs, [['\t', true], ['\x03', true]]);
});

test('unknown keys and missing handles are ignored', () => {
  const handle = fakeHandle();
  sendKey(handle, 'f13');
  sendKey(handle, 'toString');
  sendKey(null, 'tab');
  sendKey({}, 'tab');
  assert.deepEqual(handle.term.inputs, []);
  assert.equal(handle.term.focused, 0);
});
