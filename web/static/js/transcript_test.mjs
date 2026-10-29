// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';

function fakeElement({ id, isConnected = true, hidden = false, scrollHeight = 1000, clientHeight = 400, scrollTop = 600 } = {}) {
  const listeners = {};
  const dataset = {};
  const classList = {
    contains: (cls) => (cls === 'hidden' ? hidden : false),
  };
  const el = {
    id,
    isConnected,
    dataset,
    classList,
    scrollHeight,
    clientHeight,
    scrollTop,
    parentElement: null,
    firstElementChild: null,
    offsetWidth: hidden ? 0 : 400,
    offsetHeight: hidden ? 0 : 300,
    addEventListener: (type, fn) => { listeners[type] = fn; },
    emit: (type) => listeners[type]?.(),
    closest: (sel) => {
      if (sel === '[data-dock-pane]') {
        return { classList };
      }
      return null;
    },
  };
  return el;
}

// Global DOM mock for Node tests
globalThis.requestAnimationFrame = (fn) => fn();
globalThis.MutationObserver = class { observe() {} };

const mod = await import('./transcript.js');

test('isElementVisible returns false for hidden dock panes', () => {
  const hiddenScroller = fakeElement({ id: 'scroll-1', hidden: true });
  assert.equal(mod.isElementVisible(hiddenScroller), false);

  const visibleScroller = fakeElement({ id: 'scroll-2', hidden: false });
  assert.equal(mod.isElementVisible(visibleScroller), true);
});

test('checkVisibility resets to bottom on transition to visible', () => {
  const scroller = fakeElement({ id: 'task-scroll-vis', hidden: false, scrollTop: 0, scrollHeight: 2000, clientHeight: 500 });
  scroller.parentElement = { dataset: {} };
  globalThis.document = { getElementById: (id) => (id === scroller.id ? scroller : null) };

  // First time entering / becoming visible
  mod.checkVisibility(scroller);
  assert.equal(scroller.scrollTop, scroller.scrollHeight, 'should jump to bottom on entering');
  assert.equal(scroller.__iwfStuck, true, 'should be marked stuck');

  // User scrolls up
  scroller.scrollTop = 200;
  scroller.emit('scroll');
  assert.equal(scroller.__iwfStuck, false);

  // Calling checkVisibility while already visible does not reset scroll
  mod.checkVisibility(scroller);
  assert.equal(scroller.scrollTop, 200, 'stay where user scrolled while remaining visible');

  // Pane becomes hidden (user leaves tab)
  scroller.classList.contains = (c) => c === 'hidden';
  scroller.offsetWidth = 0;
  mod.checkVisibility(scroller);

  // Pane becomes visible again (user enters tab again)
  scroller.classList.contains = () => false;
  scroller.offsetWidth = 400;
  mod.checkVisibility(scroller);
  assert.equal(scroller.scrollTop, scroller.scrollHeight, 'must go back to bottom when entering again');
});
