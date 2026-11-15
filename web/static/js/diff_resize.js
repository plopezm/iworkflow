// Drag-to-resize for the changed-files tree of the diff view. A handle (data-diff-resize-handle) sits between the
// tree (data-diff-sidebar) and the file cards; dragging it, or pressing the arrow keys while it has focus,
// changes the tree width. The width is stored in localStorage and put back after every htmx swap of the diff.
// Double click restores the default width. It mirrors the side panels (panel_resize.js) and shares their limits.

import { clampWidth, draggedWidth, parseStoredWidth, steppedWidth } from './panel_resize.js';

export const STORAGE_KEY = 'iwf.diff.tree.width';

// ----- browser wiring -------------------------------------------------------------------------

function storage() {
  try {
    return globalThis.localStorage;
  } catch {
    return null;
  }
}

function applyWidth(sidebar, width) {
  if (width === null) sidebar.style.removeProperty('width');
  else sidebar.style.width = `${width}px`;
}

function save(width) {
  if (!storage()) return;
  if (width === null) storage().removeItem(STORAGE_KEY);
  else storage().setItem(STORAGE_KEY, String(width));
}

function sidebarOf(handle) {
  return handle.parentElement?.querySelector(':scope > [data-diff-sidebar]') ?? null;
}

function restore(scope = document) {
  const width = parseStoredWidth(storage()?.getItem(STORAGE_KEY) ?? null);
  if (width === null) return;
  for (const sidebar of scope.querySelectorAll?.('[data-diff-sidebar]') ?? []) {
    applyWidth(sidebar, clampWidth(width, globalThis.innerWidth));
  }
}

function init() {
  let drag = null;
  document.addEventListener('pointerdown', (event) => {
    const handle = event.target?.closest?.('[data-diff-resize-handle]');
    const sidebar = handle && sidebarOf(handle);
    if (!handle || !sidebar) return;
    event.preventDefault();
    handle.setPointerCapture?.(event.pointerId);
    document.body.classList.add('dock-resizing');
    drag = { sidebar, startX: event.clientX, startWidth: sidebar.getBoundingClientRect().width };
  });
  document.addEventListener('pointermove', (event) => {
    if (!drag) return;
    applyWidth(drag.sidebar, draggedWidth('left', drag.startWidth, drag.startX, event.clientX, globalThis.innerWidth));
  });
  const end = () => {
    if (!drag) return;
    document.body.classList.remove('dock-resizing');
    save(Math.round(drag.sidebar.getBoundingClientRect().width));
    drag = null;
  };
  document.addEventListener('pointerup', end);
  document.addEventListener('pointercancel', end);
  document.addEventListener('dblclick', (event) => {
    if (!event.target?.closest?.('[data-diff-resize-handle]')) return;
    for (const sidebar of document.querySelectorAll('[data-diff-sidebar]')) applyWidth(sidebar, null);
    save(null);
  });
  document.addEventListener('keydown', (event) => {
    const handle = event.target?.closest?.('[data-diff-resize-handle]');
    const sidebar = handle && sidebarOf(handle);
    if (!sidebar) return;
    const width = steppedWidth('left', sidebar.getBoundingClientRect().width, event.key, globalThis.innerWidth);
    if (width === null) return;
    event.preventDefault();
    applyWidth(sidebar, width);
    save(width);
  });
  // Every layout button swaps the whole diff view, and the tab itself may arrive after the page loaded: both drop the
  // inline width, so put the stored one back whenever htmx settles or loads content.
  const again = () => restore();
  document.body.addEventListener('htmx:afterSettle', again);
  document.body.addEventListener('htmx:load', again);
  restore();
}

if (typeof document !== 'undefined') {
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
}
