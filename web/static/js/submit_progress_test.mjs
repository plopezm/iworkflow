// Run with: node --test web/static/js/
import test, { mock } from 'node:test';
import assert from 'node:assert/strict';
import { visibleSteps, showStage, formatElapsed, startProgress, stopProgress, installSubmitProgress } from './submit_progress.js';

function step(phase) {
  return { hidden: false, dataset: { progressPhase: phase, state: 'pending' } };
}
function overlay(steps) {
  return { querySelectorAll: () => steps };
}
function form(startNow) {
  return { querySelector: () => (startNow === null ? null : { checked: startNow }) };
}

test('the start steps are hidden when Start now is unchecked', () => {
  const steps = [step(''), step('start'), step('start')];
  assert.equal(visibleSteps(form(false), overlay(steps)).length, 1);
  assert.deepEqual(steps.map((s) => s.hidden), [false, true, true]);
});

test('every step shows when Start now is checked or the form has no such box', () => {
  assert.equal(visibleSteps(form(true), overlay([step(''), step('start')])).length, 2);
  assert.equal(visibleSteps(form(null), overlay([step(''), step('start')])).length, 2);
});

test('showStage marks done, active and pending steps', () => {
  const steps = [step(''), step(''), step('')];
  showStage(steps, 1);
  assert.deepEqual(steps.map((s) => s.dataset.state), ['done', 'active', 'pending']);
});

test('formatElapsed counts whole seconds', () => {
  assert.equal(formatElapsed(0), '0 s');
  assert.equal(formatElapsed(12999), '12 s');
});

// A panel with the parts the clock writes: the steps, the seconds and the slow hint.
function livePanel({ connected = true, steps = 3 } = {}) {
  const items = Array.from({ length: steps }, () => step(''));
  const elapsed = { textContent: '' };
  const slow = { hidden: true };
  const panel = {
    isConnected: connected,
    querySelectorAll: () => items,
    querySelector: (selector) => (selector === '[data-progress-elapsed]' ? elapsed : slow),
  };
  return { panel, items, elapsed, slow };
}

test('the clock moves the stage along, counts seconds and shows the hint when the wait is long', () => {
  mock.timers.enable({ apis: ['setInterval', 'Date'] });
  try {
    const { panel, items, elapsed, slow } = livePanel();
    startProgress(form(true), panel);
    assert.deepEqual(items.map((s) => s.dataset.state), ['active', 'pending', 'pending']);
    assert.equal(slow.hidden, true);
    mock.timers.tick(3000);
    assert.deepEqual(items.map((s) => s.dataset.state), ['done', 'active', 'pending']);
    assert.equal(elapsed.textContent, '3 s');
    mock.timers.tick(10000);
    assert.deepEqual(items.map((s) => s.dataset.state), ['done', 'done', 'active']);
    assert.equal(slow.hidden, false);
    stopProgress(panel);
  } finally {
    mock.timers.reset();
  }
});

test('stopProgress ends the clock and is safe to call twice', () => {
  mock.timers.enable({ apis: ['setInterval', 'Date'] });
  try {
    const { panel, elapsed } = livePanel();
    startProgress(form(true), panel);
    mock.timers.tick(1000);
    assert.equal(elapsed.textContent, '1 s');
    stopProgress(panel);
    stopProgress(panel);
    mock.timers.tick(5000);
    assert.equal(elapsed.textContent, '1 s');
  } finally {
    mock.timers.reset();
  }
});

test('the clock stops by itself when the panel left the document before an event could stop it', () => {
  mock.timers.enable({ apis: ['setInterval', 'Date'] });
  try {
    const { panel, elapsed } = livePanel();
    startProgress(form(true), panel);
    mock.timers.tick(1000);
    assert.equal(elapsed.textContent, '1 s');
    panel.isConnected = false;
    mock.timers.tick(500);
    mock.timers.tick(5000);
    assert.equal(elapsed.textContent, '1 s');
  } finally {
    mock.timers.reset();
  }
});

test('htmx events start the clock on the form and the cleanup of the form stops it', () => {
  mock.timers.enable({ apis: ['setInterval', 'Date'] });
  try {
    const { panel, elapsed } = livePanel();
    const formElement = {
      matches: (selector) => selector === 'form',
      querySelector: () => panel,
    };
    const listeners = {};
    installSubmitProgress({ addEventListener: (name, handler) => { listeners[name] = handler; } });
    assert.ok(listeners['htmx:beforeRequest'] && listeners['htmx:afterRequest'] && listeners['htmx:beforeCleanupElement']);
    listeners['htmx:beforeRequest']({ target: { ...formElement, querySelector: () => null } });
    mock.timers.tick(2000);
    assert.equal(elapsed.textContent, '', 'a form without a panel is left alone');
    // Same form, with its panel (the stand-in form answers the child lookup).
    formElement.querySelector = (selector) => (selector === '[name="start_now"]' ? null : panel);
    listeners['htmx:beforeRequest']({ target: formElement });
    mock.timers.tick(2000);
    assert.equal(elapsed.textContent, '2 s');
    listeners['htmx:beforeCleanupElement']({ target: formElement });
    mock.timers.tick(5000);
    assert.equal(elapsed.textContent, '2 s');
  } finally {
    mock.timers.reset();
  }
});
