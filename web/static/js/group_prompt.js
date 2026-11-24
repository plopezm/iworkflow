// Asks for a name in a dialog of the page before the workspace group requests go out. It replaces htmx's hx-prompt,
// which calls window.prompt: Electron does not implement it, so the "+" and the rename did nothing in the desktop app.
// An element opts in with data-prompt-question (the label), data-prompt-ok and data-prompt-cancel (the button texts)
// and data-prompt-field (the form field that carries the answer). The group endpoints read their fields from the form.

// answers holds the answer of each element until its request is configured; the request consumes it.
const answers = new WeakMap();

function attr(element, name) {
  return element && element.getAttribute ? element.getAttribute(name) : null;
}

// isBlank reports whether an answer has no name to send: the user cancelled, or typed nothing but spaces.
function isBlank(answer) {
  return typeof answer !== 'string' || answer.trim() === '';
}

// askInDialog shows a modal dialog with one text field and resolves to what the user typed, or null when they
// cancel (Cancel button, Escape or a click on the backdrop).
export function askInDialog(element, root = document) {
  return new Promise((resolve) => {
    const dialog = root.createElement('dialog');
    dialog.className = 'm-auto w-[min(100%,24rem)] rounded-2xl border border-border/60 bg-surface p-0 text-fg shadow-2xl backdrop:bg-black/60 backdrop:backdrop-blur-sm';
    const form = root.createElement('form');
    form.method = 'dialog';
    form.className = 'stack gap-3 p-5';
    const label = root.createElement('label');
    label.className = 'stack gap-1.5 text-sm font-semibold text-fg';
    label.textContent = attr(element, 'data-prompt-question') || '';
    const input = root.createElement('input');
    input.type = 'text';
    input.name = 'answer';
    input.className = 'input';
    input.autocomplete = 'off';
    label.append(input);
    const buttons = root.createElement('div');
    buttons.className = 'row justify-end gap-2';
    const cancel = root.createElement('button');
    cancel.type = 'button';
    cancel.className = 'btn-ghost';
    cancel.textContent = attr(element, 'data-prompt-cancel') || 'Cancel';
    const ok = root.createElement('button');
    ok.type = 'submit';
    ok.className = 'btn-primary';
    ok.textContent = attr(element, 'data-prompt-ok') || 'OK';
    buttons.append(cancel, ok);
    form.append(label, buttons);
    dialog.append(form);

    let answer = null;
    form.addEventListener('submit', () => { answer = input.value; });
    cancel.addEventListener('click', () => dialog.close());
    dialog.addEventListener('click', (event) => { if (event.target === dialog) dialog.close(); });
    dialog.addEventListener('close', () => {
      dialog.remove();
      resolve(answer);
    });
    root.body.append(dialog);
    dialog.showModal();
    input.focus();
  });
}

export function setupGroupPrompt(root = document, ask = askInDialog) {
  if (!root.body) return;
  // htmx fires htmx:confirm before it asks anything: hold the request, ask in the dialog, then let it go on.
  root.body.addEventListener('htmx:confirm', (event) => {
    const element = event.detail.elt;
    if (!attr(element, 'data-prompt-question') || !attr(element, 'data-prompt-field')) return;
    event.preventDefault();
    ask(element).then((answer) => {
      if (isBlank(answer)) return;
      answers.set(element, answer);
      event.detail.issueRequest(true);
    });
  });
  root.body.addEventListener('htmx:configRequest', (event) => {
    const element = event.detail.elt;
    if (!answers.has(element)) return;
    event.detail.parameters[attr(element, 'data-prompt-field')] = answers.get(element);
    answers.delete(element);
  });
}

if (typeof document !== 'undefined') setupGroupPrompt(document);
