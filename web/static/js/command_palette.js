// Command palette: Ctrl/Cmd+K opens a quick switcher over the links the sidebar already shows
// (navigation, workspaces and the tasks that need attention). It queries nothing: choosing an
// entry clicks the original link, so the dock opens it as a tab exactly as the sidebar does.

// Selectors of the sidebar markup this module reads. They are duplicated by hand from the templates:
// NAV_LINKS from internal/core/components/shell.templ (data-app-shell) and the sidebar navigation, ROW_SELECTORS
// from internal/modules/tasks/presentation/components/sidebar.templ (data-workspace-row, data-attention-task).
// Renaming one of those attributes requires changing the matching constant here.
const NAV_LINKS = '[data-app-shell] nav a[href]';
const ROW_SELECTORS = ['[data-workspace-row]', '[data-attention-task]'];
// The sidebar hint element; the server renders 'Ctrl K' and this module swaps in the Apple label.
const SHORTCUT_HINT = '[data-palette-kbd]';

function normalize(text) {
  return String(text).normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase().trim();
}

// Keeps the entries whose label contains the query (case and accents ignored); the ones that
// start with it come first. An empty query keeps everything in the given order.
export function filterEntries(entries, query) {
  const needle = normalize(query);
  if (needle === '') return entries.slice();
  const starting = [];
  const containing = [];
  for (const entry of entries) {
    const label = normalize(entry.label);
    if (label.startsWith(needle)) starting.push(entry);
    else if (label.includes(needle)) containing.push(entry);
  }
  return starting.concat(containing);
}

// An Enter that confirms an IME composition (CJK input, dead keys) is not a choice. Safari reports it as keyCode 229.
export function isComposing(ev) {
  return Boolean(ev.isComposing || ev.keyCode === 229);
}

// Apple platforms (macOS, iOS, iPadOS) read the Command key; everywhere else the hint says Ctrl.
export function isApplePlatform(nav) {
  const hint = (nav && nav.userAgentData && nav.userAgentData.platform) || (nav && nav.platform) || '';
  return /mac|iphone|ipad|ipod/i.test(String(hint));
}

export function paletteShortcutLabel(isApple) {
  return isApple ? '⌘K' : 'Ctrl K';
}

export function isPaletteShortcut(ev) {
  const pressed = String(ev.key).toLowerCase();
  return Boolean(ev.ctrlKey || ev.metaKey) && !ev.altKey && !ev.shiftKey && pressed === 'k';
}

// A terminal owns Ctrl+K (kill to end of line), so the palette stays out of its way.
export function isTerminalTarget(element) {
  return Boolean(element && typeof element.closest === 'function' && element.closest('.xterm, [data-terminal]'));
}

function labelOf(link) {
  const row = link.closest('[data-attention-task]');
  if (row && row.dataset.attentionTitle) return row.dataset.attentionTitle;
  const copy = link.cloneNode(true);
  copy.querySelectorAll('[aria-hidden="true"]').forEach((icon) => icon.remove());
  return copy.textContent.replace(/\s+/g, ' ').trim();
}

// Reads the entries from the current sidebar; the first link of a path wins.
export function collectEntries(root = document) {
  const links = Array.from(root.querySelectorAll(NAV_LINKS));
  for (const selector of ROW_SELECTORS) {
    root.querySelectorAll(selector).forEach((row) => {
      const link = row.querySelector('a[href]');
      if (link) links.push(link);
    });
  }
  const seen = new Set();
  const entries = [];
  for (const link of links) {
    const href = link.getAttribute('href');
    const label = labelOf(link);
    if (!label || seen.has(href)) continue;
    seen.add(href);
    entries.push({ href, label, link });
  }
  return entries;
}

export function setupCommandPalette(root = document, nav = globalThis.navigator) {
  const label = paletteShortcutLabel(isApplePlatform(nav));
  root.querySelectorAll(SHORTCUT_HINT).forEach((hint) => {
    hint.textContent = label;
  });
  const dialog = root.getElementById('command-palette');
  if (!dialog) return;
  const input = dialog.querySelector('[data-palette-input]');
  const list = dialog.querySelector('[data-palette-list]');
  const empty = dialog.querySelector('[data-palette-empty]');
  let shown = [];
  let active = 0;

  const choose = (entry) => {
    dialog.close();
    entry.link.click();
  };
  const paint = () => {
    list.replaceChildren();
    shown.forEach((entry, index) => {
      const item = root.createElement('li');
      item.setAttribute('role', 'option');
      item.setAttribute('aria-selected', String(index === active));
      item.className = 'palette-item';
      item.textContent = entry.label;
      item.addEventListener('click', () => choose(entry));
      list.append(item);
    });
    empty.hidden = shown.length > 0;
    if (list.children[active]) list.children[active].scrollIntoView({ block: 'nearest' });
  };
  const refresh = () => {
    shown = filterEntries(collectEntries(root), input.value);
    active = 0;
    paint();
  };

  input.addEventListener('input', refresh);
  dialog.addEventListener('keydown', (ev) => {
    if (ev.key === 'ArrowDown' || ev.key === 'ArrowUp') {
      ev.preventDefault();
      if (shown.length === 0) return;
      active = (active + (ev.key === 'ArrowDown' ? 1 : shown.length - 1)) % shown.length;
      paint();
    } else if (ev.key === 'Enter' && !isComposing(ev) && shown[active]) {
      ev.preventDefault();
      choose(shown[active]);
    }
  });
  // A click on the backdrop (the dialog itself) closes it.
  dialog.addEventListener('click', (ev) => {
    if (ev.target === dialog) dialog.close();
  });
  const open = () => {
    input.value = '';
    refresh();
    dialog.showModal();
    input.focus();
  };
  root.addEventListener('keydown', (ev) => {
    if (!isPaletteShortcut(ev) || isTerminalTarget(ev.target)) return;
    ev.preventDefault();
    if (dialog.open) dialog.close();
    else open();
  });
  // The visible search field of the sidebar opens it too, for touch screens and for discovery.
  root.addEventListener('click', (ev) => {
    const trigger = ev.target instanceof Element ? ev.target.closest('[data-palette-open]') : null;
    if (trigger && !dialog.open) open();
  });
}

if (typeof document !== 'undefined') setupCommandPalette(document);
