// The progress panel of the task forms (see SubmitProgress in internal/core/components). htmx marks the form with
// htmx-request while its request waits, and the stylesheet shows the panel then. This script only animates it: it
// moves the active stage along and counts the seconds. The server answers once, at the end, so the stages are an
// estimate of the wait, never a report; the last stage stays active until the answer arrives.

const STEP_MS = 2500;
const SLOW_MS = 8000;
const timers = new WeakMap();

/** The steps the form will run: the ones in the phase "start" are left out when "Start now" is not checked. */
export function visibleSteps(form, overlay) {
  const startNow = form.querySelector('[name="start_now"]');
  const skipStart = startNow !== null && startNow.checked === false;
  const steps = [...overlay.querySelectorAll('[data-progress-step]')];
  for (const step of steps) step.hidden = skipStart && step.dataset.progressPhase === 'start';
  return steps.filter((step) => !step.hidden);
}

/** Marks the steps before index as done, the one at index as active and the rest as pending. */
export function showStage(steps, index) {
  steps.forEach((step, position) => {
    step.dataset.state = position < index ? 'done' : position === index ? 'active' : 'pending';
  });
}

export function formatElapsed(ms) {
  return `${Math.floor(ms / 1000)} s`;
}

/** Starts the stages and the clock of the panel. The interval stops by itself once the panel leaves the document. */
export function startProgress(form, overlay) {
  stopProgress(overlay);
  const steps = visibleSteps(form, overlay);
  const elapsed = overlay.querySelector('[data-progress-elapsed]');
  const slow = overlay.querySelector('[data-progress-slow]');
  const began = Date.now();
  let index = 0;
  showStage(steps, index);
  if (slow) slow.hidden = true;
  if (elapsed) elapsed.textContent = formatElapsed(0);
  const timer = setInterval(() => {
    // The form was swapped out or its tab closed while the request was in flight: no event will stop this interval.
    if (!overlay.isConnected) {
      stopProgress(overlay);
      return;
    }
    const waited = Date.now() - began;
    index = Math.min(Math.floor(waited / STEP_MS), steps.length - 1);
    showStage(steps, index);
    if (elapsed) elapsed.textContent = formatElapsed(waited);
    if (slow) slow.hidden = waited < SLOW_MS;
  }, 500);
  timers.set(overlay, timer);
}

/** Stops the clock of the panel. Safe to call when it is not running. */
export function stopProgress(overlay) {
  clearInterval(timers.get(overlay));
  timers.delete(overlay);
}

function panelOf(event) {
  const form = event.target;
  if (!form?.matches?.('form')) return null;
  const overlay = form.querySelector(':scope > [data-submit-progress]');
  return overlay ? { form, overlay } : null;
}

/** Connects the panel of every form of the document; the events bubble, so forms swapped in later work too. */
export function installSubmitProgress(root) {
  root.addEventListener('htmx:beforeRequest', (event) => {
    const panel = panelOf(event);
    if (panel) startProgress(panel.form, panel.overlay);
  });
  root.addEventListener('htmx:afterRequest', (event) => {
    const panel = panelOf(event);
    if (panel) stopProgress(panel.overlay);
  });
  // htmx is about to remove or swap out the form: stop its clock now, whatever the state of the request.
  root.addEventListener('htmx:beforeCleanupElement', (event) => {
    const panel = panelOf(event);
    if (panel) stopProgress(panel.overlay);
  });
}
