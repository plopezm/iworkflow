// Drag-to-resize for the desktop side panels (CollapsePanel). A handle (data-resize-handle) sits on the inner
// edge of each panel; dragging it, or pressing the arrow keys while it has focus, changes the panel width.
// The width lives in the --panel-width custom property of the panel root (web/tailwind.css applies it from the
// md breakpoint up) and is stored in localStorage per side. Double click restores the default width.

export const MIN_WIDTH = 200;
export const MAX_WIDTH = 720;
const KEY_STEP = 16;

/** Keeps a width between the minimum and the smaller of the maximum and half the viewport. */
export function clampWidth(width, viewport) {
  const max = Math.max(MIN_WIDTH, Math.min(MAX_WIDTH, Math.floor(viewport / 2)));
  return Math.min(max, Math.max(MIN_WIDTH, Math.round(width)));
}

/** The width a drag gives the panel: a left panel grows toward the right, a right panel toward the left. */
export function draggedWidth(side, startWidth, startX, currentX, viewport) {
  const travel = currentX - startX;
  return clampWidth(side === 'right' ? startWidth - travel : startWidth + travel, viewport);
}

/** The stored width, or null when nothing usable is stored. */
export function parseStoredWidth(stored) {
  if (stored === null || stored === undefined) return null;
  const value = Number(stored);
  return Number.isFinite(value) && value > 0 ? value : null;
}

/** The width after an arrow key (the edge moves toward the arrow), or null for any other key. */
export function steppedWidth(side, width, key, viewport) {
  if (key !== 'ArrowLeft' && key !== 'ArrowRight') return null;
  const towardRight = key === 'ArrowRight';
  const grows = side === 'left' ? towardRight : !towardRight;
  return clampWidth(width + (grows ? KEY_STEP : -KEY_STEP), viewport);
}

// ----- browser wiring -------------------------------------------------------------------------

function storage() {
  try {
    return globalThis.localStorage;
  } catch {
    return null;
  }
}

function applyWidth(root, width) {
  if (width === null) root.style.removeProperty('--panel-width');
  else root.style.setProperty('--panel-width', `${width}px`);
}

function save(handle, width) {
  const key = handle.dataset.resizeKey;
  if (!key || !storage()) return;
  if (width === null) storage().removeItem(key);
  else storage().setItem(key, String(width));
}

function currentWidth(root) {
  return root.querySelector(':scope > aside')?.getBoundingClientRect().width ?? 0;
}

function restore(scope = document) {
  const handles = [...(scope.querySelectorAll?.('[data-resize-handle]') ?? [])];
  for (const handle of handles) {
    const root = handle.closest('[data-collapse-root]');
    const width = parseStoredWidth(storage()?.getItem(handle.dataset.resizeKey ?? '') ?? null);
    if (root && width !== null) applyWidth(root, clampWidth(width, globalThis.innerWidth));
  }
}

function init() {
  let drag = null;
  document.addEventListener('pointerdown', (event) => {
    const handle = event.target?.closest?.('[data-resize-handle]');
    const root = handle?.closest('[data-collapse-root]');
    if (!handle || !root) return;
    event.preventDefault();
    handle.setPointerCapture?.(event.pointerId);
    root.setAttribute('data-resizing', '');
    drag = { handle, root, side: handle.dataset.side, startX: event.clientX, startWidth: currentWidth(root) };
  });
  document.addEventListener('pointermove', (event) => {
    if (!drag) return;
    applyWidth(drag.root, draggedWidth(drag.side, drag.startWidth, drag.startX, event.clientX, globalThis.innerWidth));
  });
  const end = () => {
    if (!drag) return;
    drag.root.removeAttribute('data-resizing');
    save(drag.handle, Math.round(currentWidth(drag.root)));
    drag = null;
  };
  document.addEventListener('pointerup', end);
  document.addEventListener('pointercancel', end);
  document.addEventListener('dblclick', (event) => {
    const handle = event.target?.closest?.('[data-resize-handle]');
    const root = handle?.closest('[data-collapse-root]');
    if (!handle || !root) return;
    applyWidth(root, null);
    save(handle, null);
  });
  document.addEventListener('keydown', (event) => {
    const handle = event.target?.closest?.('[data-resize-handle]');
    const root = handle?.closest('[data-collapse-root]');
    if (!handle || !root) return;
    const width = steppedWidth(handle.dataset.side, currentWidth(root), event.key, globalThis.innerWidth);
    if (width === null) return;
    event.preventDefault();
    applyWidth(root, width);
    save(handle, width);
  });
  // A swap that renders a panel again (a task opens) loses the inline property: put the stored width back.
  document.body.addEventListener('htmx:afterSettle', (event) => {
    if (event.target instanceof Element) restore(event.target);
  });
  restore();
}

if (typeof document !== 'undefined') {
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
}
