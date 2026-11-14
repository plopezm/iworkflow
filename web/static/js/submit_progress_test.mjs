// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';
import { visibleSteps, showStage, formatElapsed } from './submit_progress.js';

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
