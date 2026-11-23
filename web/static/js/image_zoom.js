// Zooms the images that carry data-zoomable: a click (or tap) toggles between the image fitted to its box and the
// image at its real size, scrollable inside its box. The CSS of each image reacts to the data-zoomed attribute.

export const ZOOM_ATTR = 'data-zoomed';

// isZoomed reports whether the image is shown at its real size.
export function isZoomed(img) {
  return img?.hasAttribute?.(ZOOM_ATTR) === true;
}

// setZoom shows the image at its real size (true) or fitted to its box (false), and keeps aria-label in sync.
export function setZoom(img, zoomed) {
  if (!img) return;
  if (zoomed) img.setAttribute(ZOOM_ATTR, '');
  else img.removeAttribute(ZOOM_ATTR);
}

// onClick toggles the zoom of a clicked zoomable image.
export function onClick(event) {
  const img = event.target?.closest?.('img[data-zoomable]');
  if (!img) return;
  setZoom(img, !isZoomed(img));
}

if (typeof document !== 'undefined') document.addEventListener('click', onClick);
