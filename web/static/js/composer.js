// Behaviour of the message box of a task: the menus of /commands and @files, the references, Ctrl/Cmd+Enter to send
// and the state of the send button. Port of the Rust composer.rs and composer_completion.rs. The server decides what a
// message means (a /plan or /agent switches the mode there); this script only edits the text and shows the menus.
// Events are delegated from the document, so they keep working after htmx replaces the composer body.

const COMMAND = 'command';
const FILE = 'file';

/** Finds the completion trigger at the caret: a / at the start of the message, or an @ after whitespace. */
export function detectTrigger(text, caret) {
  const head = text.slice(0, caret);
  let start = 0;
  for (let i = head.length - 1; i >= 0; i -= 1) {
    if (/\s/.test(head[i])) {
      start = i + 1;
      break;
    }
  }
  const token = head.slice(start);
  if (token[0] === '@') return { kind: FILE, start, end: caret, query: token.slice(1) };
  if (token[0] === '/' && start === 0) return { kind: COMMAND, start, end: caret, query: token.slice(1) };
  return null;
}

/** Commands whose name starts with the query, ignoring case. */
export function matchingCommands(query, commands) {
  const needle = query.toLowerCase();
  return commands.filter((command) => command.label.slice(1).toLowerCase().startsWith(needle));
}

/** Replaces the trigger token with the insert text and a space. Returns the new text and the caret after the space. */
export function applySuggestion(text, trigger, insert) {
  const inserted = `${insert} `;
  return {
    text: `${text.slice(0, trigger.start)}${inserted}${text.slice(trigger.end)}`,
    caret: trigger.start + inserted.length,
  };
}

/** Distinct @path references of the text, in order of appearance, without the @. */
export function references(text) {
  const found = [];
  for (const token of text.split(/\s+/)) {
    if (!token.startsWith('@')) continue;
    const path = token.slice(1);
    if (path !== '' && !found.includes(path)) found.push(path);
  }
  return found;
}

/** Removes every @path reference to path, and the whitespace that followed it. */
export function removeReference(text, path) {
  const target = `@${path}`;
  const parts = text.split(/(\s+)/);
  let out = '';
  for (let i = 0; i < parts.length; i += 1) {
    if (i % 2 === 0 && parts[i] === target) {
      i += 1;
      continue;
    }
    out += parts[i];
  }
  return out.trimEnd();
}

// Per composer element: the trigger at the caret, the highlighted suggestion and whether Escape closed the menu.
const states = new WeakMap();

const stateOf = (composer) => states.get(composer) ?? { trigger: null, active: 0, dismissed: false };
const composerOf = (node) => node?.closest?.('[data-composer]') ?? null;
const textareaOf = (composer) => composer.querySelector('[data-composer-input]');
const menuOf = (composer) => composer.querySelector('[data-composer-menu]');
const sendOf = (composer) => composer.querySelector('[data-composer-send]');
const itemsOf = (composer) => Array.from(menuOf(composer).querySelectorAll('[data-composer-item]'));

function commandItem(offer) {
  const item = document.createElement('li');
  item.className = 'composer-menu-item';
  item.setAttribute('role', 'option');
  item.dataset.composerItem = '';
  item.dataset.insert = offer.insert;
  const label = document.createElement('span');
  label.className = 'truncate';
  label.textContent = offer.label;
  const hint = document.createElement('span');
  hint.className = 'composer-menu-hint truncate';
  hint.textContent = offer.hint;
  item.append(label, hint);
  return item;
}

function refChip(composer, path) {
  const chip = document.createElement('li');
  chip.className = 'composer-ref';
  const label = document.createElement('span');
  label.className = 'truncate';
  label.textContent = `@${path}`;
  const remove = document.createElement('button');
  remove.type = 'button';
  remove.className = 'composer-ref-remove';
  remove.dataset.composerRemoveRef = path;
  remove.setAttribute('aria-label', (composer.dataset.removeLabel ?? '').replace('{name}', path));
  remove.textContent = '×';
  chip.append(label, remove);
  return chip;
}

