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

if (typeof window !== 'undefined') {
  window.iwfNotify = { requestPermissionOnFirstInteraction, showNotification };
}
