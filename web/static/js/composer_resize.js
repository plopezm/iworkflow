// A drag handle for the height of the composer textareas. Browsers on touch screens give a textarea no native
// resize handle, so the grip (data-composer-grip) in the corner of the field stretches it by pointer drag.
// The listeners are delegated from the document, so it works wherever the composer is rendered.

const MIN_HEIGHT = 72;

/** The height a drag gives the textarea: the start height plus the pointer travel, never under the minimum. */
export function draggedHeight(startHeight, startY, currentY, minHeight = MIN_HEIGHT) {
  return Math.max(minHeight, startHeight + (currentY - startY));
}

// The documents the listeners are connected to: a second install on the same document does nothing.
const installed = new WeakSet();

/** Connects the resize grips of the document. The page calls it once; a repeat does nothing. */
export function installComposerResize(doc) {
  if (installed.has(doc)) return;
  installed.add(doc);
  let drag = null;
  doc.addEventListener('pointerdown', (event) => {
    const grip = event.target?.closest?.('[data-composer-grip]');
    const textarea = grip?.closest('[data-composer-field]')?.querySelector('[data-composer-input]');
    if (!textarea) return;
    event.preventDefault();
    grip.setPointerCapture?.(event.pointerId);
    drag = { grip, textarea, startY: event.clientY, startHeight: textarea.getBoundingClientRect().height };
  });
  doc.addEventListener('pointermove', (event) => {
    if (!drag) return;
    const min = Number.parseFloat(getComputedStyle(drag.textarea).minHeight) || MIN_HEIGHT;
    drag.textarea.style.height = `${draggedHeight(drag.startHeight, drag.startY, event.clientY, min)}px`;
  });
  const end = () => {
    drag = null;
  };
  doc.addEventListener('pointerup', end);
  doc.addEventListener('pointercancel', end);
}
