// Behaviour of the task forms: shows and hides the advanced options, keeps the workspace of the task out of its
// related projects, previews the chosen images and takes the images pasted into a prompt. The server validates everything (types, sizes, counts, limits); this script only
// shows it and fills the file input.
//
// The message box of the task detail page is connected from here, because every page already loads this script: its
// menus and keys are delegated from the document, so it works wherever the composer is rendered.
import { installComposer } from './composer.js';

installComposer(document);

/** Shows or hides the block a toggle names in data-toggle-target, following the toggle. */
function toggleBlock(toggle) {
  const block = document.getElementById(toggle.dataset.toggleTarget);
  if (block) block.hidden = !toggle.checked;
}

/**
 * Hides and unchecks the related project row of the workspace the form's select names, so the task cannot relate to
 * the project it runs in. The server drops that choice too; this only keeps the list honest. Without a select (the
 * edit page, which has no workspace choice) there is nothing to sync.
 */
export function syncOwnProject(select) {
  if (!select) return;
  const form = select.closest('form');
  const inputs = form?.querySelectorAll('[data-related-projects] input[name="related_project_id"]') ?? [];
  for (const input of inputs) {
    const own = input.dataset.workspaceId === select.value;
    const row = input.closest('label') ?? input;
    row.hidden = own;
    if (own) input.checked = false;
  }
}

/** Shows the chosen files of an image input as previews inside its picker. */
function previewImages(input) {
  const picker = input.closest('[data-image-picker]');
  const previews = picker?.querySelector('[data-image-previews]');
  if (!previews) return;
  previews.replaceChildren(
    ...Array.from(input.files, (file) => {
      const image = document.createElement('img');
      image.src = URL.createObjectURL(file);
      image.alt = file.name;
      image.className = 'h-16 w-16 rounded object-cover';
      return image;
    }),
  );
}

/** Adds the pasted image files to the input of the picker that belongs to the form of the textarea. */
function takePastedImages(textarea, files) {
  const input = textarea.closest('form')?.querySelector('[data-image-input]');
  if (!input || files.length === 0) return;
  const transfer = new DataTransfer();
  for (const file of input.files) transfer.items.add(file);
  for (const file of files) transfer.items.add(file);
  input.files = transfer.files;
  input.dispatchEvent(new Event('change', { bubbles: true }));
}

// The documents the listeners are connected to: a second install on the same document does nothing.
const installed = new WeakSet();

/** Connects the task forms of the document. The page calls it once through the module bottom; a repeat does nothing. */
export function installTaskForm(doc) {
  if (installed.has(doc)) return;
  installed.add(doc);
  // The create dialog and the edit page arrive through htmx, so their related projects list is synced when it loads.
  doc.addEventListener('htmx:load', (event) => {
    if (!(event.target instanceof Element)) return;
    for (const select of event.target.querySelectorAll('select[name="workspace_id"]')) syncOwnProject(select);
  });

  doc.addEventListener('change', (event) => {
    const target = event.target;
    if (target instanceof HTMLSelectElement && target.name === 'workspace_id') syncOwnProject(target);
    if (!(target instanceof HTMLInputElement)) return;
    if (target.matches('[data-toggle-target]')) toggleBlock(target);
    if (target.matches('[data-image-input]')) previewImages(target);
  });

  doc.addEventListener('paste', (event) => {
    const target = event.target;
    if (!(target instanceof HTMLTextAreaElement) || !target.matches('[data-image-paste]')) return;
    const images = Array.from(event.clipboardData?.files ?? []).filter((file) => file.type.startsWith('image/'));
    if (images.length === 0) return;
    event.preventDefault();
    takePastedImages(target, images);
  });
}

installTaskForm(document);