function setActive(composer, index) {
  const items = itemsOf(composer);
  items.forEach((item, position) => {
    const active = position === index;
    item.classList.toggle('is-active', active);
    item.setAttribute('aria-selected', String(active));
  });
  states.set(composer, { ...stateOf(composer), active: index });
}

function closeMenu(composer) {
  cancelFiles(composer);
  menuOf(composer).hidden = true;
}

function refreshRefs(composer) {
  const list = composer.querySelector('[data-composer-refs]');
  const paths = references(textareaOf(composer).value);
  list.replaceChildren(...paths.map((path) => refChip(composer, path)));
  list.hidden = paths.length === 0;
}

function refreshSend(composer) {
  const send = sendOf(composer);
  if (!send) return;
  const textarea = textareaOf(composer);
  const images = composer.querySelector('[data-image-input]')?.files.length ?? 0;
  send.disabled = (textarea?.value.trim() ?? '') === '' && images === 0;
}

function refreshAll(composer) {
  refreshRefs(composer);
  refreshSend(composer);
}

// The file request of each composer: the query it asks for and the controller that cancels it. See suggestFiles.
const fileRequests = new WeakMap();

const cancelFiles = (composer) => {
  fileRequests.get(composer)?.controller.abort();
  fileRequests.delete(composer);
};

/** Loads the files that match a query from the server of the task. */
async function fetchFiles(composer, query, signal) {
  const separator = (composer.dataset.filesPath || '').includes('?') ? '&' : '?';
  let url = `${composer.dataset.filesPath}${separator}q=${encodeURIComponent(query)}`;
  const workspaceSelect = composer.closest('form')?.querySelector('select[name="workspace_id"]');
  if (workspaceSelect?.value) {
    url += `&workspace_id=${encodeURIComponent(workspaceSelect.value)}`;
  }
  const response = await fetch(url, {
    headers: { 'HX-Request': 'true' },
    signal,
  });
  // A failed answer carries an error page, not file names: it takes the same path as a request that never arrived.
  if (!response.ok) throw new Error(`the file suggestions answered ${response.status}`);
  return response.text();
}

/**
 * Shows the files of a query in the menu. The query on screen is asked once, however many events report it, and an
 * answer is shown only while its query is still the one typed: a late answer for a query the user has left is dropped.
 */
export function suggestFiles(composer, query, load = fetchFiles) {
  if (fileRequests.get(composer)?.query === query) return undefined;
  cancelFiles(composer);
  const controller = new AbortController();
  const request = { query, controller };
  fileRequests.set(composer, request);
  return load(composer, query, controller.signal).then(
    (html) => {
      if (controller.signal.aborted || fileRequests.get(composer) !== request) return;
      const menu = menuOf(composer);
      menu.innerHTML = html;
      menu.hidden = false;
      setActive(composer, 0);
    },
    () => {
      if (!controller.signal.aborted) closeMenu(composer);
    },
  );
}

/** Opens the menu for the trigger at the caret: commands come from the list of the composer, files from the server. */
function updateMenu(composer) {
  const current = stateOf(composer);
  const textarea = textareaOf(composer);
  const trigger = current.dismissed ? null : detectTrigger(textarea.value, textarea.selectionStart ?? 0);
  states.set(composer, { ...current, trigger });
  if (!trigger) return closeMenu(composer);
  const menu = menuOf(composer);
  if (trigger.kind === COMMAND) {
    cancelFiles(composer);
    const commands = JSON.parse(composer.dataset.commands || '[]');
    const offers = matchingCommands(trigger.query, commands);
    if (offers.length === 0) return closeMenu(composer);
    menu.replaceChildren(...offers.map(commandItem));
    menu.hidden = false;
    return setActive(composer, 0);
  }
  menu.hidden = false;
  // The server lists the files of the task; the answer replaces the items and resets the highlight (see suggestFiles).
  return suggestFiles(composer, trigger.query);
}

function accept(composer, index) {
  const current = stateOf(composer);
  const item = itemsOf(composer)[index];
  if (!current.trigger || !item) return;
  const textarea = textareaOf(composer);
  const { text, caret } = applySuggestion(textarea.value, current.trigger, item.dataset.insert);
  textarea.value = text;
  states.set(composer, { ...current, trigger: null });
  closeMenu(composer);
  textarea.focus();
  textarea.setSelectionRange(caret, caret);
  refreshAll(composer);
}

