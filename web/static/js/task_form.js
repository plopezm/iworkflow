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

/** Shows the chosen related projects as chips and their count next to the button that opens the dialog. */
export function syncRelatedSummary(root) {
  const summary = root?.querySelector?.('[data-related-summary]');
  const count = root?.querySelector?.('[data-related-count]');
  if (!summary || !count) return;
  const chosen = Array.from(root.querySelectorAll('input[name="related_project_id"]')).filter((input) => input.checked);
  count.textContent = chosen.length > 0 ? `(${chosen.length})` : '';
  summary.replaceChildren(
    ...chosen.map((input) => {
      const chip = document.createElement('li');
      chip.className = 'rounded border border-border px-2 py-0.5 text-xs text-fg';
      chip.textContent = input.closest?.('[data-related-row]')?.dataset.relatedName ?? input.value;
      return chip;
    }),
  );
}

/** Keeps the rows of the related projects dialog whose name or path contains the filter text (case-insensitive). */
export function filterRelatedProjects(filter) {
  const root = filter.closest?.('[data-related-projects]');
  if (!root) return;
  const needle = filter.value.trim().toLowerCase();
  let visible = 0;
  for (const row of root.querySelectorAll('[data-related-row]')) {
    const match = row.dataset.relatedText.toLowerCase().includes(needle);
    // An inline display rule, so it never fights the hidden attribute that keeps the own project out of the list.
    row.style.display = match ? '' : 'none';
    if (match && !row.hidden) visible += 1;
  }
  const empty = root.querySelector('[data-related-empty]');
  if (empty) empty.hidden = visible > 0;
}

/** Opens the related projects dialog and focuses its filter. */
function openRelatedDialog(button) {
  const root = button.closest('[data-related-projects]');
  const modal = root?.querySelector('[data-modal]');
  if (!modal) return;
  modal.style.display = 'flex';
  root.querySelector('[data-related-filter]')?.focus();
}

/** The identity of a chosen file: the same picture picked twice is kept once. */
function fileKey(file) {
  return `${file.name}:${file.size}:${file.lastModified}`;
}

/** Returns the files already chosen followed by the new ones that are not among them. */
export function mergeFiles(chosen, incoming) {
  const seen = new Set(chosen.map(fileKey));
  const merged = [...chosen];
  for (const file of incoming) {
    if (seen.has(fileKey(file))) continue;
    seen.add(fileKey(file));
    merged.push(file);
  }
  return merged;
}

/** Returns the files without the one at index; an index outside the list leaves it as it is. */
export function withoutFile(files, index) {
  return files.filter((_, i) => i !== index);
}

// The browser replaces the files of an input on every pick, so the picker keeps its own list and writes it back.
const chosenImages = new WeakMap();

/** Sets the files of the input to the list and shows them as previews, each with a button that removes it. */
function showImages(input, files) {
  chosenImages.set(input, files);
  const transfer = new DataTransfer();
  for (const file of files) transfer.items.add(file);
  input.files = transfer.files;
  const previews = input.closest('[data-image-picker]')?.querySelector('[data-image-previews]');
  if (!previews) return;
  previews.replaceChildren(
    ...files.map((file, index) => {
      const item = document.createElement('div');
      item.className = 'relative';
      const image = document.createElement('img');
      image.src = URL.createObjectURL(file);
      image.alt = file.name;
      image.className = 'h-16 w-16 rounded object-cover';
      const remove = document.createElement('button');
      remove.type = 'button';
      remove.className = 'composer-ref-remove absolute -right-1 -top-1';
      remove.dataset.imageRemove = String(index);
      remove.setAttribute('aria-label', (previews.dataset.removeLabel ?? '').replace('{name}', file.name));
      remove.textContent = '×';
      item.append(image, remove);
      return item;
    }),
  );
}

/** Adds files to the ones the picker of the input already holds. */
function addImages(input, files) {
  showImages(input, mergeFiles(chosenImages.get(input) ?? [], files));
}

/** Adds the pasted image files to the input of the picker that belongs to the form of the textarea. */
function takePastedImages(textarea, files) {
  const input = textarea.closest('form')?.querySelector('[data-image-input]');
  if (!input || files.length === 0) return;
  addImages(input, files);
  input.dispatchEvent(new Event('change', { bubbles: true }));
}

/** Marks or unmarks an image the task already has for removal on save. */
function toggleExistingImage(button) {
  const item = button.closest('[data-existing-image]');
  const checkbox = item?.querySelector('input[name="remove_attachment_ids"]');
  if (!checkbox) return;
  checkbox.checked = !checkbox.checked;
  item.classList.toggle('opacity-40', checkbox.checked);
  button.setAttribute('aria-pressed', String(checkbox.checked));
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
    for (const root of event.target.querySelectorAll('[data-related-projects]')) syncRelatedSummary(root);
  });

  doc.addEventListener('click', (event) => {
    const button = event.target?.closest?.('[data-related-open]');
    if (button) openRelatedDialog(button);
    const existing = event.target?.closest?.('[data-existing-image-toggle]');
    if (existing) toggleExistingImage(existing);
    const remove = event.target?.closest?.('[data-image-remove]');
    const input = remove?.closest('[data-image-picker]')?.querySelector('[data-image-input]');
    if (input) showImages(input, withoutFile(chosenImages.get(input) ?? [], Number(remove.dataset.imageRemove)));
  });

  doc.addEventListener('input', (event) => {
    if (event.target?.matches?.('[data-related-filter]')) filterRelatedProjects(event.target);
  });

  // Enter in the filter must not submit the form behind the dialog.
  doc.addEventListener('keydown', (event) => {
    if (event.key === 'Enter' && event.target?.matches?.('[data-related-filter]')) event.preventDefault();
  });

  doc.addEventListener('change', (event) => {
    const target = event.target;
    if (target instanceof HTMLSelectElement && target.name === 'workspace_id') {
      syncOwnProject(target);
      for (const root of target.closest('form')?.querySelectorAll?.('[data-related-projects]') ?? []) syncRelatedSummary(root);
    }
    if (!(target instanceof HTMLInputElement)) return;
    if (target.name === 'related_project_id') syncRelatedSummary(target.closest?.('[data-related-projects]'));
    if (target.matches('[data-toggle-target]')) toggleBlock(target);
    if (target.matches('[data-image-input]') && target.closest('[data-image-picker]')) addImages(target, Array.from(target.files));
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
