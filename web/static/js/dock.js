// Dock behaviour in the browser: the layout is kept in localStorage, every change is an htmx
// request that returns the new dock fragment, and the tab panes are re-attached after each swap
// so their state (unsaved text, scroll) survives. Replaces src/core/dock/dock.js and the
// browser glue of src/core/dock/context.rs and host.rs.
//
// The pure helpers are exported for the Node tests (dock_test.mjs); the wiring runs only when a
// document exists.

export const LAYOUT_STORAGE_KEY = 'iwf-dock-layout';
export const NARROW_QUERY = '(max-width: 767px)';

const MIN_RATIO = 0.12;
const MAX_RATIO = 0.88;

// Maps the data-dock-* attributes of an element to the form fields of a dock request:
// data-dock-group → group, data-dock-tab → tab, … The action name itself is not a field.
export function actionValues(dataset) {
  const values = {};
  for (const [key, value] of Object.entries(dataset)) {
    if (!key.startsWith('dock') || key === 'dockAction') continue;
    const name = key.slice('dock'.length);
    values[name.charAt(0).toLowerCase() + name.slice(1)] = value;
  }
  return values;
}

// True when a tab dropped at clientX lands on the right half of its rectangle (insert after it).
export function dropAfter(clientX, rect) {
  return clientX > rect.left + rect.width / 2;
}

// Clamps a divider position (0..1) to the allowed range of the first child.
export function clampRatio(value) {
  return Math.min(MAX_RATIO, Math.max(MIN_RATIO, value));
}

// Position of the divider inside its container, as a fraction, for the drag in progress.
export function ratioAt(clientPosition, start, size) {
  return clampRatio((clientPosition - start) / size);
}

// Decides which kept panes can be re-attached after a dock swap. `placeholders` and `kept` are
// arrays of { id, path }: the tabs of the new fragment and the panes still in memory. A kept pane
// is reused only while its tab shows the same path. A tab whose path changed (replace) or that was
// closed gets its pane discarded, so the new placeholder loads its own content.
export function reconcilePanes(placeholders, kept) {
  const wanted = new Map(placeholders.map((tab) => [tab.id, tab.path]));
  const reattach = [];
  const discard = [];
  for (const pane of kept) {
    if (wanted.get(pane.id) === pane.path) reattach.push(pane.id);
    else discard.push(pane.id);
  }
  return { reattach, discard };
}

// Which action a tab's close handle or menu item posts; the nearest action wins over its ancestors.
export function nearestAction(target) {
  return target && target.closest ? target.closest('[data-dock-action]') : null;
}

// Address of the page of a path in the address bar; the root path has no tab to show.
export function addressFor(focusedPath) {
  return focusedPath ? focusedPath : null;
}

// ----- browser wiring -------------------------------------------------------------------------

function readLayout() {
  try {
    return localStorage.getItem(LAYOUT_STORAGE_KEY) || '';
  } catch {
    return '';
  }
}

function writeLayout(json) {
  try {
    localStorage.setItem(LAYOUT_STORAGE_KEY, json);
  } catch {
    // Storage can be full or disabled: the layout still works for this page view.
  }
}

// Panes that are not in the DOM right now, keyed by tab id. They keep their content while the
// dock is rebuilt around them.
const kept = new Map();

function stash() {
  let container = document.getElementById('dock-stash');
  if (!container) {
    container = document.createElement('div');
    container.id = 'dock-stash';
    container.hidden = true;
    document.body.appendChild(container);
  }
  return container;
}

// Before htmx replaces the dock: lift every pane out so the swap cannot remove it.
function keepPanes(dock) {
  for (const pane of dock.querySelectorAll('[data-dock-pane][data-tab-id]')) {
    kept.set(pane.dataset.tabId, pane);
    stash().appendChild(pane);
  }
}

