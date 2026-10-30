// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';

function fakeEl({ scrollHeight = 1000, clientHeight = 400, scrollTop = 600 } = {}) {
  const listeners = {};
  const el = {
    scrollHeight, clientHeight, scrollTop,
    parentElement: { dataset: {} },
    addEventListener: (type, fn) => { listeners[type] = fn; },
    scrollTo: ({ top }) => { el.scrolledTo = top; },
    emit: (type) => listeners[type]?.(),
  };
  return el;
}

// Node has no DOM: the observer only needs to exist, the tests drive the scroll events themselves.
globalThis.MutationObserver = class { observe() {} };

let timers = [];
globalThis.setTimeout = (fn) => { timers.push(fn); return timers.length; };
globalThis.clearTimeout = (id) => { if (id) timers[id - 1] = null; };
const fireTimers = () => { const t = timers; timers = []; t.forEach((fn) => fn?.()); };

function installDom(el) {
  timers = [];
  globalThis.requestAnimationFrame = (fn) => fn();
  globalThis.document = { getElementById: (id) => (id === 'log' ? el : null) };
}

const mod = await import('./follow_bottom.js');

test('near-bottom uses an 80px threshold', () => {
  assert.equal(mod.isNearBottom(fakeEl({ scrollTop: 600 })), true);
  assert.equal(mod.isNearBottom(fakeEl({ scrollTop: 521 })), true);
  assert.equal(mod.isNearBottom(fakeEl({ scrollTop: 520 })), false);
});

test('followBottom jumps to the end while stuck and flags at-bottom', () => {
  const el = fakeEl({ scrollTop: 600 });
  installDom(el);
  mod.followBottom('log', true);
  assert.equal(el.scrollTop, el.scrollHeight);
  assert.equal(el.parentElement.dataset.atBottom, 'true');
});

test('followBottom does not move once the user scrolled away', () => {
  const el = fakeEl({ scrollTop: 600 });
  installDom(el);
  mod.followBottom('log', true);
  el.scrollTop = 100;
  el.emit('scroll');
  el.scrollHeight = 1200;
  mod.followBottom('log', true);
  assert.equal(el.scrollTop, 100);
  assert.equal(el.parentElement.dataset.atBottom, 'false');
});

test('smooth scroll keeps following through intermediate positions', () => {
  const el = fakeEl({ scrollTop: 100 });
  installDom(el);
  mod.followBottom('log', true);
  el.scrollTop = 100;
  el.emit('scroll'); // far from bottom => unstuck
  assert.equal(el.parentElement.dataset.atBottom, 'false');

  mod.scrollToBottom('log');
  assert.equal(el.scrolledTo, el.scrollHeight);
  el.scrollTop = 300;
  el.emit('scroll'); // mid-animation
  el.scrollHeight = 1100; // new message arrives mid-animation
  mod.followBottom('log', true);
  assert.equal(el.parentElement.dataset.atBottom, 'true');
  assert.equal(el.scrollTop, 300, 'follow must not fight the animation');

  el.scrollTop = 700;
  el.emit('scroll'); // landed
  mod.followBottom('log', true);
  assert.equal(el.scrollTop, el.scrollHeight, 'stuck again after landing');
});

test('a manual gesture cancels the smooth scroll', () => {
  const el = fakeEl({ scrollTop: 100 });
  installDom(el);
  mod.followBottom('log', true);
  mod.scrollToBottom('log');
  el.emit('wheel');
  el.scrollTop = 200;
  el.emit('scroll');
  assert.equal(el.parentElement.dataset.atBottom, 'false');
});

test('scrollend ends the smooth scroll even when it stops short of the bottom', () => {
  const el = fakeEl({ scrollTop: 100 });
  installDom(el);
  mod.followBottom('log', true);
  mod.scrollToBottom('log');
  el.scrollHeight = 2000; // content grew: the animation lands far from the new end
  el.scrollTop = 600;
  el.emit('scrollend');
  assert.equal(el.parentElement.dataset.atBottom, 'false');
  el.scrollTop = 300;
  el.emit('scroll'); // user scroll is no longer swallowed
  assert.equal(el.parentElement.dataset.atBottom, 'false');
  mod.followBottom('log', true);
  assert.equal(el.scrollTop, 300, 'not stuck, so follow does not move');
});

test('a safety timeout clears the smooth flag when scrollend never fires', () => {
  const el = fakeEl({ scrollTop: 100 });
  installDom(el);
  mod.followBottom('log', true);
  mod.scrollToBottom('log');
  el.scrollHeight = 2000;
  el.scrollTop = 600;
  fireTimers();
  assert.equal(el.parentElement.dataset.atBottom, 'false');
  el.scrollTop = 300;
  el.emit('scroll');
  assert.equal(el.parentElement.dataset.atBottom, 'false');
});

test('resetToBottom jumps directly to the end even after user scrolled away', () => {
  const el = fakeEl({ scrollTop: 600 });
  installDom(el);
  mod.followBottom('log', true);
  // User scrolls up away from bottom
  el.scrollTop = 100;
  el.emit('scroll');
  assert.equal(el.parentElement.dataset.atBottom, 'false');

  // Entering the window / resetToBottom forces scroll to bottom and restores stuck flag
  mod.resetToBottom('log');
  assert.equal(el.scrollTop, el.scrollHeight);
  assert.equal(el.parentElement.dataset.atBottom, 'true');

  // New content arriving continues to be followed
  el.scrollHeight = 1500;
  mod.followBottom('log', true);
  assert.equal(el.scrollTop, 1500);

  // But if the user scrolls up again, their position is respected
  el.scrollTop = 200;
  el.emit('scroll');
  assert.equal(el.parentElement.dataset.atBottom, 'false');
  el.scrollHeight = 1800;
  mod.followBottom('log', true);
  assert.equal(el.scrollTop, 200);
});

