// Keeps a scrolling log pinned to its end while the user is at the bottom, and lets the user
// scroll away without being pulled back. Port of src/core/follow_bottom.js.

const STICK_THRESHOLD_PX = 80;
const SMOOTH_TIMEOUT_MS = 1500;

// True while the scroll position is within the stick threshold of the end.
export function isNearBottom(el) {
  return el.scrollHeight - el.scrollTop - el.clientHeight < STICK_THRESHOLD_PX;
}

function syncBottom(el) {
  el.__iwfStuck = isNearBottom(el);
  if (el.parentElement) el.parentElement.dataset.atBottom = String(el.__iwfStuck);
}

function endSmooth(el) {
  clearTimeout(el.__iwfSmoothTimer);
  el.__iwfSmoothTimer = undefined;
  if (!el.__iwfSmooth) return;
  el.__iwfSmooth = false;
  syncBottom(el);
}

function watch(el) {
  if (el.__iwfStuck !== undefined) return;
  el.__iwfStuck = true;
  el.addEventListener('scroll', () => {
    if (el.__iwfSmooth) {
      // A smooth scroll passes through intermediate positions: keep following until it lands.
      if (isNearBottom(el)) endSmooth(el);
      else return;
    }
    syncBottom(el);
  });
  // Any manual gesture cancels a pending smooth scroll so the user can leave the bottom.
  const cancelSmooth = () => endSmooth(el);
  el.addEventListener('wheel', cancelSmooth, { passive: true });
  el.addEventListener('touchstart', cancelSmooth, { passive: true });
  // The animation can finish short of the bottom (content grew); never stay "smooth" forever.
  el.addEventListener('scrollend', cancelSmooth);
}

// Content can grow after the caller's signal fires (streamed text, late layout): while
// auto-scroll is on, every DOM change inside the container pins it to the bottom.
function observe(el) {
  if (el.__iwfObserver) return;
  el.__iwfObserver = new MutationObserver(() => {
    if (shouldFollow(el)) el.scrollTop = el.scrollHeight;
  });
  el.__iwfObserver.observe(el, { childList: true, subtree: true, characterData: true });
}

// Following needs both: the caller asked for it (`auto`) and the user has not scrolled away.
// A user who scrolled up keeps their position even while new content keeps arriving.
function shouldFollow(el) {
  return Boolean(el.__iwfAuto) && el.__iwfStuck === true && !el.__iwfSmooth;
}

// Pins the element with `id` to its end when `auto` is true and the user is still at the bottom.
export function followBottom(id, auto) {
  requestAnimationFrame(() => {
    const el = document.getElementById(id);
    if (!el) return;
    watch(el);
    observe(el);
    el.__iwfAuto = auto;
    if (shouldFollow(el)) el.scrollTop = el.scrollHeight;
    if (!el.__iwfSmooth) syncBottom(el);
  });
}

// Scrolls the element with `id` to its end smoothly and follows the animation to the bottom.
export function scrollToBottom(id) {
  const el = document.getElementById(id);
  if (!el) return;
  watch(el);
  el.__iwfStuck = true;
  el.__iwfSmooth = true;
  clearTimeout(el.__iwfSmoothTimer);
  el.__iwfSmoothTimer = setTimeout(() => endSmooth(el), SMOOTH_TIMEOUT_MS);
  if (el.parentElement) el.parentElement.dataset.atBottom = 'true';
  el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' });
}

if (typeof window !== 'undefined') {
  window.iwfFollowBottom = { isNearBottom, followBottom, scrollToBottom };
}
