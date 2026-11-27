// Keeps the scroll position of the scrolling container while htmx replaces an element that carries
// `data-keep-scroll`. Replacing a large element with outerHTML makes the browser lose its scroll anchor and
// jump to the end of the container; the position seen before the swap is put back right after it.

// Nearest ancestor that scrolls vertically, or null when the page itself scrolls.
export function scrollParent(el) {
  for (let node = el?.parentElement; node; node = node.parentElement) {
    const overflowY = getComputedStyle(node).overflowY;
    if ((overflowY === 'auto' || overflowY === 'scroll') && node.scrollHeight > node.clientHeight) return node;
  }
  return null;
}

export function setupKeepScroll(doc = document) {
  let saved = null;
  doc.addEventListener('htmx:beforeSwap', (event) => {
    const target = event.target;
    if (!target?.hasAttribute?.('data-keep-scroll')) return;
    const scroller = scrollParent(target);
    saved = scroller ? { scroller, top: scroller.scrollTop } : null;
  });
  const restore = () => {
    if (!saved) return;
    const { scroller, top } = saved;
    if (scroller.isConnected && scroller.scrollTop !== top) scroller.scrollTop = top;
  };
  doc.addEventListener('htmx:afterSwap', restore);
  doc.addEventListener('htmx:afterSettle', () => {
    restore();
    saved = null;
  });
}

if (typeof document !== 'undefined') setupKeepScroll(document);