function move(composer, step) {
  const count = itemsOf(composer).length;
  const { active } = stateOf(composer);
  setActive(composer, (active + step + count) % count);
}

function onKeydown(event) {
  const textarea = event.target;
  const composer = composerOf(textarea);
  if (!composer || event.isComposing) return;
  const menuOpen = !menuOf(composer).hidden && itemsOf(composer).length > 0;
  if (menuOpen) {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      return move(composer, event.key === 'ArrowDown' ? 1 : -1);
    }
    if ((event.key === 'Enter' || event.key === 'Tab') && !(event.ctrlKey || event.metaKey)) {
      event.preventDefault();
      return accept(composer, stateOf(composer).active);
    }
  }
  if (event.key === 'Escape' && !menuOf(composer).hidden) {
    event.preventDefault();
    states.set(composer, { ...stateOf(composer), dismissed: true });
    return closeMenu(composer);
  }
  if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
    event.preventDefault();
    // The send button is the state of a send: disabled when there is nothing to send, and while one is in flight.
    if (sendOf(composer)?.disabled) return undefined;
    return textarea.form?.requestSubmit();
  }
  return undefined;
}

// Keys that move inside the menu or confirm it; the caret is read again only for the other keys.
const MENU_KEYS = new Set(['ArrowUp', 'ArrowDown', 'Enter', 'Tab', 'Escape']);

// Brings every message box in the document up to date: the send button and the references of its text.
function refreshEvery(doc) {
  doc.querySelectorAll('[data-composer]').forEach(refreshAll);
}

// The documents the listeners are connected to: a second install on the same document does nothing.
const installed = new WeakSet();

/** Connects the message boxes of the document. The page calls it once; importing this script connects nothing. */
export function installComposer(doc) {
  if (installed.has(doc)) return;
  installed.add(doc);
  refreshEvery(doc);
  doc.addEventListener('htmx:load', () => refreshEvery(doc));
  doc.addEventListener('input', (event) => {
    const composer = composerOf(event.target);
    if (!composer || !event.target.matches('[data-composer-input]')) return;
    states.set(composer, { ...stateOf(composer), dismissed: false });
    updateMenu(composer);
    refreshAll(composer);
  });
  doc.addEventListener('keydown', onKeydown);
  doc.addEventListener('keyup', (event) => {
    const composer = composerOf(event.target);
    if (!composer || !event.target.matches('[data-composer-input]') || MENU_KEYS.has(event.key)) return;
    updateMenu(composer);
  });
  doc.addEventListener('click', (event) => {
    const target = event.target;
    const composer = composerOf(target);
    if (!composer) return;
    if (target.matches('[data-composer-input]')) updateMenu(composer);
    if (target.matches('[data-composer-attach]')) composer.querySelector('[data-image-input]').click();
    const remove = target.closest('[data-composer-remove-ref]');
    if (remove) {
      const textarea = textareaOf(composer);
      textarea.value = removeReference(textarea.value, remove.dataset.composerRemoveRef);
      refreshAll(composer);
    }
  });
  doc.addEventListener('mousedown', (event) => {
    const item = event.target.closest?.('[data-composer-item]');
    const composer = composerOf(item);
    if (!item || !composer) return;
    event.preventDefault();
    accept(composer, itemsOf(composer).indexOf(item));
  });
  doc.addEventListener('pointerover', (event) => {
    const item = event.target.closest?.('[data-composer-item]');
    const composer = composerOf(item);
    if (item && composer) setActive(composer, itemsOf(composer).indexOf(item));
  });
  doc.addEventListener('change', (event) => {
    const composer = composerOf(event.target);
    if (composer && event.target.matches('[data-image-input]')) refreshSend(composer);
  });
  doc.addEventListener('htmx:afterSwap', (event) => {
    const composer = composerOf(event.target);
    if (composer) {
      states.set(composer, { trigger: null, active: 0, dismissed: false });
      closeMenu(composer);
      refreshAll(composer);
    }
  });
}
