// Progressive enhancement of `<select data-searchable>` (the branch selects): a text input and a filtered dropdown are
// placed in front of the real select. The select stays in the form, visually hidden, so it is still submitted, is
// targeted by `hx-include="find select"` and keeps its value; a pick sets the select and fires a bubbling `change`.
// Without this script the select is a plain select. The enhancement is idempotent: it runs on load and after every
// htmx swap, and rebuilds the combobox only when a morph or swap removed it.

const SELECTOR = 'select[data-searchable]';
const WRAPPER_ATTR = 'data-combobox';
const EMPTY_ATTR = 'data-combobox-empty';
const ACTIVE_CLASS = 'bg-surface-3';
const PANEL_CLASS = 'absolute top-full left-0 right-0 z-40 mt-1 max-h-60 overflow-y-auto rounded-md border border-border bg-surface p-1 shadow-lg';
const EMPTY_CLASS = 'px-2.5 py-1.5 text-xs text-muted';

// nextActive moves the highlighted option by step (1 or -1) over count options and wraps at both ends. Without a
// highlight, the first step down highlights the first option and the first step up the last one.
export function nextActive(current, step, count) {
  if (count <= 0) return -1;
  if (current < 0 || current >= count) return step > 0 ? 0 : count - 1;
  return (current + step + count) % count;
}

// filterOptions keeps the entries whose label or value contains the query, ignoring case and surrounding spaces.
// An empty query keeps all of them. Each entry keeps its index in the select, which is what a pick sets.
export function filterOptions(entries, query) {
  const needle = String(query ?? '').trim().toLocaleLowerCase();
  if (needle === '') return entries.slice();
  return entries.filter((entry) =>
    String(entry.label).toLocaleLowerCase().includes(needle) || String(entry.value).toLocaleLowerCase().includes(needle));
}

function entriesOf(select) {
  return Array.from(select.options, (option, index) => ({ index, value: option.value, label: option.textContent.trim() }));
}

function selectedLabel(select) {
  const option = select.options[select.selectedIndex];
  return option ? option.textContent.trim() : '';
}

// The combobox state of one select. The select is the source of truth: the input only shows its label while closed.
const instances = new WeakMap();
// Selects that already carry the change listener of their combobox (a WeakSet, because a morph may strip attributes).
const changeBound = new WeakSet();
let listSeq = 0;

function build(select, doc) {
  listSeq += 1;
  const listId = `combobox-list-${listSeq}`;

  const wrapper = doc.createElement('div');
  wrapper.className = 'relative';
  wrapper.setAttribute(WRAPPER_ATTR, '');

  const input = doc.createElement('input');
  input.type = 'text';
  input.className = 'input';
  input.autocomplete = 'off';
  input.spellcheck = false;
  input.setAttribute('role', 'combobox');
  input.setAttribute('aria-autocomplete', 'list');
  input.setAttribute('aria-expanded', 'false');
  input.setAttribute('aria-controls', listId);
  input.placeholder = select.getAttribute('data-search-placeholder') ?? '';

  const panel = doc.createElement('div');
  panel.className = PANEL_CLASS;
  panel.hidden = true;

  const list = doc.createElement('div');
  list.id = listId;
  list.setAttribute('role', 'listbox');

  const empty = doc.createElement('div');
  empty.className = EMPTY_CLASS;
  empty.setAttribute(EMPTY_ATTR, '');
  empty.textContent = select.getAttribute('data-search-empty') ?? '';
  empty.hidden = true;

  panel.append(list, empty);
  wrapper.append(input, panel);

  select.parentNode.insertBefore(wrapper, select);
  select.classList.add('sr-only');
  select.setAttribute('aria-hidden', 'true');
  select.tabIndex = -1;

  const state = { select, wrapper, input, panel, list, empty, listId, open: false, query: '', active: -1, shown: [] };
  instances.set(select, state);

  input.value = selectedLabel(select);
  input.addEventListener('focus', () => { state.query = ''; input.select(); });
  input.addEventListener('click', () => { if (!state.open) openList(state, true); });
  input.addEventListener('input', () => {
    state.query = input.value;
    openList(state, false);
  });
  input.addEventListener('blur', () => closeList(state));
  input.addEventListener('keydown', (event) => onKeydown(state, event));
  panel.addEventListener('mousedown', (event) => event.preventDefault());
  panel.addEventListener('click', (event) => {
    const option = event.target && event.target.closest ? event.target.closest('[role="option"]') : null;
    if (option) pick(state, state.shown[Number(option.dataset.shown)]);
  });
  // One change listener per select, however often a morph rebuilds the combobox around it: the listener reads the
  // current state, so a rebuilt combobox needs no second one.
  if (!changeBound.has(select)) {
    changeBound.add(select);
    select.addEventListener('change', () => {
      const current = instances.get(select);
      if (current && !current.open) current.input.value = selectedLabel(select);
    });
  }
  return state;
}

