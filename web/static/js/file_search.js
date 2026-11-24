// Search inside the open file (Ctrl+F) and the handoff of the tree's content filter.
//
// Pure helpers (tested in file_search_test.mjs) and the DOM glue of the file page:
//   [data-file-search-root] data-scope, data-path: holds the bar and the editor (outside the editor form,
//                           which htmx swaps)
//   [data-file-search]      the bar, hidden until opened: [data-fs-input], [data-fs-count], [data-fs-prev],
//                           [data-fs-next], [data-fs-close] and the checkboxes re/cs/ww
//   textarea[data-code-editor]  the text searched; [data-code-layer] code is its highlighted copy
//   form[data-content-filter], a[data-hit-file], a[data-hit-line]  the tree side of the handoff
//
// The tree saves its query and the jump in sessionStorage and announces the filter with the
// `iwf:content-filter` event. The viewer reads them; its own query never writes storage nor changes the tree.

const FILTER_PREFIX = 'iwf-content-search:';
const JUMP_PREFIX = 'iwf-content-jump:';
const HL_ALL = 'iwf-search';
const HL_CURRENT = 'iwf-search-current';
const FILTER_EVENT = 'iwf:content-filter';
const ROOT_SELECTOR = '[data-file-search-root]';
const MAX_MATCHES = 5000;
const LINE_REM = 1.25;

