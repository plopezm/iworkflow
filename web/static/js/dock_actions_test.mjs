// Run with: node --test web/static/js/
// Ordering of dock actions and the data-dock-leave hook of the htmx afterSettle handler. These tests
// need no DOM: the handler receives plain objects shaped like the elements it reads.
import test from 'node:test';
import assert from 'node:assert/strict';
import * as dock from './dock.js';

test('a dock request waits for the previous one to settle before it is sent', () => {
  const sent = [];
  const queue = dock.createRequestQueue();
  queue.enqueue(() => sent.push('open /workspaces'));
  queue.enqueue(() => sent.push('open /models'));
  assert.deepEqual(sent, ['open /workspaces'], 'the second request must not overtake the first');
  queue.settle();
  assert.deepEqual(sent, ['open /workspaces', 'open /models'], 'settling releases the next request');
});

test('a request runs at enqueue time when the dock is idle', () => {
  const sent = [];
  const queue = dock.createRequestQueue();
  queue.enqueue(() => sent.push('close_tab'));
  assert.deepEqual(sent, ['close_tab']);
});

test('the queue keeps the order of the requests it was given', () => {
  const sent = [];
  const queue = dock.createRequestQueue();
  queue.enqueue(() => sent.push('a'));
  queue.enqueue(() => sent.push('b'));
  queue.enqueue(() => sent.push('c'));
  queue.settle();
  queue.settle();
  assert.deepEqual(sent, ['a', 'b', 'c']);
});

// A stand-in for an element of the swapped dock. Only the members handleAfterSettle reads exist.
function element({ id = '', dataset = {}, leave = null, pane = null, children = 0 } = {}) {
  return {
    id,
    dataset,
    childElementCount: children,
    matches: () => false,
    querySelector: (selector) => (selector === '[data-dock-leave]' ? leave : null),
    closest: (selector) => (selector === '[data-dock-pane]' ? pane : null),
  };
}

test('a response marking data-dock-leave closes its tab and opens the named path', () => {
  const calls = [];
  const pane = element({ dataset: { tabId: '7', path: '/tasks/1/edit' }, children: 1 });
  const leave = element({ dataset: { dockLeave: '/' } });
  const target = element({ leave, pane });
  dock.handleAfterSettle({ target }, (op, values) => calls.push([op, values]));
  assert.deepEqual(calls, [['close_and_open', { tab: '7', path: '/' }]]);
});

test('a settled element without the leave marker does not close the tab', () => {
  const calls = [];
  const pane = element({ dataset: { tabId: '7', path: '/tasks/1/edit' }, children: 1 });
  const target = element({ pane });
  dock.handleAfterSettle({ target }, (op, values) => calls.push([op, values]));
  assert.deepEqual(calls, []);
});

// A pointer down focuses its group, except on the group's actions and on its menus (the "+" and "⋯").
function pointerTarget({ action = null, menu = null, inGroup = true } = {}) {
  return {
    closest: (selector) => {
      if (selector === '[data-dock-action]') return action;
      if (selector === '[data-menu]') return menu;
      if (selector === 'section[data-dock-group]') return inGroup ? {} : null;
      return null;
    },
  };
}

test('a pointer down on a group body focuses that group', () => {
  assert.equal(dock.focusesGroup(pointerTarget()), true);
});

test('a pointer down on a menu of the group does not focus it, so the open menu survives', () => {
  assert.equal(dock.focusesGroup(pointerTarget({ menu: {} })), false);
});

test('a pointer down on a dock action does not focus the group', () => {
  assert.equal(dock.focusesGroup(pointerTarget({ action: {} })), false);
});
