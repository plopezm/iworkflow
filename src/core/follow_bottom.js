const PI_STICK_THRESHOLD_PX = 80;
const PI_SMOOTH_TIMEOUT_MS = 1500;

export function pi_is_near_bottom(el) {
  return el.scrollHeight - el.scrollTop - el.clientHeight < PI_STICK_THRESHOLD_PX;
}

function piSyncBottom(el) {
  el.__piStuck = pi_is_near_bottom(el);
  if (el.parentElement) el.parentElement.dataset.atBottom = String(el.__piStuck);
}

function piEndSmooth(el) {
  clearTimeout(el.__piSmoothTimer);
  el.__piSmoothTimer = undefined;
  if (!el.__piSmooth) return;
  el.__piSmooth = false;
  piSyncBottom(el);
}

function piWatch(el) {
  if (el.__piStuck !== undefined) return;
  el.__piStuck = true;
  el.addEventListener('scroll', () => {
    if (el.__piSmooth) {
      // A smooth scroll passes through intermediate positions: keep following until it lands.
      if (pi_is_near_bottom(el)) piEndSmooth(el);
      else return;
    }
    piSyncBottom(el);
  });
  // Any manual gesture cancels a pending smooth scroll so the user can leave the bottom.
  const cancelSmooth = () => piEndSmooth(el);
  el.addEventListener('wheel', cancelSmooth, { passive: true });
  el.addEventListener('touchstart', cancelSmooth, { passive: true });
  // The animation can finish short of the bottom (content grew); never stay "smooth" forever.
  el.addEventListener('scrollend', cancelSmooth);
}

// Content can grow after the caller's signal fires (streamed text, late layout): while
// auto-scroll is on, every DOM change inside the container pins it to the bottom.
function piObserve(el) {
  if (el.__piObserver) return;
  el.__piObserver = new MutationObserver(() => {
    if (el.__piAuto && !el.__piSmooth) el.scrollTop = el.scrollHeight;
  });
  el.__piObserver.observe(el, { childList: true, subtree: true, characterData: true });
}

export function pi_follow_bottom(id, auto) {
  requestAnimationFrame(() => {
    const el = document.getElementById(id);
    if (!el) return;
    piWatch(el);
    piObserve(el);
    el.__piAuto = auto;
    if (auto && !el.__piSmooth) el.scrollTop = el.scrollHeight;
    if (!el.__piSmooth) piSyncBottom(el);
  });
}

export function pi_scroll_to_bottom(id) {
  const el = document.getElementById(id);
  if (!el) return;
  piWatch(el);
  el.__piStuck = true;
  el.__piSmooth = true;
  clearTimeout(el.__piSmoothTimer);
  el.__piSmoothTimer = setTimeout(() => piEndSmooth(el), PI_SMOOTH_TIMEOUT_MS);
  if (el.parentElement) el.parentElement.dataset.atBottom = 'true';
  el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' });
}