// After the swap: put the kept panes back into their new placeholders, and drop the panes of
// closed tabs.
function attachPanes(dock) {
  const placeholders = [...dock.querySelectorAll('[data-dock-pane][data-tab-id]')];
  const { reattach, discard } = reconcilePanes(
    placeholders.map((el) => ({ id: el.dataset.tabId, path: el.dataset.path })),
    [...kept].map(([id, pane]) => ({ id, path: pane.dataset.path })),
  );
  for (const placeholder of placeholders) {
    const id = placeholder.dataset.tabId;
    if (!reattach.includes(id)) continue;
    const pane = kept.get(id);
    pane.className = placeholder.className;
    placeholder.replaceWith(pane);
    kept.delete(id);
  }
  for (const id of discard) {
    kept.get(id)?.remove();
    kept.delete(id);
  }
}

function dockAction(op, values) {
  htmx.ajax('POST', `/dock/action/${op}`, {
    target: '#dock',
    swap: 'outerHTML',
    values: { layout: readLayout(), ...values },
  });
}

function restore(boot) {
  htmx.ajax('POST', '/dock/layout', {
    target: '#dock',
    swap: 'outerHTML',
    values: { layout: readLayout(), boot: boot || '' },
  });
}

function isNarrow() {
  return window.matchMedia(NARROW_QUERY).matches;
}

function groupOf(element) {
  const group = element.closest('[data-dock-group]');
  return group ? group.dataset.dockGroup : '';
}

function openLink(link, event) {
  if (event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey) return;
  event.preventDefault();
  const path = link.dataset.dockPath;
  const title = link.dataset.dockTitle || '';
  const group = groupOf(link);
  if (link.dataset.dockReuse) {
    dockAction('open_or_reuse', { path, reuse: link.dataset.dockReuse, title, group });
  } else if (link.dataset.dockBeside !== undefined && !isNarrow()) {
    dockAction('open_beside', { path, title, group });
  } else {
    dockAction('open', { path, title, group });
  }
}

let dragging = null;

function setOverlays(visible) {
  document.body.classList.toggle('dock-dragging', visible);
  for (const overlay of document.querySelectorAll('[data-dock-overlay]')) {
    overlay.hidden = !visible;
  }
}

function resize(divider, event) {
  const container = divider.parentElement;
  const first = divider.previousElementSibling;
  if (!container || !first) return;
  event.preventDefault();
  const vertical = container.classList.contains('flex-col');
  const rect = container.getBoundingClientRect();
  let ratio = null;
  const move = (e) => {
    ratio = vertical
      ? ratioAt(e.clientY, rect.top, rect.height)
      : ratioAt(e.clientX, rect.left, rect.width);
    first.style.flexBasis = `${ratio * 100}%`;
  };
  const finish = () => {
    window.removeEventListener('pointermove', move);
    window.removeEventListener('pointerup', finish);
    window.removeEventListener('pointercancel', finish);
    document.body.classList.remove('dock-resizing');
    if (ratio !== null) dockAction('set_ratio', { split: divider.dataset.dockSplit, ratio: String(ratio) });
  };
  document.body.classList.add('dock-resizing');
  window.addEventListener('pointermove', move);
  window.addEventListener('pointerup', finish);
  window.addEventListener('pointercancel', finish);
}

function onClick(event) {
  const link = event.target.closest?.('a[data-dock-link]');
  if (link) {
    openLink(link, event);
    return;
  }
  const action = nearestAction(event.target);
  if (!action || action.disabled) return;
  const values = actionValues(action.dataset);
  if (values.tab === '0') return;
  dockAction(action.dataset.dockAction, values);
}

function onAuxClick(event) {
  if (event.button !== 1) return;
  const tab = event.target.closest?.('[data-dock-tab-button]');
  if (tab) dockAction('close_tab', { tab: tab.dataset.dockTab });
}

function onPointerDown(event) {
  const divider = event.target.closest?.('[data-dock-divider]');
  if (divider) {
    resize(divider, event);
    return;
  }
  if (nearestAction(event.target)) return;
  const group = event.target.closest?.('section[data-dock-group]');
  if (group && !group.classList.contains('dock-group-focused')) {
    dockAction('focus', { group: group.dataset.dockGroup });
  }
}

function onDragStart(event) {
  const tab = event.target.closest?.('[data-dock-tab-button]');
  if (!tab || !event.dataTransfer) return;
  event.dataTransfer.setData('text/plain', tab.dataset.dockTab);
  event.dataTransfer.effectAllowed = 'move';
  dragging = tab.dataset.dockTab;
  setOverlays(true);
}

