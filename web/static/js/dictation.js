// Voice dictation of a task prompt: the mic button records the microphone, the user reviews the recording, and the
// server transcribes it. The text is appended to the prompt as a new paragraph. This script records and converts the
// audio to the 16 kHz mono WAV the server takes; the server validates it and decides what an error is.
// Events are delegated from the document, so the mic keeps working after htmx replaces the composer.

/** The longest recording the server takes, in milliseconds: the recording stops itself at this point. */
export const MAX_RECORDING_MS = 60_000;

/** The sample rate of the WAV the server takes. */
export const SAMPLE_RATE = 16000;

const IDLE = 'idle';
const RECORDING = 'recording';
const STOPPING = 'stopping';
const REVIEW = 'review';
const TRANSCRIBING = 'transcribing';

/** Adds the transcribed text to the prompt as a new paragraph. A blank text leaves the prompt as it is. */
export function appendParagraph(current, text) {
  const added = text.trim();
  if (added === '') return current;
  const kept = current.trimEnd();
  return kept === '' ? added : `${kept}\n\n${added}`;
}

/** Averages the channels of a recording into one. A mono signal comes back as it is. */
export function downmix(channels) {
  const length = channels[0]?.length ?? 0;
  const mono = new Float32Array(length);
  for (const channel of channels) {
    for (let i = 0; i < length; i += 1) mono[i] += channel[i] / channels.length;
  }
  return mono;
}

