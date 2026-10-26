// Behaviour of the task forms: shows and hides the advanced options, previews the chosen images and takes the
// images pasted into a prompt. The server validates everything (types, sizes, counts, limits); this script only
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

document.addEventListener('change', (event) => {
  const target = event.target;
  if (!(target instanceof HTMLInputElement)) return;
  if (target.matches('[data-toggle-target]')) toggleBlock(target);
  if (target.matches('[data-image-input]')) previewImages(target);
});

document.addEventListener('paste', (event) => {
  const target = event.target;
  if (!(target instanceof HTMLTextAreaElement) || !target.matches('[data-image-paste]')) return;
  const images = Array.from(event.clipboardData?.files ?? []).filter((file) => file.type.startsWith('image/'));
  if (images.length === 0) return;
  event.preventDefault();
  takePastedImages(target, images);
});
