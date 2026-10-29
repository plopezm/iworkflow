// Small UI behaviour shared by the components: dropdown menus, collapsible panels and the sidebar,
// modal dialogs, the theme selector, auto-dismissing toasts and lazily loaded panes.
// Each behaviour is driven by a data attribute documented in internal/core/components.
// Replaces the signal-based state of src/core/components/*.rs.

export const THEME_STORAGE_KEY = 'pi-theme';
export const SIDEBAR_STORAGE_KEY = 'iwf-sidebar-open';
export const BOARD_TAB_STORAGE_KEY = 'iwf-board-tab';
export const TOAST_DISMISS_MS = 8000;

// Switches the active sub-tab (loose / graph) on narrow screens.
export function switchBoardTab(button) {
  const tab = button.dataset.boardTab;
  if (!tab) return;
  const tabs = button.closest('.board-tabs');
  if (!tabs) return;
  const parent = tabs.closest('.board-list') || tabs.parentElement;
  const panes = parent?.querySelector('.board-panes');
  if (!panes) return;

  panes.setAttribute('data-tab', tab);
  for (const b of tabs.querySelectorAll('[data-board-tab]')) {
    const active = b === button;
    b.setAttribute('aria-selected', String(active));
    b.classList.toggle('segment-selected', active);
  }

  try {
    globalThis.sessionStorage?.setItem(BOARD_TAB_STORAGE_KEY, tab);
  } catch {}

  if (tab === 'graph') {
    const canvas = panes.querySelector('[data-graph-canvas]');
    if (canvas && typeof canvas._fitView === 'function') {
      if (typeof globalThis.requestAnimationFrame === 'function') {
        globalThis.requestAnimationFrame(() => canvas._fitView());
      } else {
        canvas._fitView();
      }
    }
  }
}

// Restores the previously selected board tab on narrow screens.
export function restoreBoardTab(root = document) {
  let saved = null;
  try {
    saved = globalThis.sessionStorage?.getItem(BOARD_TAB_STORAGE_KEY);
  } catch {}
  if (!saved) return;
  if (typeof globalThis.innerWidth === 'number' && globalThis.innerWidth > 767) return;

  for (const tabs of root.querySelectorAll('.board-tabs')) {
    const targetBtn = tabs.querySelector(`[data-board-tab="${saved}"]`);
    if (targetBtn) {
      switchBoardTab(targetBtn);
    }
  }
}

// Resolves a stored theme preference to the theme to apply: "light" or "dark".
export function resolveTheme(stored, systemPrefersDark) {
  if (stored === 'light' || stored === 'dark') return stored;
  return systemPrefersDark ? 'dark' : 'light';
}

// The sidebar is open unless it was explicitly stored closed ("0").
export function sidebarIsOpen(stored) {
  return stored !== '0';
}

// The label of the sidebar toggle: it offers the action the click will do.
// It is undefined when the toggle carries no translated label, and then the caller leaves the attributes alone.
export function sidebarToggleLabel(dataset, collapsed) {
  const label = collapsed ? dataset.labelShow : dataset.labelHide;
  return label || undefined;
}

// Only the first call of a toast arms the timer; a toast that already has one is left alone.
export function shouldArmToast(toast) {
  return toast.dataset.toastArmed === undefined;
}

// Whether a keep-alive pane should fetch its content now: it has a src (an empty one means the content
// is rendered inline), it was never fetched, it is not hidden, and it is not a dock pane.
export function shouldLoadPane(pane) {
  return Boolean(pane.dataset.src) && pane.dataset.loaded === undefined && !pane.classList.contains('hidden') && pane.dataset.dockPane === undefined;
}

// ----- browser wiring -------------------------------------------------------------------------

function storage() {
  try {
    return globalThis.localStorage;
  } catch {
    return null;
  }
}

function prefersDark() {
  return globalThis.matchMedia?.('(prefers-color-scheme: dark)').matches ?? false;
}

function closeMenus(except = null) {
  for (const panel of document.querySelectorAll('[data-menu-panel]')) {
    if (panel === except) continue;
    panel.style.display = 'none';
    panel.parentElement.querySelector('[data-menu-trigger]')?.setAttribute('aria-expanded', 'false');
  }
}

