// Syntax highlighting for the code viewer and editor. highlight.js and code-view.js are loaded on
// first use. Port of the inline JS of src/core/components/code.rs.
//
// Markup contract (internal/core/components/code.templ):
//   [data-code-text]    the element whose text is highlighted; its content is data-code-text
//                       and data-code-filename names the language
//   [data-code-editor]  a textarea, with [data-code-layer] (highlighted copy) and [data-code-gutter]
//   Ctrl/Cmd+S         submits the form the editor belongs to

const ASSETS = ['/static/vendor/highlightjs/highlight.min.js', '/static/code-view.js'];

let loading = null;

function loadScript(src) {
  return new Promise((resolve, reject) => {
    const script = document.createElement('script');
    script.dataset.iwfSrc = src;
    script.src = src;
    script.onload = resolve;
    script.onerror = () => reject(new Error(`no se pudo cargar ${src}`));
    document.head.appendChild(script);
  });
}

// Loads the highlighter once; a failed load can be retried.
export function ready() {
  if (globalThis.iwfCode) return Promise.resolve();
  if (!loading) {
    loading = (async () => {
      for (const src of ASSETS) await loadScript(src);
    })().catch((error) => {
      loading = null;
      throw error;
    });
  }
  return loading;
}

// Renders text into el. Without the highlighter the plain text is shown.
export function renderCode(el, text, filename) {
  const draw = () => globalThis.iwfCode.render(el, text, filename);
  if (globalThis.iwfCode) return draw();
  ready()
    .then(draw)
    .catch((error) => {
      console.error(error);
      el.textContent = text;
    });
  return undefined;
}

// Connects a textarea to its highlighted layer and gutter. Returns a handle to detach it.
export function attachEditor(textarea, layer, gutter) {
  const handle = { detach: null, closed: false };
  ready()
    .then(() => {
      if (!handle.closed) handle.detach = globalThis.iwfCode.attachEditor(textarea, layer, gutter);
    })
    .catch((error) => console.error(error));
  return handle;
}

export function detachEditor(handle) {
  handle.closed = true;
  if (handle.detach) {
    handle.detach();
    handle.detach = null;
  }
}

// Ctrl/Cmd+S on an editor submits its form (htmx intercepts the submit).
export function isSaveShortcut(event) {
  return (event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's';
}

function init(root) {
  for (const el of root.querySelectorAll('[data-code-text]')) {
    if (el.dataset.codeRendered) continue;
    el.dataset.codeRendered = 'true';
    renderCode(el, el.dataset.codeText, el.dataset.codeFilename || '');
  }
  for (const textarea of root.querySelectorAll('textarea[data-code-editor]')) {
    if (textarea.dataset.codeAttached) continue;
    textarea.dataset.codeAttached = 'true';
    const layer = textarea.parentElement.querySelector('[data-code-layer] code');
    const gutter = textarea.closest('.code-box')?.querySelector('[data-code-gutter]');
    const handle = attachEditor(textarea, layer?.parentElement, gutter);
    textarea.addEventListener('keydown', (event) => {
      if (!isSaveShortcut(event)) return;
      event.preventDefault();
      textarea.form?.requestSubmit();
    });
    textarea.addEventListener('htmx:beforeCleanupElement', () => detachEditor(handle), { once: true });
  }
}

if (typeof document !== 'undefined') {
  const start = () => {
    init(document);
    document.body.addEventListener('htmx:afterSettle', (event) => {
      if (event.target instanceof Element) init(event.target);
    });
  };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start);
  else start();
}
