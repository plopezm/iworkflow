// Keeps the transcript of a task pinned to its last message: after each refresh the body is swapped in, and the
// scroller that holds it follows the bottom while the user stays there. When entering the window/tab, it jumps
// to the bottom by default.
import { followBottom, resetToBottom } from './follow_bottom.js';

// Tracks whether each scroller is currently visible so we only reset to bottom when entering the window.
const visibleScrollers = new WeakSet();

export function isElementVisible(el) {
  if (!el) return false;
  if (typeof el.isConnected === 'boolean' && !el.isConnected) return false;
  const pane = el.closest?.('[data-dock-pane]');
  if (pane?.classList?.contains?.('hidden')) return false;
  if (typeof el.offsetWidth === 'number' && typeof el.offsetHeight === 'number') {
    if (el.offsetWidth === 0 && el.offsetHeight === 0) return false;
  }
  return true;
}

export function checkVisibility(scroller) {
  if (!scroller?.id) return;
  const nowVisible = isElementVisible(scroller);
  const wasVisible = visibleScrollers.has(scroller);
  if (nowVisible && !wasVisible) {
    visibleScrollers.add(scroller);
    resetToBottom(scroller.id);
  } else if (!nowVisible && wasVisible) {
    visibleScrollers.delete(scroller);
  }
}

const observedScrollers = new WeakSet();
const visibilityObserver = typeof IntersectionObserver !== 'undefined'
  ? new IntersectionObserver((entries) => {
      for (const entry of entries) {
        checkVisibility(entry.target);
      }
    })
  : null;

function watchVisibility(scroller) {
  if (!scroller || !visibilityObserver || observedScrollers.has(scroller)) return;
  observedScrollers.add(scroller);
  visibilityObserver.observe(scroller);
}

// pin follows the bottom of every transcript under root.
export function pin(root) {
  if (!root) return;
  const bodies = [];
  if (root.matches?.('[data-follow-bottom]')) bodies.push(root);
  bodies.push(...(root.querySelectorAll?.('[data-follow-bottom]') ?? []));
  for (const body of bodies) {
    const scroller = body.parentElement;
    if (!scroller?.id) continue;
    watchVisibility(scroller);
    if (scroller.dataset.pinned) {
      // Swapping in new content (e.g. SSE message update): follow bottom only if user hasn't scrolled away
      followBottom(scroller.id, true);
    } else {
      // First time scroller is seen: mark pinned and go to bottom
      scroller.dataset.pinned = 'true';
      checkVisibility(scroller);
    }
  }
}

// When dock settles or panes change, check active panes to enter the window at bottom
export function onDockPanes(root = document) {
  if (!root || typeof root.querySelectorAll !== 'function') return;
  const scrollers = root.querySelectorAll?.('[data-dock-pane]:not(.hidden) [data-follow-bottom]') ?? [];
  for (const body of scrollers) {
    const scroller = body.parentElement;
    if (scroller?.id) checkVisibility(scroller);
  }
}

if (typeof document !== 'undefined') {
  document.body?.addEventListener?.('htmx:afterSettle', (event) => {
    const target = event.target;
    if (target?.id === 'dock') {
      onDockPanes(target);
    }
    pin(target);
  });
  const start = () => {
    pin(document);
    onDockPanes(document);
  };
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', start);
  } else {
    start();
  }
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') onDockPanes(document);
  });
}
