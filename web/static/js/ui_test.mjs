// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';
import { resolveTheme, shouldArmToast, shouldLoadPane, sidebarIsOpen, sidebarToggleLabel } from './ui.js';

test('a stored light or dark theme wins over the system preference', () => {
  assert.equal(resolveTheme('light', true), 'light');
  assert.equal(resolveTheme('dark', false), 'dark');
});

test('system (or nothing stored) follows the system preference', () => {
  assert.equal(resolveTheme('system', true), 'dark');
  assert.equal(resolveTheme(null, false), 'light');
  assert.equal(resolveTheme('garbage', true), 'dark');
});

test('the sidebar is open unless it was explicitly stored closed', () => {
  assert.equal(sidebarIsOpen(null), true);
  assert.equal(sidebarIsOpen('1'), true);
  assert.equal(sidebarIsOpen('garbage'), true);
  assert.equal(sidebarIsOpen('0'), false);
});

test('a toast arms its timer once', () => {
  const fresh = { dataset: {} };
  const armed = { dataset: { toastArmed: 'true' } };
  assert.equal(shouldArmToast(fresh), true);
  assert.equal(shouldArmToast(armed), false);
});

test('a keep-alive pane loads once, when it is visible, and dock panes are left to the dock', () => {
  const visible = { dataset: { src: '/dock/pane?path=%2Fsettings' }, classList: { contains: () => false } };
  const hidden = { dataset: { src: '/x' }, classList: { contains: (name) => name === 'hidden' } };
  const loaded = { dataset: { loaded: 'true' }, classList: { contains: () => false } };
  const dockPane = { dataset: { dockPane: '' }, classList: { contains: () => false } };
  assert.equal(shouldLoadPane(visible), true);
  assert.equal(shouldLoadPane(hidden), false);
  assert.equal(shouldLoadPane(loaded), false);
  assert.equal(shouldLoadPane(dockPane), false);
});

test('a keep-alive pane without a src renders inline and never fetches the page URL', () => {
  const inline = { dataset: { src: '' }, classList: { contains: () => false } };
  assert.equal(shouldLoadPane(inline), false);
});

test('the sidebar toggle offers to show the sidebar while it is hidden and to hide it while it is shown', () => {
  const dataset = { labelHide: 'Ocultar barra lateral', labelShow: 'Mostrar barra lateral' };
  assert.equal(sidebarToggleLabel(dataset, true), 'Mostrar barra lateral');
  assert.equal(sidebarToggleLabel(dataset, false), 'Ocultar barra lateral');
});

test('a sidebar toggle without translated labels offers no label instead of the text "undefined"', () => {
  assert.equal(sidebarToggleLabel({}, true), undefined);
  assert.equal(sidebarToggleLabel({ labelHide: 'Ocultar' }, true), undefined);
  assert.equal(sidebarToggleLabel({ labelShow: 'Mostrar' }, false), undefined);
});
