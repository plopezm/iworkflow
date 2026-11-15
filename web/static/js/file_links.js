// Opens the paths of the disk that the agent writes in the chat. The server renders them as
// <a href="#" class="file-link" data-file-path="...">; a click asks the server to open the path with the system
// (relative paths resolve against the worktree of the task named by the closest [data-task-id]).

export const OPEN_PATH_ERROR_CLASS = 'file-link-error';

// openPathURL is the endpoint that opens a path of the chat of a task.
export function openPathURL(taskId) {
  return `/tasks/${encodeURIComponent(taskId)}/open-path`;
}

// IMAGE_PATH matches the paths of the images that the server can preview, with their optional :line:col.
const IMAGE_PATH = /\.(?:png|jpe?g|gif|webp)(?::\d+(?::\d+)?)?$/i;

// previewImageURL is the endpoint that serves an image of the chat of a task.
export function previewImageURL(taskId, path) {
  return `/tasks/${encodeURIComponent(taskId)}/preview-image?path=${encodeURIComponent(path)}`;
}

// showImage shows the image of the path in the lightbox of the page. It returns false when the page has no lightbox.
export function showImage(taskId, path, doc = globalThis.document) {
  const modal = doc?.querySelector?.('[data-chat-image-modal]');
  const img = modal?.querySelector?.('[data-chat-image]');
  if (!modal || !img) return false;
  img.src = previewImageURL(taskId, path);
  img.alt = path;
  modal.style.display = 'flex';
  modal.querySelector('[data-modal-panel]')?.focus?.();
  return true;
}

// openFileLink asks the server to open the path of the link. A failure is the tooltip of the link, not an alert.
export async function openFileLink(link, fetchImpl = globalThis.fetch) {
  const taskId = link.closest?.('[data-task-id]')?.dataset?.taskId;
  const path = link.getAttribute?.('data-file-path');
  if (!taskId || !path) return false;
  link.classList.remove(OPEN_PATH_ERROR_CLASS);
  link.removeAttribute('title');
  if (IMAGE_PATH.test(path.trim()) && showImage(taskId, path)) return true;
  try {
    const response = await fetchImpl(openPathURL(taskId), {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: new URLSearchParams({ path }).toString(),
    });
    if (response.ok) return true;
    link.setAttribute('title', (await response.text()).trim());
  } catch {
    link.setAttribute('title', path);
  }
  link.classList.add(OPEN_PATH_ERROR_CLASS);
  return false;
}

// onClick opens the path of a clicked file link and keeps the browser on the page.
export function onClick(event) {
  const link = event.target?.closest?.('a[data-file-path]');
  if (!link) return;
  event.preventDefault();
  void openFileLink(link);
}

if (typeof document !== 'undefined') document.addEventListener('click', onClick);
