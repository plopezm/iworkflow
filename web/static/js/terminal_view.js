// Mounts xterm.js on every session element of the page (the dock panes and the htmx swaps included).
// The element carries the session id; the connection, the scrollback replay and the reconnection belong
// to web/static/terminal.js, which this module drives through window.iwfTerminal.

const ASSETS = [
  '/static/vendor/xterm/xterm.js',
  '/static/vendor/xterm/addon-fit.js',
  '/static/terminal.js',
];
const STYLESHEET = '/static/vendor/xterm/xterm.css';
const SESSION_SELECTOR = '[data-terminal-session]';
const KEY_SELECTOR = '[data-terminal-key]';

// Assets load once, in order: xterm first, then the addon that uses it, then the glue.
let assets = null;
// Element -> handle returned by window.iwfTerminal.open. A null handle reserves the element while the
// assets load, so it is not mounted twice.
const handles = new Map();

function loadScript(src) {
  return new Promise((resolve, reject) => {
    const script = document.createElement('script');
    script.src = src;
    script.onload = resolve;
    script.onerror = () => reject(new Error(`could not load ${src}`));
    document.head.append(script);
  });
}

function loadAssets() {
  if (assets) return assets;
  if (!document.querySelector(`link[href="${STYLESHEET}"]`)) {
    const link = document.createElement('link');
    link.rel = 'stylesheet';
    link.href = STYLESHEET;
    document.head.append(link);
  }
  assets = ASSETS.reduce((chain, src) => chain.then(() => loadScript(src)), Promise.resolve());
  // A failed load is retried by the next mount instead of staying broken for the whole page.
  assets.catch(() => {
    assets = null;
  });
  return assets;
}

function mount(element) {
  if (handles.has(element)) return;
  handles.set(element, null);
  loadAssets()
    .then(() => {
      // The element may have been discarded while the assets loaded.
      if (!handles.has(element)) return;
      handles.set(element, window.iwfTerminal.open(element, element.dataset.terminalSession));
    })
    .catch((error) => {
      handles.delete(element);
      console.error(error);
    });
}

function dispose(element) {
  const handle = handles.get(element);
  handles.delete(element);
  if (handle) window.iwfTerminal.close(handle);
}

// Mounts the sessions inside root, root included.
function mountAll(root) {
  if (!root || typeof root.querySelectorAll !== 'function') return;
  for (const element of root.querySelectorAll(SESSION_SELECTOR)) mount(element);
  if (root.matches?.(SESSION_SELECTOR)) mount(root);
}

// A session whose element left the page is closed. The dock moves the panes it keeps and removes the
// panes of closed tabs in the same task, so the check runs after the DOM settles and sees a moved pane
// as connected.
function disposeDisconnected() {
  for (const element of [...handles.keys()]) {
    if (!element.isConnected) dispose(element);
  }
}

// Clicks on a special key send it to the session of the same terminal view. The key bar must not take
// the focus from the terminal, so the press is cancelled before the browser moves it.
document.addEventListener('mousedown', (event) => {
  if (event.target.closest?.(KEY_SELECTOR)) event.preventDefault();
});

document.addEventListener('click', (event) => {
  const button = event.target.closest?.(KEY_SELECTOR);
  if (!button) return;
  const element = button.closest('[data-terminal-view]')?.querySelector(SESSION_SELECTOR);
  const handle = element ? handles.get(element) : null;
  if (handle) window.iwfTerminal.sendKey(handle, button.dataset.terminalKey);
});

// The module runs once per page load: the first import mounts what is already on the page, and every
// htmx swap mounts what it brings in.
document.body.addEventListener('htmx:afterSettle', (event) => mountAll(event.target));
new MutationObserver((records) => {
  if (records.some((record) => record.removedNodes.length > 0)) disposeDisconnected();
}).observe(document.body, { childList: true, subtree: true });
mountAll(document);