function onDragEnd() {
  dragging = null;
  setOverlays(false);
}

function onDragOver(event) {
  if (event.target.closest?.('[data-dock-tab-button], .dock-tabbar, [data-dock-zone]')) {
    event.preventDefault();
  }
  const zone = event.target.closest?.('[data-dock-zone]');
  for (const hot of document.querySelectorAll('.dock-drop-zone-hot')) {
    if (hot !== zone) hot.classList.remove('dock-drop-zone-hot');
  }
  zone?.classList.add('dock-drop-zone-hot');
}

function onDrop(event) {
  const zone = event.target.closest?.('[data-dock-zone]');
  const tab = event.target.closest?.('[data-dock-tab-button]');
  const bar = event.target.closest?.('.dock-tabbar');
  if (!dragging || !(zone || tab || bar)) return;
  event.preventDefault();
  event.stopPropagation();
  if (zone) {
    dockAction('move_tab', { tab: dragging, group: zone.dataset.dockGroup, zone: zone.dataset.dockZone });
  } else if (tab) {
    const rect = tab.getBoundingClientRect();
    dockAction('reorder_tab', {
      tab: dragging,
      target: tab.dataset.dockTab,
      after: dropAfter(event.clientX, rect) ? '1' : '0',
    });
  } else {
    dockAction('move_tab', { tab: dragging, group: groupOf(bar), zone: 'center' });
  }
  onDragEnd();
}

// Tab-pane handshakes: a page can rename its tab or redirect it once its content is loaded.
function onPaneSettled(pane) {
  const titled = pane.querySelector('[data-dock-set-title]');
  const path = pane.dataset.path;
  if (titled && path) {
    const tab = document.querySelector(`#dock [data-dock-tab-button][data-dock-path="${CSS.escape(path)}"]`);
    if (tab && tab.dataset.dockTitle !== titled.dataset.dockSetTitle) {
      dockAction('set_title', { path, title: titled.dataset.dockSetTitle });
    }
  }
  const redirect = pane.querySelector('[data-dock-redirect]');
  if (redirect) {
    dockAction('replace', { tab: pane.dataset.tabId, path: redirect.dataset.dockRedirect });
  }
}

// Loads the content of the panes that became visible for the first time (the kept ones already have it).
function loadActivePanes(dock) {
  for (const pane of dock.querySelectorAll('[data-dock-pane][data-src]')) {
    if (pane.dataset.loaded || pane.classList.contains('hidden')) continue;
    pane.dataset.loaded = 'true';
    htmx.ajax('GET', pane.dataset.src, { target: pane, swap: 'innerHTML' });
  }
}

function onDockSettled(dock) {
  attachPanes(dock);
  loadActivePanes(dock);
  writeLayout(dock.dataset.layout || '');
  const address = addressFor(dock.dataset.focusedPath);
  if (address && address !== location.pathname) history.replaceState(null, '', address);
  const active = dock.querySelector('[data-dock-tab-button][aria-selected="true"]');
  if (active) {
    requestAnimationFrame(() => active.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'nearest' }));
  }
}

function init() {
  document.addEventListener('click', onClick);
  document.addEventListener('auxclick', onAuxClick);
  document.addEventListener('pointerdown', onPointerDown);
  document.addEventListener('dragstart', onDragStart);
  document.addEventListener('dragend', onDragEnd);
  document.addEventListener('dragover', onDragOver);
  document.addEventListener('drop', onDrop);
  document.body.addEventListener('htmx:beforeSwap', (event) => {
    if (event.detail.target?.id === 'dock') keepPanes(event.detail.target);
  });
  document.body.addEventListener('htmx:afterSettle', (event) => {
    const target = event.target;
    if (!(target instanceof Element)) return;
    if (target.id === 'dock') onDockSettled(target);
    const pane = target.closest('[data-dock-pane]');
    if (pane && pane.childElementCount > 0) onPaneSettled(pane);
  });
  // Restore the saved layout; the root path opens nothing, any other path opens its tab.
  const path = location.pathname;
  restore(path === '/' ? '' : path);
}

if (typeof document !== 'undefined' && typeof htmx !== 'undefined') {
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
}
