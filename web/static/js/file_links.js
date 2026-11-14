// Opens the paths of the disk that the agent writes in the chat. The server renders them as
// <a href="#" class="file-link" data-file-path="...">; a click asks the server to open the path with the system
// (relative paths resolve against the worktree of the task named by the closest [data-task-id]).

export const OPEN_PATH_ERROR_CLASS = 'file-link-error';

// openPathURL is the endpoint that opens a path of the chat of a task.
export function openPathURL(taskId) {
  return `/tasks/${encodeURIComponent(taskId)}/open-path`;
}

// openFileLink asks the server to open the path of the link. A failure is the tooltip of the link, not an alert.
export async function openFileLink(link, fetchImpl = globalThis.fetch) {
  const taskId = link.closest?.('[data-task-id]')?.dataset?.taskId;
  const path = link.getAttribute?.('data-file-path');
  if (!taskId || !path) return false;
  link.classList.remove(OPEN_PATH_ERROR_CLASS);
  link.removeAttribute('title');
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
