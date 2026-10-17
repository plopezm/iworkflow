// Dock helpers: divider dragging and tab drag start. Pure DOM, no knowledge of the app.

// Drags the divider that fired `event`: resizes the previous sibling live (inline flex-basis)
// and reports the final fraction (0..1) of the container once to `on_done`.
export function pi_dock_resize(event, vertical, on_done) {
  const divider = event.target && event.target.closest ? event.target.closest('[data-dock-divider]') : null;
  const container = divider && divider.parentElement;
  const first = divider && divider.previousElementSibling;
  if (!container || !first) return;
  event.preventDefault();
  const rect = container.getBoundingClientRect();
  let ratio = null;
  const move = (e) => {
    const pos = vertical
      ? (e.clientY - rect.top) / rect.height
      : (e.clientX - rect.left) / rect.width;
    ratio = Math.min(0.88, Math.max(0.12, pos));
    first.style.flexBasis = (ratio * 100) + '%';
  };
  const finish = () => {
    window.removeEventListener('pointermove', move);
    window.removeEventListener('pointerup', finish);
    window.removeEventListener('pointercancel', finish);
    document.body.classList.remove('dock-resizing');
    if (ratio !== null) on_done(ratio);
  };
  document.body.classList.add('dock-resizing');
  window.addEventListener('pointermove', move);
  window.addEventListener('pointerup', finish);
  window.addEventListener('pointercancel', finish);
}

// Firefox only starts a drag when the data transfer carries data.
export function pi_dock_drag_start(event, payload) {
  const transfer = event.dataTransfer;
  if (!transfer) return;
  transfer.setData('text/plain', payload);
  transfer.effectAllowed = 'move';
}

// Scrolls the tab strip horizontally just enough to reveal `element` (no-op if visible).
export function pi_dock_reveal_tab(element) {
  if (!element || !element.scrollIntoView) return;
  requestAnimationFrame(() => {
    element.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'nearest' });
  });
}

// True when the pointer of a drag/drop `event` is over the right half of the tab it targets.
export function pi_dock_drop_after(event) {
  const tab = event.target && event.target.closest ? event.target.closest('[role="tab"]') : null;
  if (!tab) return true;
  const rect = tab.getBoundingClientRect();
  return event.clientX > rect.left + rect.width / 2;
}
