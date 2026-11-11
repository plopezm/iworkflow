// Opens the dialogs of the Hooks screen as modal dialogs and asks the server for the view again when one is dismissed.
// The server decides which dialog exists and renders it open or not; this module only shows the dialog after a swap
// and forwards a cancel (Escape) or a click on the backdrop to the view the dialog names.
// Run with: node --test web/static/js/

const DIALOG_SELECTOR = 'dialog[data-hook-dialog]';
const SCREEN_ID = 'hooks-screen';

// dialogsToOpen returns the hooks dialogs under root that are not open yet.
export function dialogsToOpen(root) {
  if (!root?.querySelectorAll) return [];
  return [...root.querySelectorAll(DIALOG_SELECTOR)].filter((dialog) => !dialog.open);
}

// dismissPathOf is the view a dialog returns to when it is dismissed. Empty when the server named none.
export function dismissPathOf(dialog) {
  return dialog?.dataset?.dismissPath || '';
}

function openDialogs(root) {
  for (const dialog of dialogsToOpen(root)) {
    if (typeof dialog.showModal === 'function') dialog.showModal();
  }
}

// dismiss asks the server for the view the dialog names. The answer replaces the screen, so the dialog goes with it.
function dismiss(dialog) {
  const path = dismissPathOf(dialog);
  const screen = document.getElementById(SCREEN_ID);
  if (!path || !screen || typeof htmx === 'undefined') return;
  htmx.ajax('GET', path, { target: screen, swap: 'outerHTML' });
}

if (typeof document !== 'undefined') {
  // Escape cancels the dialog natively; it stays open here until the server answers with the view.
  document.addEventListener('cancel', (event) => {
    if (!event.target.matches?.(DIALOG_SELECTOR)) return;
    event.preventDefault();
    dismiss(event.target);
  }, true);

  // A click on the backdrop lands on the dialog itself, since its content fills it.
  document.addEventListener('click', (event) => {
    if (event.target.matches?.(DIALOG_SELECTOR)) dismiss(event.target);
  });

  document.addEventListener('htmx:afterSettle', () => openDialogs(document));
  openDialogs(document);
}