/** The elapsed time of a recording as m:ss. */
export function formatElapsed(milliseconds) {
  const seconds = Math.floor(milliseconds / 1000);
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`;
}

function writeText(view, offset, text) {
  for (let i = 0; i < text.length; i += 1) view.setUint8(offset + i, text.charCodeAt(i));
}

/**
 * Encodes mono samples in [-1, 1] as a 16-bit PCM WAV file. Values outside the range are clipped. The header is the
 * canonical 44-byte RIFF one, so the server reads the rate and the channels from it.
 */
export function encodeWav(samples, sampleRate) {
  const dataBytes = samples.length * 2;
  const buffer = new ArrayBuffer(44 + dataBytes);
  const view = new DataView(buffer);
  writeText(view, 0, 'RIFF');
  view.setUint32(4, 36 + dataBytes, true);
  writeText(view, 8, 'WAVE');
  writeText(view, 12, 'fmt ');
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true);
  view.setUint16(22, 1, true);
  view.setUint32(24, sampleRate, true);
  view.setUint32(28, sampleRate * 2, true);
  view.setUint16(32, 2, true);
  view.setUint16(34, 16, true);
  writeText(view, 36, 'data');
  view.setUint32(40, dataBytes, true);
  for (let i = 0; i < samples.length; i += 1) {
    const sample = Math.max(-1, Math.min(1, samples[i]));
    view.setInt16(44 + i * 2, sample < 0 ? Math.round(sample * 32768) : Math.round(sample * 32767), true);
  }
  return buffer;
}

/**
 * Creates the controller of one composer root. Its collaborators come in deps (see browserDeps): the recorder, the
 * conversion to WAV, the request, the object URLs, the clock and the timers. The DOM is read through the root's
 * querySelector and dataset, so the controller works on any root that carries the attributes of TaskPromptField.
 */
export function createDictation(root, deps = browserDeps()) {
  const element = (selector) => root.querySelector(selector);
  const label = (name) => root.dataset[`label${name}`] ?? '';

  let state = IDLE;
  // True while the microphone is being opened, so a second click cannot start a second recording.
  let starting = false;
  // The recording in progress, the blob under review and the object URL of that blob.
  let session = null;
  let recording = null;
  let url = null;
  // The notice of the panel while idle or in review: a refused microphone, a server error, the generic failure.
  let message = '';
  let startedAt = 0;
  // The auto-stop and the elapsed-time tick of the recording in progress.
  let timers = null;

  const statusText = () => {
    if (state === RECORDING || state === STOPPING) return label('Recording');
    if (state === TRANSCRIBING) return label('Transcribing');
    return message;
  };

  // Shows the state: the mic, its elapsed time, the panel, the status line, the review and its two buttons.
  function render() {
    const recordingNow = state === RECORDING;
    const reviewing = state === REVIEW;
    const mic = element('[data-dictation-mic]');
    mic.setAttribute('aria-label', recordingNow ? label('Stop') : label('Record'));
    mic.setAttribute('aria-pressed', String(recordingNow));
    mic.disabled = state === STOPPING || reviewing || state === TRANSCRIBING;
    const elapsed = element('[data-dictation-elapsed]');
    elapsed.hidden = !recordingNow;
    elapsed.textContent = recordingNow ? formatElapsed(deps.now() - startedAt) : '';
    element('[data-dictation-panel]').hidden = state === IDLE && message === '';
    element('[data-dictation-status]').textContent = statusText();
    element('[data-dictation-audio]').hidden = !reviewing;
    const question = element('[data-dictation-question]');
    question.hidden = !reviewing;
    question.textContent = reviewing ? label('Question') : '';
    element('[data-dictation-actions]').hidden = !reviewing;
    element('[data-dictation-confirm]').textContent = label('Confirm');
    element('[data-dictation-discard]').textContent = label('Discard');
  }

  function clearTimers() {
    if (!timers) return;
    deps.timers.clearTimeout(timers.limit);
    deps.timers.clearInterval(timers.tick);
    timers = null;
  }

  // Shows the recording for the user to listen to; nothing is sent yet.
  function review(blob) {
    recording = blob;
    url = deps.createUrl(blob);
    element('[data-dictation-audio]').src = url;
    state = REVIEW;
    message = '';
    render();
  }

  // Forgets the recording and its URL and returns to idle. The microphone was released when the recording stopped.
  function finish() {
    if (url) deps.revokeUrl(url);
    url = null;
    recording = null;
    message = '';
    state = IDLE;
    element('[data-dictation-audio]').removeAttribute('src');
    render();
  }

  async function start() {
    if (state !== IDLE || starting) return undefined;
    message = '';
    if (!deps.supported()) {
      message = label('Unsupported');
      return render();
    }
    starting = true;
    let opened;
    try {
      opened = await deps.record();
    } catch {
      message = label('Denied');
      return render();
    } finally {
      starting = false;
    }
    session = opened;
    startedAt = deps.now();
    state = RECORDING;
    timers = {
      limit: deps.timers.setTimeout(() => stop(), MAX_RECORDING_MS),
      tick: deps.timers.setInterval(render, 1000),
    };
    return render();
  }

  async function stop() {
    if (state !== RECORDING) return;
    const finished = session;
    session = null;
    clearTimers();
    state = STOPPING;
    render();
    let blob = null;
    try {
      blob = await finished.stop();
    } catch {
      blob = null;
    } finally {
      finished.release();
    }
    if (!blob || blob.size === 0) {
      message = label('Error');
      state = IDLE;
      return render();
    }
    return review(blob);
  }

  async function confirm() {
    if (state !== REVIEW) return;
    state = TRANSCRIBING;
    message = '';
    render();
    let text = null;
    let failure = label('Error');
    try {
      const wav = await deps.encode(recording);
      const response = await deps.send(wav, root.dataset.dictationPath);
      const body = await response.json().catch(() => null);
      if (response.ok && typeof body?.text === 'string' && body.text.trim() !== '') {
        text = body.text;
      } else if (typeof body?.error === 'string' && body.error !== '') {
        // The server message is already translated, so it is shown as it is.
        failure = body.error;
      }
    } catch {
      // A failed conversion or request keeps the recording, so the user can retry; the generic message says so.
    }
    if (text === null) {
      message = failure;
      state = REVIEW;
      return render();
    }
    insert(text);
    return finish();
  }

  // Appends the text to the prompt and tells the composer, which updates its menus and send button.
  function insert(text) {
    const textarea = element('[data-composer-input]');
    textarea.value = appendParagraph(textarea.value, text);
    textarea.dispatchEvent(new Event('input', { bubbles: true }));
    textarea.focus();
    textarea.setSelectionRange(textarea.value.length, textarea.value.length);
  }

  return {
    /** The mic button: starts a recording when idle, stops it while recording, and does nothing while busy. */
    toggle() {
      if (state === IDLE) return start();
      if (state === RECORDING) return stop();
      return undefined;
    },
    confirm,
    discard() {
      if (state === REVIEW) finish();
    },
    get state() {
      return state;
    },
  };
}

/** Records the microphone with MediaRecorder. The stop resolves the recording and always releases the microphone. */
async function startRecorder() {
  const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
  const release = () => stream.getTracks().forEach((track) => track.stop());
  let recorder;
  try {
    recorder = new MediaRecorder(stream);
  } catch (error) {
    release();
    throw error;
  }
  const chunks = [];
  recorder.addEventListener('dataavailable', (event) => {
    if (event.data.size > 0) chunks.push(event.data);
  });
  const stopped = new Promise((resolve, reject) => {
    recorder.addEventListener('stop', () => resolve(new Blob(chunks, { type: recorder.mimeType })), { once: true });
    recorder.addEventListener('error', () => reject(new Error('the recorder failed')), { once: true });
  });
  recorder.start();
  return {
    async stop() {
      try {
        if (recorder.state !== 'inactive') recorder.stop();
        return await stopped;
      } finally {
        release();
      }
    },
    release,
  };
}

/** Decodes a recording, downmixes it and resamples it to the server rate, then encodes it as WAV. */
async function encodeRecording(blob) {
  const context = new AudioContext();
  try {
    const decoded = await context.decodeAudioData(await blob.arrayBuffer());
    const channels = Array.from({ length: decoded.numberOfChannels }, (_, index) => decoded.getChannelData(index));
    const source = context.createBuffer(1, decoded.length, decoded.sampleRate);
    source.copyToChannel(downmix(channels), 0);
    const frames = Math.max(1, Math.ceil(decoded.duration * SAMPLE_RATE));
    const offline = new OfflineAudioContext(1, frames, SAMPLE_RATE);
    const player = offline.createBufferSource();
    player.buffer = source;
    player.connect(offline.destination);
    player.start();
    const rendered = await offline.startRendering();
    return encodeWav(rendered.getChannelData(0), SAMPLE_RATE);
  } finally {
    await context.close();
  }
}

/** The collaborators of the page: the microphone, the conversion, the request, object URLs and the timers. */
function browserDeps() {
  return {
    // Without a secure context there is no microphone API, so the mic reports itself unsupported.
    supported: () => Boolean(navigator.mediaDevices?.getUserMedia) && typeof MediaRecorder === 'function',
    record: startRecorder,
    encode: encodeRecording,
    send: (wav, path) => fetch(path, { method: 'POST', headers: { 'Content-Type': 'audio/wav' }, body: wav }),
    createUrl: (blob) => URL.createObjectURL(blob),
    revokeUrl: (url) => URL.revokeObjectURL(url),
    now: () => Date.now(),
    timers: {
      setTimeout: (fn, ms) => setTimeout(fn, ms),
      clearTimeout: (handle) => clearTimeout(handle),
      setInterval: (fn, ms) => setInterval(fn, ms),
      clearInterval: (handle) => clearInterval(handle),
    },
  };
}

// The controller of each composer root. htmx replaces the root with a new element, which gets a new controller.
const controllers = new WeakMap();

const controllerOf = (root, deps) => {
  let controller = controllers.get(root);
  if (!controller) {
    controller = createDictation(root, deps);
    controllers.set(root, controller);
  }
  return controller;
};

// The documents the listeners are connected to: a second install on the same document does nothing.
const installed = new WeakSet();

/** Connects the voice dictation of the document. The page calls it once; a repeat does nothing. */
export function installDictation(doc, deps = browserDeps()) {
  if (installed.has(doc)) return;
  installed.add(doc);
  doc.addEventListener('click', (event) => {
    const target = event.target;
    const root = target?.closest?.('[data-dictation]');
    if (!root) return;
    const controller = controllerOf(root, deps);
    if (target.closest('[data-dictation-mic]')) controller.toggle();
    if (target.closest('[data-dictation-confirm]')) controller.confirm();
    if (target.closest('[data-dictation-discard]')) controller.discard();
  });
}