function toggleMenu(trigger) {
  const panel = trigger.closest('[data-menu]')?.querySelector('[data-menu-panel]');
  if (!panel) return;
  const open = panel.style.display === 'none';
  closeMenus(panel);
  panel.style.display = open ? '' : 'none';
  trigger.setAttribute('aria-expanded', String(open));
}

// Shows or hides the sidebar on desktop; the rail that replaces it when hidden follows.
function setCollapsed(root, collapsed) {
  if (collapsed) root.setAttribute('data-collapsed', '');
  else root.removeAttribute('data-collapsed');
  for (const toggle of root.querySelectorAll('[data-collapse-toggle]')) {
    toggle.setAttribute('aria-expanded', String(!collapsed));
    toggle.setAttribute('aria-label', collapsed ? toggle.dataset.labelClosed : toggle.dataset.labelOpen);
    const span = toggle.querySelector('span');
    if (span && toggle.dataset.glyphClosed && toggle.dataset.glyphOpen) {
      span.textContent = collapsed ? toggle.dataset.glyphClosed : toggle.dataset.glyphOpen;
    }
  }
  for (const toggle of document.querySelectorAll('[data-sidebar-toggle]')) {
    const label = sidebarToggleLabel(toggle.dataset, collapsed);
    if (label === undefined) continue;
    toggle.setAttribute('aria-label', label);
    toggle.setAttribute('title', label);
  }
  const rail = document.querySelector('[data-sidebar-rail]');
  if (rail) rail.style.display = collapsed ? '' : 'none';
  const key = root.dataset.persistKey;
  if (key && storage()) storage().setItem(key, collapsed ? '0' : '1');
}

function restoreSidebars(root = document) {
  const elements = [];
  if (root.matches?.('[data-collapse-root][data-persist-key]')) elements.push(root);
  elements.push(...(root.querySelectorAll?.('[data-collapse-root][data-persist-key]') ?? []));
  for (const el of elements) {
    const open = sidebarIsOpen(storage()?.getItem(el.dataset.persistKey) ?? null);
    setCollapsed(el, !open);
  }
}

function toggleCollapsibleSection(toggle) {
  const section = toggle.closest('[data-section]');
  const body = section?.querySelector('[data-section-body]');
  if (!body) return;
  const open = body.style.display === 'none';
  body.style.display = open ? '' : 'none';
  toggle.setAttribute('aria-expanded', String(open));
}

function toggleCollapsiblePanel(toggle) {
  const panel = toggle.closest('[data-collapsible]');
  const body = panel?.querySelector('[data-collapsible-body]');
  if (!panel || !body) return;
  const collapsed = !panel.classList.contains('side-panel-collapsed');
  panel.classList.toggle('side-panel-collapsed', collapsed);
  body.hidden = collapsed;
  toggle.setAttribute('aria-expanded', String(!collapsed));
  toggle.querySelector('span').textContent = collapsed ? '▸' : '▾';
}

// The choice is stored as light, dark or system (the Rust version stored the same values).
function applyTheme(choice) {
  storage()?.setItem(THEME_STORAGE_KEY, choice);
  document.documentElement.setAttribute('data-theme', resolveTheme(choice, prefersDark()));
  for (const button of document.querySelectorAll('[data-theme-choice]')) {
    const selected = button.dataset.themeChoice === choice;
    button.setAttribute('aria-pressed', String(selected));
    button.classList.toggle('chip-selected', selected);
  }
}

function closeModal(modal) {
  modal.style.display = 'none';
}

// Toasts remove themselves after TOAST_DISMISS_MS; arming is idempotent.
function armToasts(root) {
  for (const toast of root.querySelectorAll('[data-toast]')) {
    if (!shouldArmToast(toast)) continue;
    toast.dataset.toastArmed = 'true';
    setTimeout(() => toast.remove(), TOAST_DISMISS_MS);
  }
}

// Fetches the content of a keep-alive pane the first time it is shown. Dock panes are loaded by
// dock.js, which knows about the layout they belong to.
function loadPanes(root) {
  for (const pane of root.querySelectorAll('[data-keep-alive][data-src]')) {
    if (!shouldLoadPane(pane)) continue;
    pane.dataset.loaded = 'true';
    htmx.ajax('GET', pane.dataset.src, { target: pane, swap: 'innerHTML' });
  }
}

