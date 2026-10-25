// Keeps the transcript of a task pinned to its last message: after each refresh the body is swapped in, and the
// scroller that holds it follows the bottom while the user stays there. This is UI behaviour only; the content comes
// from the server. The body carries data-follow-bottom and sits directly inside its scroller.
import { followBottom, scrollToBottom } from './follow_bottom.js';

// pin follows the bottom of every transcript under root. The first time a scroller is seen it goes to the bottom.
function pin(root) {
  const bodies = [];
  if (root.matches?.('[data-follow-bottom]')) bodies.push(root);
  bodies.push(...(root.querySelectorAll?.('[data-follow-bottom]') ?? []));
  for (const body of bodies) {
    const scroller = body.parentElement;
    if (!scroller?.id) continue;
    if (scroller.dataset.pinned) {
      followBottom(scroller.id, true);
    } else {
      scroller.dataset.pinned = 'true';
      scrollToBottom(scroller.id);
    }
  }
}

document.body.addEventListener('htmx:afterSettle', (event) => pin(event.target));
