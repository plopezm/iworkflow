// Code | Rendered switch of the file editors of Markdown files.
//
// Markup contract (internal/core/components/markdown_switch.templ):
//   form[data-md-form]       the editor form; the value is the preview endpoint. It holds a textarea named "content"
//   [data-md-mode-set]       a switch button; the value is "code" or "rendered"
//   [data-md-preview]        where the rendered view goes
//   [data-md-hide]           hidden while the rendered view is shown
// The chosen mode is kept in the form (data-md-mode) and remembered for the editors that open later.

const STORAGE_KEY = 'iwf-md-mode';
const CODE = 'code';
const RENDERED = 'rendered';

export function normalizeMode(value) {
  return value === RENDERED ? RENDERED : CODE;
}

function storedMode() {
  try {
    return normalizeMode(globalThis.localStorage?.getItem(STORAGE_KEY));
  } catch {
    return CODE;
  }
}

function remember(mode) {
  try {
    globalThis.localStorage?.setItem(STORAGE_KEY, mode);
  } catch {
    // Private mode: the choice just does not outlive the page.
  }
}

// Fills the preview with the current text of the editor. A newer request wins over an older one.
export async function refreshPreview(form) {
  const preview = form.querySelector('[data-md-preview]');
  const field = form.querySelector('textarea[name="content"]');
  if (!preview || !field) return;
  const ticket = (form.mdTicket = (form.mdTicket || 0) + 1);
  try {
    const response = await fetch(form.dataset.mdForm, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded', 'HX-Request': 'true' },
      body: new URLSearchParams({ content: field.value }),
    });
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const html = await response.text();
    if (ticket === form.mdTicket) preview.innerHTML = html;
  } catch (error) {
    console.error(error);
  }
}

export function setMode(form, mode) {
  const next = normalizeMode(mode);
  form.dataset.mdMode = next;
  form.querySelectorAll('[data-md-mode-set]').forEach((button) => {
    button.setAttribute('aria-pressed', String(button.dataset.mdModeSet === next));
  });
  if (next === RENDERED) refreshPreview(form);
}

export function initMarkdownViews(root = document) {
  root.querySelectorAll('form[data-md-form]').forEach((form) => {
    if (form.dataset.mdMode) return;
    setMode(form, storedMode());
  });
}

export function setupMarkdownViews(doc = document) {
  doc.addEventListener('click', (event) => {
    const button = event.target.closest?.('[data-md-mode-set]');
    const form = button?.closest('form[data-md-form]');
    if (!form) return;
    const mode = normalizeMode(button.dataset.mdModeSet);
    remember(mode);
    setMode(form, mode);
  });
  doc.addEventListener('htmx:afterSettle', (event) => initMarkdownViews(event.target?.querySelectorAll ? event.target : doc));
  initMarkdownViews(doc);
}

if (typeof document !== 'undefined') {
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', () => setupMarkdownViews());
  else setupMarkdownViews();
}
