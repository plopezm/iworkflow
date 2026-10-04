const PI_STICK_THRESHOLD_PX = 80;

export function pi_is_near_bottom(el) {
  return el.scrollHeight - el.scrollTop - el.clientHeight < PI_STICK_THRESHOLD_PX;
}

function piSyncBottom(el) {
  el.__piStuck = pi_is_near_bottom(el);
  if (el.parentElement) el.parentElement.dataset.atBottom = String(el.__piStuck);
}

function piWatch(el) {
  if (el.__piStuck !== undefined) return;
  el.__piStuck = true;
  el.addEventListener('scroll', () => {
    if (el.__piSmooth) {
      // A smooth scroll passes through intermediate positions: keep following until it lands.
      if (pi_is_near_bottom(el)) el.__piSmooth = false;
      else return;
    }
    piSyncBottom(el);
  });
  // Any manual gesture cancels a pending smooth scroll so the user can leave the bottom.
  const cancelSmooth = () => { el.__piSmooth = false; };
  el.addEventListener('wheel', cancelSmooth, { passive: true });
  el.addEventListener('touchstart', cancelSmooth, { passive: true });
}

export function pi_follow_bottom(id) {
  requestAnimationFrame(() => {
    const el = document.getElementById(id);
    if (!el) return;
    piWatch(el);
    if (el.__piStuck && !el.__piSmooth) el.scrollTop = el.scrollHeight;
    if (!el.__piSmooth) piSyncBottom(el);
  });
}

export function pi_scroll_to_bottom(id) {
  const el = document.getElementById(id);
  if (!el) return;
  piWatch(el);
  el.__piStuck = true;
  el.__piSmooth = true;
  if (el.parentElement) el.parentElement.dataset.atBottom = 'true';
  el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' });
}