function onClick(event) {
  const target = event.target;
  const trigger = target.closest?.('[data-menu-trigger]');
  if (trigger) {
    toggleMenu(trigger);
    return;
  }
  // A click outside every menu, or on an entry of one, closes the open menus.
  if (!target.closest?.('[data-menu]') || target.closest?.('[data-menu-panel]')) closeMenus();

  const collapse = target.closest?.('[data-collapse-toggle]')?.closest('[data-collapse-root]');
  if (collapse) return setCollapsed(collapse, !collapse.hasAttribute('data-collapsed'));

  const sidebarToggle = target.closest?.('[data-sidebar-toggle]');
  if (sidebarToggle) {
    const root = document.getElementById('sidebar-panel');
    if (root) setCollapsed(root, !root.hasAttribute('data-collapsed'));
    return;
  }
  const mobileOpen = target.closest?.('[data-mobile-open][data-target]');
  if (mobileOpen) {
    const root = document.getElementById(mobileOpen.dataset.target);
    root?.setAttribute('data-mobile-open', '');
    const overlay = root?.querySelector('[data-overlay-close]');
    if (overlay) overlay.hidden = false;
    return;
  }
  const overlayClose = target.closest?.('[data-overlay-close]');
  if (overlayClose) {
    const root = overlayClose.closest('[data-collapse-root]');
    root?.removeAttribute('data-mobile-open');
    const overlay = root?.querySelector('[data-overlay-close]');
    if (overlay) overlay.hidden = true;
    return;
  }

  const print = target.closest?.('[data-print-report]');
  if (print) {
    window.print();
    return;
  }

  const section = target.closest?.('[data-section-toggle]');
  if (section) return toggleCollapsibleSection(section);

  const panel = target.closest?.('[data-collapsible-toggle]');
  if (panel) return toggleCollapsiblePanel(panel);

  const choice = target.closest?.('[data-theme-choice]');
  if (choice) return applyTheme(choice.dataset.themeChoice);

  const boardTab = target.closest?.('[data-board-tab]');
  if (boardTab) {
    switchBoardTab(boardTab);
    return;
  }

  const addToGraph = target.closest?.('[hx-post*="/add-to-graph"]');
  if (addToGraph) {
    try {
      globalThis.sessionStorage?.setItem(BOARD_TAB_STORAGE_KEY, 'graph');
    } catch {}
  }

  const close = target.closest?.('[data-modal-close]');
  if (close) return closeModal(close.closest('[data-modal]'));
  if (target.matches?.('[data-modal]')) return closeModal(target);

  const dismiss = target.closest?.('[data-toast-dismiss]');
  if (dismiss) dismiss.closest('[data-toast]')?.remove();
}

function onKeyDown(event) {
  if (event.key !== 'Escape') return;
  closeMenus();
  for (const modal of document.querySelectorAll('[data-modal]')) {
    if (modal.style.display !== 'none') closeModal(modal);
  }
}

// Error fragments (4xx and 5xx) are swapped in, so the message the server rendered reaches the user.
// 204 means "nothing to swap". Without this htmx ignores error responses and the user sees nothing.
function configureHtmx() {
  if (typeof htmx === 'undefined') return;
  htmx.config.responseHandling = [
    { code: '204', swap: false },
    { code: '[23]..', swap: true },
    { code: '[45]..', swap: true, error: false },
  ];
}

function init() {
  configureHtmx();
  document.addEventListener('click', onClick);
  document.addEventListener('keydown', onKeyDown);
  document.body.addEventListener('htmx:afterSettle', (event) => {
    if (event.target instanceof Element) {
      armToasts(event.target);
      loadPanes(event.target);
      restoreSidebars(event.target);
      restoreBoardTab(event.target);
      // Moving or opening tabs closes the mobile sidebar overlay.
      if (event.target.id === 'dock') document.getElementById('sidebar-panel')?.removeAttribute('data-mobile-open');
    }
  });
  restoreSidebars();
  armToasts(document);
  loadPanes(document);
  restoreBoardTab(document);
}

if (typeof document !== 'undefined') {
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
}
