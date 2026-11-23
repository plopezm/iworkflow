// The workspace search of the sidebar is closed until its icon is pressed. The icon (data-search-toggle) and the
// wrapper that holds the input (data-search-panel) are client state: the icon flips the wrapper's hidden flag and the
// button's aria-expanded, and the keepClientState hook in ui.js keeps both through the sidebar's morph refreshes.
// The server renders the wrapper open while a query is active, and a query that is typed cannot close it.

const SEARCH_INPUT_ID = 'board-sidebar-filter';
const SEARCH_PANEL_SELECTOR = '[data-search-panel]';
const SEARCH_TOGGLE_SELECTOR = '[data-search-toggle]';

// toggleFor finds the icon that controls the given wrapper.
function toggleFor(panel, root) {
  const toggles = root.querySelectorAll ? root.querySelectorAll(SEARCH_TOGGLE_SELECTOR) : [];
  for (const toggle of toggles) {
    if (toggle.getAttribute('aria-controls') === panel.id) return toggle;
  }
  return null;
}

function setOpen(toggle, panel, open) {
  panel.hidden = !open;
  if (toggle) toggle.setAttribute('aria-expanded', String(open));
}

// toggleSidebarSearch is the click on the icon. Opening focuses the input. Closing is refused while the input holds a
// query, so the list it narrows never hides behind a closed search. It returns whether the search changed.
export function toggleSidebarSearch(toggle, root = document) {
  const panel = root.getElementById ? root.getElementById(toggle.getAttribute('aria-controls')) : null;
  if (!panel) return false;
  const input = panel.querySelector('input');
  const opening = panel.hidden;
  if (!opening && input && input.value !== '') return false;
  setOpen(toggle, panel, opening);
  if (opening && input) input.focus();
  return true;
}

// onSearchEscape closes the search when Escape is pressed in its empty input, and gives the focus back to the icon.
// Any other key, or an input that holds a query, is left alone.
export function onSearchEscape(event, root = document) {
  if (event.key !== 'Escape') return false;
  const input = event.target;
  if (!input || input.id !== SEARCH_INPUT_ID || input.value !== '') return false;
  const panel = input.closest ? input.closest(SEARCH_PANEL_SELECTOR) : null;
  if (!panel) return false;
  const toggle = toggleFor(panel, root);
  if (event.preventDefault) event.preventDefault();
  setOpen(toggle, panel, false);
  if (toggle && toggle.focus) toggle.focus();
  return true;
}

// setupSidebarSearch listens on the document, so the icon keeps working after the sidebar is morphed.
export function setupSidebarSearch(doc = document) {
  doc.addEventListener('click', (event) => {
    const toggle = event.target && event.target.closest ? event.target.closest(SEARCH_TOGGLE_SELECTOR) : null;
    if (toggle) toggleSidebarSearch(toggle, doc);
  });
  doc.addEventListener('keydown', (event) => onSearchEscape(event, doc));
}

if (typeof document !== 'undefined') setupSidebarSearch(document);
