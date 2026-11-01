// Native notifications for finished or waiting tasks. Port of src/core/components/browser_notification.rs.
// Every call is a no-op where the Notification API is missing or permission is not granted.

// The browser has not asked the user yet, so its native prompt can still be shown.
export function shouldRequest(permission) {
  return permission === 'default';
}

// Notifications may only be shown once the user granted permission.
export function canShow(permission) {
  return permission === 'granted';
}

function currentPermission() {
  if (typeof Notification === 'undefined') return undefined;
  return Notification.permission;
}

// Asks for permission on the first click on the page (browsers only honour the request from a
// user gesture). Does nothing when permission was already decided or the API is missing.
export function requestPermissionOnFirstInteraction() {
  if (!shouldRequest(currentPermission())) return;
  document.addEventListener(
    'click',
    () => {
      Notification.requestPermission().catch((error) => console.error('no se pudo solicitar el permiso de notificaciones', error));
    },
    { once: true },
  );
}

// Shows a native notification; clicking it focuses the window. A tag replaces an earlier
// notification with the same tag, but the new one still alerts.
export function showNotification(title, body, tag) {
  if (!canShow(currentPermission())) return;
  const notification = new Notification(title, { body, tag, renotify: true });
  notification.onclick = () => window.focus();
}

// The rows that were not waiting before: the rising edge of "needs attention".
export function newlyNeedingAttention(previousIds, rows) {
  return rows.filter((row) => !previousIds.has(row.id));
}

// The sidebar lists the tasks that need the user; each row carries its task in data attributes.
function readAttentionRows(root) {
  return Array.from(root.querySelectorAll('[data-attention-task]'), (el) => ({
    id: el.dataset.attentionTask,
    title: el.dataset.attentionTitle ?? '',
    body: el.dataset.attentionBody ?? '',
  }));
}

// Notifies when a task starts needing attention. The first read only records the baseline, so
// opening the page never notifies for tasks that were already waiting.
export function watchAttention(root = document) {
  let known = new Set(readAttentionRows(root).map((row) => row.id));
  root.body.addEventListener('htmx:afterSettle', (event) => {
    if (!(event.target instanceof Element) || event.target.id !== 'board-sidebar') return;
    const rows = readAttentionRows(event.target);
    for (const row of newlyNeedingAttention(known, rows)) showNotification(row.title, row.body, row.id);
    known = new Set(rows.map((row) => row.id));
  });
}

if (typeof window !== 'undefined') {
  window.iwfNotify = { requestPermissionOnFirstInteraction, showNotification };
  const start = () => {
    requestPermissionOnFirstInteraction();
    watchAttention(document);
  };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start);
  else start();
}