function escapeRegex(text) {
  return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

// buildRegex compiles the query: literal unless flags.re, case-insensitive unless flags.cs, whole words when
// flags.ww. Lines are anchored with the m flag. It returns null for an empty query or an invalid regex.
export function buildRegex(query, flags) {
  if (!query) return null;
  const source = flags.re ? query : escapeRegex(query);
  const wrapped = flags.ww ? `\\b(?:${source})\\b` : source;
  try {
    return new RegExp(wrapped, `gm${flags.cs ? '' : 'i'}`);
  } catch {
    return null;
  }
}

// findMatches returns the non-empty matches as {start, end} offsets over the whole text, at most limit of them.
export function findMatches(text, re, limit = MAX_MATCHES) {
  if (!re) return [];
  const scan = new RegExp(re.source, re.flags.includes('g') ? re.flags : `${re.flags}g`);
  const found = [];
  let m = scan.exec(text);
  while (m !== null && found.length < limit) {
    if (m[0].length === 0) {
      scan.lastIndex++;
    } else {
      found.push({ start: m.index, end: m.index + m[0].length });
    }
    m = scan.exec(text);
  }
  return found;
}

// lineOf is the 1-based line of a text offset.
export function lineOf(text, offset) {
  let line = 1;
  for (let i = 0; i < offset && i < text.length; i++) {
    if (text.charCodeAt(i) === 10) line++;
  }
  return line;
}

// stepIndex moves the current match by dir (1 or -1) and wraps around; -1 when there are no matches.
export function stepIndex(current, total, dir) {
  if (total <= 0) return -1;
  return (((current + dir) % total) + total) % total;
}

// counterText is the "n / total" counter; "0 / 0" when there are no matches.
export function counterText(index, total) {
  return `${total > 0 ? index + 1 : 0} / ${total}`;
}

function readJSON(store, key) {
  if (!store) return null;
  const raw = store.getItem(key);
  if (raw === null || raw === undefined) return null;
  try {
    return JSON.parse(raw);
  } catch {
    return null;
  }
}

// saveFilter stores the query of a scope; an empty query removes it.
export function saveFilter(store, scope, filter) {
  if (!store) return;
  const key = FILTER_PREFIX + scope;
  if (!filter.q) {
    store.removeItem(key);
    return;
  }
  store.setItem(key, JSON.stringify({ q: filter.q, re: !!filter.re, cs: !!filter.cs, ww: !!filter.ww }));
}

// loadFilter returns the stored query of a scope, or null when there is none or it is corrupt.
export function loadFilter(store, scope) {
  const data = readJSON(store, FILTER_PREFIX + scope);
  if (!data || typeof data.q !== 'string' || data.q === '') return null;
  return { q: data.q, re: data.re === true, cs: data.cs === true, ww: data.ww === true };
}

// saveJump stores the file and line the tree asks the viewer to show.
export function saveJump(store, scope, jump) {
  if (!store) return;
  store.setItem(JUMP_PREFIX + scope, JSON.stringify({ path: String(jump.path), line: Number(jump.line) || 0 }));
}

// takeJump returns the stored jump and consumes it, but only when it is for the given path.
export function takeJump(store, scope, path) {
  const key = JUMP_PREFIX + scope;
  const data = readJSON(store, key);
  if (!data || data.path !== path) return null;
  store.removeItem(key);
  return { path: data.path, line: Number(data.line) || 0 };
}

// pickCurrent is the index of the first match on the given line, else of the first match; -1 without matches.
export function pickCurrent(text, found, line) {
  if (found.length === 0) return -1;
  if (line > 0) {
    const index = found.findIndex((m) => lineOf(text, m.start) === line);
    if (index >= 0) return index;
  }
  return 0;
}

// scrollTopFor centres a line in the viewport; never below zero.
export function scrollTopFor({ line, lineHeight, padTop, viewport }) {
  const centre = (line - 1) * lineHeight + padTop + lineHeight / 2;
  return Math.max(0, Math.round(centre - viewport / 2));
}

// textRangeAt maps the text offsets [start, end) onto the text nodes (given by their lengths). It returns
// null when the offsets are outside the nodes.
export function textRangeAt(nodes, start, end) {
  let startNode = -1;
  let startOffset = 0;
  let endNode = -1;
  let endOffset = 0;
  let base = 0;
  for (let i = 0; i < nodes.length; i++) {
    const length = nodes[i].length;
    if (startNode < 0 && start >= base && start < base + length) {
      startNode = i;
      startOffset = start - base;
    }
    if (end > base && end <= base + length) {
      endNode = i;
      endOffset = end - base;
      break;
    }
    base += length;
  }
  if (startNode < 0 || endNode < 0) return null;
  return { startNode, startOffset, endNode, endOffset };
}

// ---- DOM glue (browser only) ----

const viewers = new WeakMap();

function storage() {
  try {
    return globalThis.sessionStorage || null;
  } catch {
    return null;
  }
}

function highlightRegistry() {
  return typeof CSS !== 'undefined' && CSS.highlights && typeof Highlight !== 'undefined' ? CSS.highlights : null;
}

function textNodes(node) {
  const nodes = [];
  const walker = document.createTreeWalker(node, NodeFilter.SHOW_TEXT);
  for (let n = walker.nextNode(); n; n = walker.nextNode()) nodes.push(n);
  return nodes;
}

// attachViewer wires the bar of one file page. It runs once per page root; a swapped page is a new root.
function attachViewer(root) {
  if (root.dataset.fsReady === 'true') return;
  const bar = root.querySelector('[data-file-search]');
  const input = root.querySelector('[data-fs-input]');
  const count = root.querySelector('[data-fs-count]');
  if (!bar || !input || !count) return;
  root.dataset.fsReady = 'true';

  const scope = root.dataset.scope || '';
  const path = root.dataset.path || '';
  let open = false;
  let matches = [];
  let current = -1;
  let frame = 0;
  let resetPending = false;

  const textarea = () => root.querySelector('textarea[data-code-editor]');
  const codeNode = () => root.querySelector('[data-code-layer] code');
  const flagBox = (name) => bar.querySelector(`input[name="${name}"]`);
  const readFlags = () => ({ re: flagBox('re').checked, cs: flagBox('cs').checked, ww: flagBox('ww').checked });

  function setCounter() {
    count.textContent = counterText(current, matches.length);
  }

  // paint draws the matches over the highlighted layer. Without the CSS Custom Highlight API it does nothing.
  function paint() {
    const registry = highlightRegistry();
    if (!registry) return;
    registry.delete(HL_ALL);
    registry.delete(HL_CURRENT);
    const ta = textarea();
    const code = codeNode();
    if (!open || !ta || !code || matches.length === 0) return;
    // The layer shows the text plus a trailing newline; until the highlighter renders it, nothing lines up.
    if (code.textContent !== `${ta.value}\n`) return;
    const nodes = textNodes(code);
    const all = new Highlight();
    const now = new Highlight();
    matches.forEach((m, i) => {
      const range = textRangeAt(nodes, m.start, m.end);
      if (!range) return;
      const dom = document.createRange();
      dom.setStart(nodes[range.startNode], range.startOffset);
      dom.setEnd(nodes[range.endNode], range.endOffset);
      (i === current ? now : all).add(dom);
    });
    registry.set(HL_ALL, all);
    registry.set(HL_CURRENT, now);
  }

  function refresh(reset) {
    const ta = textarea();
    matches = ta ? findMatches(ta.value, buildRegex(input.value, readFlags()), MAX_MATCHES) : [];
    if (reset || current < 0 || current >= matches.length) current = matches.length ? 0 : -1;
    setCounter();
    paint();
  }

  // schedule coalesces refreshes into one animation frame.
  function schedule(reset) {
    resetPending = resetPending || reset;
    if (frame) return;
    frame = requestAnimationFrame(() => {
      frame = 0;
      const pending = resetPending;
      resetPending = false;
      refresh(pending);
    });
  }

  function jumpTo(index) {
    const ta = textarea();
    const m = matches[index];
    if (!ta || !m) return;
    const style = getComputedStyle(ta);
    const rem = parseFloat(getComputedStyle(document.documentElement).fontSize) || 16;
    ta.scrollTop = scrollTopFor({
      line: lineOf(ta.value, m.start),
      lineHeight: parseFloat(style.lineHeight) || LINE_REM * rem,
      padTop: parseFloat(style.paddingTop) || 0,
      viewport: ta.clientHeight,
    });
  }

  function go(dir) {
    if (matches.length === 0) return;
    current = stepIndex(current, matches.length, dir);
    setCounter();
    paint();
    jumpTo(current);
  }

  function show() {
    open = true;
    bar.hidden = false;
  }

  // openBar is Ctrl+F: the selection of the editor, when it is one line, becomes the query.
  function openBar() {
    const ta = textarea();
    if (ta && ta.selectionStart !== ta.selectionEnd) {
      const picked = ta.value.slice(ta.selectionStart, ta.selectionEnd);
      if (!picked.includes('\n')) {
        input.value = picked;
        input.dataset.fsDirty = 'true';
      }
    }
    show();
    input.focus();
    input.select();
    refresh(true);
    if (current >= 0) jumpTo(current);
  }

  function closeBar() {
    open = false;
    bar.hidden = true;
    delete input.dataset.fsDirty;
    paint();
    textarea()?.focus();
  }

  // sync applies the tree's query to the bar, unless the user typed in the bar itself.
  function sync() {
    if (input.dataset.fsDirty === 'true') return;
    const filter = loadFilter(storage(), scope);
    input.value = filter ? filter.q : '';
    flagBox('re').checked = filter ? filter.re : false;
    flagBox('cs').checked = filter ? filter.cs : false;
    flagBox('ww').checked = filter ? filter.ww : false;
    schedule(true);
  }

  // handoff opens the bar with the tree's query and goes to the jump line, or to the first match.
  function handoff() {
    const jump = takeJump(storage(), scope, path);
    const filter = loadFilter(storage(), scope);
    if (!filter) return;
    input.value = filter.q;
    flagBox('re').checked = filter.re;
    flagBox('cs').checked = filter.cs;
    flagBox('ww').checked = filter.ww;
    show();
    refresh(true);
    const ta = textarea();
    current = pickCurrent(ta ? ta.value : '', matches, jump ? jump.line : 0);
    setCounter();
    paint();
    if (current >= 0) jumpTo(current);
  }

  root.addEventListener('input', (event) => {
    if (event.target === input) {
      input.dataset.fsDirty = 'true';
      schedule(true);
    } else if (open && event.target instanceof Element && event.target.matches('textarea[data-code-editor]')) {
      schedule(false);
    }
  });

  root.addEventListener('change', (event) => {
    // Only the toggles: the text input also fires `change` on blur, which must not reset the current match.
    if (open && event.target instanceof Element && bar.contains(event.target) && event.target.type === 'checkbox') {
      input.dataset.fsDirty = 'true';
      schedule(true);
    }
  });

  root.addEventListener('keydown', (event) => {
    if ((event.ctrlKey || event.metaKey) && !event.altKey && !event.shiftKey && event.key.toLowerCase() === 'f') {
      event.preventDefault();
      openBar();
      return;
    }
    if (!open) return;
    if (event.key === 'F3') {
      event.preventDefault();
      go(event.shiftKey ? -1 : 1);
    } else if (event.key === 'Escape') {
      event.preventDefault();
      closeBar();
    } else if (event.key === 'Enter' && event.target === input) {
      event.preventDefault();
      go(event.shiftKey ? -1 : 1);
    }
  });

  root.addEventListener('click', (event) => {
    if (!(event.target instanceof Element)) return;
    if (event.target.closest('[data-fs-prev]')) go(-1);
    else if (event.target.closest('[data-fs-next]')) go(1);
    else if (event.target.closest('[data-fs-close]')) closeBar();
  });

  // The highlighter renders asynchronously and replaces the layer nodes; the editor's save swaps its form.
  // Both change the DOM under the root, so the bar repaints. Mutations inside the bar itself are ignored.
  const observer = new MutationObserver((records) => {
    if (records.every((record) => bar.contains(record.target))) return;
    schedule(false);
  });
  observer.observe(root, { childList: true, subtree: true, characterData: true });

  root.addEventListener('htmx:beforeCleanupElement', (event) => {
    if (event.target !== root) return;
    observer.disconnect();
    if (frame) cancelAnimationFrame(frame);
    const registry = highlightRegistry();
    if (registry) {
      registry.delete(HL_ALL);
      registry.delete(HL_CURRENT);
    }
  });

  viewers.set(root, { sync });
  handoff();
}

// Tree side: the content filter saves its query and announces it; a hit link saves the jump to its line.
function onTreeEdit(event) {
  const form = event.target instanceof Element ? event.target.closest('form[data-content-filter]') : null;
  if (!form) return;
  const scope = form.querySelector('input[name="scope"]')?.value || '';
  const data = new FormData(form);
  saveFilter(storage(), scope, {
    q: String(data.get('q') || ''),
    re: data.has('re'),
    cs: data.has('cs'),
    ww: data.has('ww'),
  });
  document.dispatchEvent(new CustomEvent(FILTER_EVENT, { detail: { scope } }));
}

function onHitClick(event) {
  const link = event.target instanceof Element ? event.target.closest('a[data-hit-file], a[data-hit-line]') : null;
  if (!link) return;
  const holder = link.closest('[data-hit-path]');
  const scope = document.querySelector('form[data-content-filter] input[name="scope"]')?.value;
  if (!holder || scope === undefined) return;
  saveJump(storage(), scope, { path: holder.dataset.hitPath, line: Number(link.dataset.hitLine || 0) });
}

function initViewers(node) {
  if (node.matches?.(ROOT_SELECTOR)) attachViewer(node);
  for (const root of node.querySelectorAll?.(ROOT_SELECTOR) ?? []) attachViewer(root);
}

if (typeof document !== 'undefined') {
  const start = () => {
    initViewers(document);
    document.body.addEventListener('htmx:afterSettle', (event) => {
      if (event.target instanceof Element) initViewers(event.target);
    });
    document.addEventListener('input', onTreeEdit);
    document.addEventListener('change', onTreeEdit);
    document.addEventListener('click', onHitClick);
    document.addEventListener(FILTER_EVENT, () => {
      for (const root of document.querySelectorAll(ROOT_SELECTOR)) viewers.get(root)?.sync();
    });
  };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start);
  else start();
}
