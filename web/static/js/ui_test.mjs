// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';
import { resolveTheme, shouldArmToast, shouldLoadPane, sidebarIsOpen, sidebarToggleLabel, switchBoardTab, restoreBoardTab, BOARD_TAB_STORAGE_KEY } from './ui.js';

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

test('switchBoardTab updates panes data-tab, button aria-selected and segment-selected class', () => {
  const panes = {
    attrs: {},
    setAttribute(k, v) { this.attrs[k] = v; },
    querySelector() { return null; },
  };

  const btnLoose = {
    dataset: { boardTab: 'loose' },
    attrs: { 'aria-selected': 'false' },
    classes: new Set(),
    setAttribute(k, v) { this.attrs[k] = v; },
    classList: {
      toggle(cls, state) {
        if (state) btnLoose.classes.add(cls);
        else btnLoose.classes.delete(cls);
      },
    },
    closest(sel) {
      if (sel === '.board-tabs') return tabs;
      return null;
    },
  };

  const btnGraph = {
    dataset: { boardTab: 'graph' },
    attrs: { 'aria-selected': 'true' },
    classes: new Set(['segment-selected']),
    setAttribute(k, v) { this.attrs[k] = v; },
    classList: {
      toggle(cls, state) {
        if (state) btnGraph.classes.add(cls);
        else btnGraph.classes.delete(cls);
      },
    },
    closest(sel) {
      if (sel === '.board-tabs') return tabs;
      return null;
    },
  };

  const tabs = {
    querySelectorAll(sel) {
      if (sel === '[data-board-tab]') return [btnLoose, btnGraph];
      return [];
    },
    closest(sel) {
      if (sel === '.board-list') return boardList;
      return null;
    },
    parentElement: null,
  };

  const boardList = {
    querySelector(sel) {
      if (sel === '.board-panes') return panes;
      return null;
    },
  };

  const storageMap = new Map();
  globalThis.sessionStorage = {
    setItem(k, v) { storageMap.set(k, v); },
    getItem(k) { return storageMap.get(k) || null; },
  };

  // Switch to loose
  switchBoardTab(btnLoose);
  assert.equal(panes.attrs['data-tab'], 'loose');
  assert.equal(btnLoose.attrs['aria-selected'], 'true');
  assert.equal(btnLoose.classes.has('segment-selected'), true);
  assert.equal(btnGraph.attrs['aria-selected'], 'false');
  assert.equal(btnGraph.classes.has('segment-selected'), false);
  assert.equal(storageMap.get(BOARD_TAB_STORAGE_KEY), 'loose');

  // Switch to graph
  switchBoardTab(btnGraph);
  assert.equal(panes.attrs['data-tab'], 'graph');
  assert.equal(btnGraph.attrs['aria-selected'], 'true');
  assert.equal(btnGraph.classes.has('segment-selected'), true);
  assert.equal(btnLoose.attrs['aria-selected'], 'false');
  assert.equal(btnLoose.classes.has('segment-selected'), false);
  assert.equal(storageMap.get(BOARD_TAB_STORAGE_KEY), 'graph');
});

test('restoreBoardTab restores saved tab on narrow screens and ignores on desktop', () => {
  const panes = {
    attrs: { 'data-tab': 'graph' },
    setAttribute(k, v) { this.attrs[k] = v; },
    querySelector() { return null; },
  };

  const btnLoose = {
    dataset: { boardTab: 'loose' },
    attrs: { 'aria-selected': 'false' },
    classes: new Set(),
    setAttribute(k, v) { this.attrs[k] = v; },
    classList: { toggle() {} },
    closest(sel) { return sel === '.board-tabs' ? tabs : null; },
  };

  const btnGraph = {
    dataset: { boardTab: 'graph' },
    attrs: { 'aria-selected': 'true' },
    classes: new Set(['segment-selected']),
    setAttribute(k, v) { this.attrs[k] = v; },
    classList: { toggle() {} },
    closest(sel) { return sel === '.board-tabs' ? tabs : null; },
  };

  const tabs = {
    querySelector(sel) {
      if (sel === '[data-board-tab="loose"]') return btnLoose;
      if (sel === '[data-board-tab="graph"]') return btnGraph;
      return null;
    },
    querySelectorAll(sel) {
      if (sel === '[data-board-tab]') return [btnLoose, btnGraph];
      return [];
    },
    closest(sel) { return sel === '.board-list' ? boardList : null; },
  };

  const boardList = {
    querySelector(sel) { return sel === '.board-panes' ? panes : null; },
  };

  const root = {
    querySelectorAll(sel) { return sel === '.board-tabs' ? [tabs] : []; },
  };

  globalThis.sessionStorage = {
    getItem() { return 'loose'; },
    setItem() {},
  };

  // On desktop (> 767), should do nothing
  globalThis.innerWidth = 1024;
  restoreBoardTab(root);
  assert.equal(panes.attrs['data-tab'], 'graph');

  // On mobile (<= 767), should restore 'loose'
  globalThis.innerWidth = 400;
  restoreBoardTab(root);
  assert.equal(panes.attrs['data-tab'], 'loose');
});
