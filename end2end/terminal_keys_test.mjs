// Run with: node --test end2end/terminal_keys_test.mjs
import test from 'node:test';
import assert from 'node:assert/strict';

globalThis.window = globalThis;
await import('../public/terminal.js');
const { sendKey } = window.iwfTerminal;

function setNavigator(value) {
  Object.defineProperty(globalThis, 'navigator', { value, configurable: true, writable: true });
}

function fakeHandle({ applicationCursorKeysMode = false } = {}) {
  const term = {
    modes: { applicationCursorKeysMode },
    inputs: [],
    focused: 0,
    input(data, wasUserInput) { term.inputs.push([data, wasUserInput]); },
    focus() { term.focused += 1; },
    pasted: [],
    paste(text) { term.pasted.push(text); },
  };
  return { term };
}

const NORMAL = { tab: '\t', esc: '\x1b', ctrl_c: '\x03', up: '\x1b[A', down: '\x1b[B', right: '\x1b[C', left: '\x1b[D' };
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

test('paste writes the clipboard text through term.paste and refocuses', async () => {
  const handle = fakeHandle();
  setNavigator({ clipboard: { readText: async () => 'ls -la\n' } });
  sendKey(handle, 'paste');
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(handle.term.pasted, ['ls -la\n']);
  assert.deepEqual(handle.term.inputs, []);
  assert.equal(handle.term.focused, 1);
});

test('paste is a no-op when the clipboard is unavailable or denied', async () => {
  const handle = fakeHandle();
  setNavigator({});
  sendKey(handle, 'paste');
  setNavigator({ clipboard: { readText: async () => { throw new Error('denied'); } } });
  sendKey(handle, 'paste');
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(handle.term.pasted, []);
});

function touchRig({ type = 'normal', rows = 10, lineHeight = 20 } = {}) {
  const listeners = {};
  const wheels = [];
  const screen = {
    clientHeight: rows * lineHeight,
    getBoundingClientRect: () => ({ left: 0, top: 0, width: 100, height: 100 }),
    dispatchEvent: (event) => wheels.push(event),
  };
  const el = {
    style: {},
    addEventListener: (name, fn) => { listeners[name] = fn; },
    removeEventListener: (name) => { delete listeners[name]; },
    querySelector: () => screen,
  };
  const scrolled = [];
  const term = { rows, buffer: { active: { type } }, scrollLines: (n) => scrolled.push(n) };
  globalThis.WheelEvent = class { constructor(name, init) { this.name = name; Object.assign(this, init); } };
  const frames = [];
  globalThis.requestAnimationFrame = (fn) => frames.push(fn) && frames.length;
  globalThis.cancelAnimationFrame = () => {};
  const stop = window.iwfTerminal.enableTouchScroll(term, el);
  const touch = (y, timeStamp, type) => ({ touches: [{ clientY: y }], timeStamp, type, preventDefault() { this.prevented = true; } });
  return { listeners, scrolled, wheels, frames, stop, el, touch };
}

test('dragging up scrolls the normal buffer down by whole lines and blocks page scroll', () => {
  const rig = touchRig();
  rig.listeners.touchstart(rig.touch(200, 0));
  const move = rig.touch(140, 16);
  rig.listeners.touchmove(move);
  assert.deepEqual(rig.scrolled, [3]);
  assert.equal(move.prevented, true);
  assert.equal(rig.el.style.touchAction, 'none');
});

test('a tap shorter than the threshold neither scrolls nor blocks the click', () => {
  const rig = touchRig();
  rig.listeners.touchstart(rig.touch(200, 0));
  const move = rig.touch(197, 16);
  rig.listeners.touchmove(move);
  assert.deepEqual(rig.scrolled, []);
  assert.notEqual(move.prevented, true);
});

test('the alternate buffer receives synthetic line wheel events instead of scrollLines', () => {
  const rig = touchRig({ type: 'alternate' });
  rig.listeners.touchstart(rig.touch(200, 0));
  rig.listeners.touchmove(rig.touch(160, 16));
  assert.deepEqual(rig.scrolled, []);
  assert.equal(rig.wheels.length, 1);
  assert.equal(rig.wheels[0].deltaY, 2);
  assert.equal(rig.wheels[0].deltaMode, 1);
});

test('inertia starts after a quick flick but not after holding still or cancelling', () => {
  const flick = touchRig();
  flick.listeners.touchstart(flick.touch(300, 0));
  flick.listeners.touchmove(flick.touch(200, 16));
  flick.listeners.touchend(flick.touch(200, 20, 'touchend'));
  assert.equal(flick.frames.length, 1);

  const held = touchRig();
  held.listeners.touchstart(held.touch(300, 0));
  held.listeners.touchmove(held.touch(200, 16));
  held.listeners.touchend(held.touch(200, 400, 'touchend'));
  assert.equal(held.frames.length, 0);

  const cancelled = touchRig();
  cancelled.listeners.touchstart(cancelled.touch(300, 0));
  cancelled.listeners.touchmove(cancelled.touch(200, 16));
  cancelled.listeners.touchcancel(cancelled.touch(200, 20, 'touchcancel'));
  assert.equal(cancelled.frames.length, 0);
});

test('the returned disposer removes every touch listener', () => {
  const rig = touchRig();
  rig.stop();
  assert.deepEqual(Object.keys(rig.listeners), []);
});