function onKeydown(state, event) {
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault();
    if (!state.open) return openList(state, true);
    state.active = nextActive(state.active, event.key === 'ArrowDown' ? 1 : -1, state.shown.length);
    return refreshActive(state);
  }
  if (event.key === 'Enter') {
    // While the list is open, Enter answers it and never submits the form with a half-typed filter.
    if (!state.open) return;
    event.preventDefault();
    if (state.active >= 0) pick(state, state.shown[state.active]);
    return;
  }
  if (event.key === 'Escape' && state.open) {
    event.preventDefault();
    closeList(state);
  }
}

// openList shows the dropdown. resetActive highlights the chosen option (opening without typing); typing highlights the
// first match, so Enter picks what the filter shows.
function openList(state, resetActive) {
  state.open = true;
  state.panel.hidden = false;
  state.input.setAttribute('aria-expanded', 'true');
  renderOptions(state, resetActive);
}

function closeList(state) {
  state.open = false;
  state.query = '';
  state.panel.hidden = true;
  state.input.setAttribute('aria-expanded', 'false');
  state.input.removeAttribute('aria-activedescendant');
  state.input.value = selectedLabel(state.select);
}

function renderOptions(state, resetActive) {
  const { select, list, empty } = state;
  state.shown = filterOptions(entriesOf(select), state.query);
  const chosen = state.shown.findIndex((entry) => entry.index === select.selectedIndex);
  state.active = resetActive ? chosen : (state.shown.length > 0 ? 0 : -1);
  const items = state.shown.map((entry, position) => {
    const item = list.ownerDocument.createElement('div');
    item.className = 'menu-item';
    item.id = `${state.listId}-${position}`;
    item.setAttribute('role', 'option');
    item.dataset.shown = String(position);
    item.textContent = entry.label;
    return item;
  });
  list.replaceChildren(...items);
  empty.hidden = state.shown.length > 0;
  refreshActive(state);
}

// refreshActive highlights the option at state.active and points aria-activedescendant at it.
function refreshActive(state) {
  const { list, input } = state;
  Array.from(list.children).forEach((item, position) => {
    const active = position === state.active;
    item.classList.toggle(ACTIVE_CLASS, active);
    item.setAttribute('aria-selected', String(active));
  });
  const current = list.children[state.active];
  if (current) {
    input.setAttribute('aria-activedescendant', current.id);
    if (current.scrollIntoView) current.scrollIntoView({ block: 'nearest' });
  } else {
    input.removeAttribute('aria-activedescendant');
  }
}

function pick(state, entry) {
  if (!entry) return;
  state.select.selectedIndex = entry.index;
  state.select.dispatchEvent(new Event('change', { bubbles: true }));
  closeList(state);
}

// enhance upgrades one select, or refreshes the input of a combobox that is already in place. It returns the state, or
// null when the element is not a searchable select.
export function enhance(select, doc = document) {
  if (!select || !select.matches || !select.matches(SELECTOR)) return null;
  const existing = instances.get(select);
  if (existing && select.previousElementSibling === existing.wrapper) {
    if (!existing.open) existing.input.value = selectedLabel(select);
    return existing;
  }
  // A morph or swap removed the combobox but kept the select (or replaced it): drop the leftover and build again.
  const stale = select.previousElementSibling;
  if (stale && stale.hasAttribute && stale.hasAttribute(WRAPPER_ATTR)) stale.remove();
  return build(select, doc);
}

// enhanceAll upgrades the select under root, and root itself when it is one.
export function enhanceAll(root = document) {
  const found = root.matches && root.matches(SELECTOR) ? [root] : [];
  if (root.querySelectorAll) found.push(...root.querySelectorAll(SELECTOR));
  return found.map((select) => enhance(select, root.ownerDocument ?? document)).filter(Boolean);
}

// setupSearchableSelects enhances the page now and again after every htmx swap or settle, so a fragment that replaces
// a select (the lazy branch list, a workspace change, a morph of the edit page) gets its combobox too.
export function setupSearchableSelects(doc = document) {
  enhanceAll(doc);
  const onSwap = (event) => enhanceAll(event.target && event.target.querySelectorAll ? event.target : doc);
  // afterSwap covers a plain swap; afterSettle covers a morph, whose new nodes only settle later. enhance is
  // idempotent, so the second pass over the same select is a no-op.
  doc.addEventListener('htmx:afterSettle', onSwap);
  doc.addEventListener('htmx:afterSwap', onSwap);
}

if (typeof document !== 'undefined') {
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', () => setupSearchableSelects(document), { once: true });
  else setupSearchableSelects(document);
}
