// Run with: node --test web/static/js/
// The voice dictation of a task prompt: the pure helpers (paragraph insertion, downmix, WAV encoding, elapsed time) and
// the flow of the controller, driven through small stand-ins for the DOM and for the recorder, the decoder and fetch.
import test from 'node:test';
import assert from 'node:assert/strict';
import { appendParagraph, createDictation, downmix, encodeWav, formatElapsed, frameCount, installDictation, MAX_RECORDING_MS, SAMPLE_RATE } from './dictation.js';

// A DOM element stand-in: only the properties and methods the controller reads or writes.
function fakeElement() {
  const attributes = {};
  return {
    hidden: false,
    disabled: false,
    textContent: '',
    value: '',
    src: '',
    focused: false,
    selection: null,
    events: [],
    dataset: {},
    setAttribute(name, value) {
      attributes[name] = String(value);
    },
    getAttribute(name) {
      return attributes[name] ?? null;
    },
    removeAttribute(name) {
      delete attributes[name];
    },
    focus() {
      this.focused = true;
    },
    setSelectionRange(start, end) {
      this.selection = [start, end];
    },
    dispatchEvent(event) {
      this.events.push(event.type);
      return true;
    },
  };
}

const SELECTORS = [
  '[data-dictation-mic]',
  '[data-dictation-elapsed]',
  '[data-dictation-panel]',
  '[data-dictation-status]',
  '[data-dictation-audio]',
  '[data-dictation-question]',
  '[data-dictation-actions]',
  '[data-dictation-confirm]',
  '[data-dictation-discard]',
  '[data-composer-input]',
];

// The composer root: its dataset carries the path and the translated labels, as the templ renders them.
function fakeRoot(prompt = '') {
  const parts = Object.fromEntries(SELECTORS.map((selector) => [selector, fakeElement()]));
  parts['[data-composer-input]'].value = prompt;
  return {
    dataset: {
      dictationPath: '/transcriptions',
      labelRecord: 'Record audio',
      labelStop: 'Stop recording',
      labelRecording: 'Recording',
      labelQuestion: 'Is the recording OK?',
      labelConfirm: 'Transcribe',
      labelDiscard: 'Discard',
      labelTranscribing: 'Transcribing',
      labelUnsupported: 'Recording is not available here',
      labelDenied: 'Microphone access was denied',
      labelError: 'The recording could not be transcribed',
    },
    parts,
    querySelector: (selector) => parts[selector] ?? null,
  };
}

const textareaOf = (root) => root.parts['[data-composer-input]'];
const statusOf = (root) => root.parts['[data-dictation-status]'];

// A recorded blob stand-in and the session the recorder returns: stop() resolves the blob, release() stops the mic.
function fakeSession(calls, blob = { size: 3, type: 'audio/webm' }) {
  return {
    stop: async () => {
      calls.stops += 1;
      return blob;
    },
    release: () => {
      calls.releases += 1;
    },
  };
}

// The answer of the server, as far as the controller reads it: ok, status and the JSON body.
function serverAnswer(body, { ok = true, status = 200 } = {}) {
  return { ok, status, json: async () => body };
}

// Every collaborator of the controller, with the calls recorded. Overrides replace any of them.
function fakeDeps(overrides = {}) {
  const calls = { record: 0, stops: 0, releases: 0, encoded: [], sent: [], created: [], revoked: [] };
  // The auto-stop the controller schedules: the callback and the delay, as the timers stand-in records them.
  const limit = { fn: null, ms: null };
  const deps = {
    limit,
    calls,
    supported: () => true,
    record: async () => {
      calls.record += 1;
      return fakeSession(calls);
    },
    encode: async (blob) => {
      calls.encoded.push(blob);
      return new ArrayBuffer(4);
    },
    send: async (wav, path) => {
      calls.sent.push({ wav, path });
      return serverAnswer({ text: 'hola' });
    },
    createUrl: (blob) => {
      calls.created.push(blob);
      return `blob:${calls.created.length}`;
    },
    revokeUrl: (url) => {
      calls.revoked.push(url);
    },
    now: () => 0,
    timers: {
      setTimeout: (fn, ms) => {
        limit.fn = fn;
        limit.ms = ms;
        return 'limit';
      },
      clearTimeout: () => {
        limit.fn = null;
      },
      setInterval: () => 'tick',
      clearInterval: () => {},
    },
    ...overrides,
  };
  return deps;
}

test('appendParagraph gives the text alone when the prompt is empty or blank', () => {
  assert.equal(appendParagraph('', 'hola'), 'hola');
  assert.equal(appendParagraph('  \n\t ', 'hola'), 'hola');
});

test('appendParagraph starts a new paragraph after the existing text, dropping its trailing whitespace', () => {
  assert.equal(appendParagraph('uno', 'dos'), 'uno\n\ndos');
  assert.equal(appendParagraph('uno\n\n\n', 'dos'), 'uno\n\ndos');
  assert.equal(appendParagraph('uno  \t', 'dos'), 'uno\n\ndos');
});

test('appendParagraph keeps the prompt as it is when the transcription is blank', () => {
  assert.equal(appendParagraph('uno\n', ''), 'uno\n');
  assert.equal(appendParagraph('uno', '   '), 'uno');
});

test('appendParagraph trims the transcribed text so no stray blank lines appear', () => {
  assert.equal(appendParagraph('uno', '  dos \n'), 'uno\n\ndos');
});

test('downmix averages the channels and keeps a mono signal as it is', () => {
  assert.deepEqual(Array.from(downmix([Float32Array.of(0.5, -1), Float32Array.of(0.25, 0)])), [0.375, -0.5]);
  assert.deepEqual(Array.from(downmix([Float32Array.of(0.25, -0.5)])), [0.25, -0.5]);
  assert.equal(downmix([]).length, 0);
});

test('encodeWav writes a 16-bit mono PCM RIFF header at the sample rate', () => {
  const view = new DataView(encodeWav(new Float32Array(3), 16000));
  const text = (offset) => String.fromCharCode(...[0, 1, 2, 3].map((i) => view.getUint8(offset + i)));

  assert.equal(text(0), 'RIFF');
  assert.equal(view.getUint32(4, true), 36 + 6, 'RIFF size counts the header after the size field plus the data');
  assert.equal(text(8), 'WAVE');
  assert.equal(text(12), 'fmt ');
  assert.equal(view.getUint32(16, true), 16, 'PCM fmt chunk has 16 bytes');
  assert.equal(view.getUint16(20, true), 1, 'format 1 is PCM');
  assert.equal(view.getUint16(22, true), 1, 'one channel');
  assert.equal(view.getUint32(24, true), 16000, 'sample rate');
  assert.equal(view.getUint32(28, true), 32000, 'byte rate is rate x 2 bytes');
  assert.equal(view.getUint16(32, true), 2, 'block align is 2 bytes per frame');
  assert.equal(view.getUint16(34, true), 16, '16 bits per sample');
  assert.equal(text(36), 'data');
  assert.equal(view.getUint32(40, true), 6, 'data size is the samples x 2 bytes');
  assert.equal(view.byteLength, 44 + 6);
});

test('encodeWav writes the samples as little-endian int16 and clips values outside [-1, 1]', () => {
  const view = new DataView(encodeWav(Float32Array.of(0, 0.5, -0.5, 1, -1, 2, -3), 16000));
  const sample = (index) => view.getInt16(44 + index * 2, true);

  assert.equal(sample(0), 0);
  assert.equal(sample(1), 16384);
  assert.equal(sample(2), -16384);
  assert.equal(sample(3), 32767, 'full scale positive');
  assert.equal(sample(4), -32768, 'full scale negative');
  assert.equal(sample(5), 32767, 'above 1 is clipped to the positive maximum');
  assert.equal(sample(6), -32768, 'below -1 is clipped to the negative minimum');
});

test('formatElapsed shows minutes and zero-padded seconds', () => {
  assert.equal(formatElapsed(0), '0:00');
  assert.equal(formatElapsed(4999), '0:04');
  assert.equal(formatElapsed(65_000), '1:05');
});

test('confirm sends the WAV once and inserts the transcribed text as a new paragraph', async () => {
  const root = fakeRoot('primer párrafo');
  const deps = fakeDeps();
  const dictation = createDictation(root, deps);

  await dictation.toggle();
  await dictation.toggle();
  await dictation.confirm();

  assert.equal(deps.calls.sent.length, 1, 'the recording is sent once');
  assert.equal(deps.calls.sent[0].path, '/transcriptions');
  assert.equal(deps.calls.encoded.length, 1);
  assert.equal(textareaOf(root).value, 'primer párrafo\n\nhola');
  assert.deepEqual(textareaOf(root).events, ['input'], 'composer.js hears the insertion');
  assert.equal(textareaOf(root).focused, true);
  assert.deepEqual(textareaOf(root).selection, [textareaOf(root).value.length, textareaOf(root).value.length]);
  assert.equal(dictation.state, 'idle');
  assert.deepEqual(deps.calls.revoked, ['blob:1'], 'the review URL is released');
});

test('stop shows the review and sends nothing until the user confirms', async () => {
  const root = fakeRoot();
  const deps = fakeDeps();
  const dictation = createDictation(root, deps);

  await dictation.toggle();
  await dictation.toggle();

  assert.equal(dictation.state, 'review');
  assert.equal(deps.calls.sent.length, 0);
  assert.equal(deps.calls.releases, 1, 'the microphone is released when the recording stops');
  assert.equal(root.parts['[data-dictation-audio]'].src, 'blob:1');
  assert.equal(root.parts['[data-dictation-audio]'].hidden, false);
  assert.equal(root.parts['[data-dictation-question]'].textContent, 'Is the recording OK?');
  assert.equal(root.parts['[data-dictation-actions]'].hidden, false);
  assert.equal(root.parts['[data-dictation-confirm]'].textContent, 'Transcribe');
  assert.equal(root.parts['[data-dictation-discard]'].textContent, 'Discard');
});

test('discard returns to idle, sends nothing and leaves the prompt as it was', async () => {
  const root = fakeRoot('texto');
  const deps = fakeDeps();
  const dictation = createDictation(root, deps);

  await dictation.toggle();
  await dictation.toggle();
  dictation.discard();

  assert.equal(dictation.state, 'idle');
  assert.equal(deps.calls.sent.length, 0);
  assert.equal(textareaOf(root).value, 'texto');
  assert.deepEqual(deps.calls.revoked, ['blob:1']);
  assert.equal(root.parts['[data-dictation-panel]'].hidden, true);
});

test('a server error keeps the prompt intact, shows its message and lets the user retry', async () => {
  const root = fakeRoot('texto');
  let answer = serverAnswer({ error: 'The recording is too long' }, { ok: false, status: 413 });
  const deps = fakeDeps({
    send: async () => answer,
  });
  const dictation = createDictation(root, deps);

  await dictation.toggle();
  await dictation.toggle();
  await dictation.confirm();

  assert.equal(textareaOf(root).value, 'texto', 'the prompt is never lost');
  assert.equal(dictation.state, 'review', 'the recording is kept for a retry');
  assert.equal(statusOf(root).textContent, 'The recording is too long');
  assert.equal(root.parts['[data-dictation-actions]'].hidden, false);

  answer = serverAnswer({ text: 'ahora sí' });
  await dictation.confirm();
  assert.equal(textareaOf(root).value, 'texto\n\nahora sí');
  assert.equal(dictation.state, 'idle');
});

test('a failed request shows the generic error and keeps the prompt intact', async () => {
  const root = fakeRoot('texto');
  const deps = fakeDeps({
    send: async () => {
      throw new TypeError('Failed to fetch');
    },
  });
  const dictation = createDictation(root, deps);

  await dictation.toggle();
  await dictation.toggle();
  await dictation.confirm();

  assert.equal(textareaOf(root).value, 'texto');
  assert.equal(statusOf(root).textContent, 'The recording could not be transcribed');
  assert.equal(dictation.state, 'review');
});

test('an answer that is not JSON counts as an error', async () => {
  const root = fakeRoot('texto');
  const deps = fakeDeps({
    send: async () => ({ ok: true, status: 200, json: async () => Promise.reject(new SyntaxError('bad json')) }),
  });
  const dictation = createDictation(root, deps);

  await dictation.toggle();
  await dictation.toggle();
  await dictation.confirm();

  assert.equal(textareaOf(root).value, 'texto');
  assert.equal(statusOf(root).textContent, 'The recording could not be transcribed');
});

test('clicks while busy are ignored: a second start, and a second confirm while transcribing', async () => {
  const root = fakeRoot();
  const sent = [];
  let finish;
  const deps = fakeDeps({
    send: (wav) => {
      sent.push(wav);
      return new Promise((resolve) => {
        finish = resolve;
      });
    },
  });
  const dictation = createDictation(root, deps);

  const starting = dictation.toggle();
  await dictation.toggle();
  await starting;
  assert.equal(deps.calls.record, 1, 'a second click while the microphone is being opened starts nothing');

  await dictation.toggle();
  const first = dictation.confirm();
  await dictation.confirm();
  assert.equal(dictation.state, 'transcribing');
  assert.equal(root.parts['[data-dictation-mic]'].disabled, true, 'the mic is disabled while transcribing');
  assert.equal(statusOf(root).textContent, 'Transcribing');

  // The request starts after the conversion, which is a few microtasks away; a macrotask lets it reach the server stand-in.
  await new Promise((resolve) => setImmediate(resolve));
  finish(serverAnswer({ text: 'hola' }));
  await first;
  assert.equal(sent.length, 1, 'the recording is sent once');
});

test('without microphone support the unsupported label is shown and nothing is recorded', async () => {
  const root = fakeRoot();
  const deps = fakeDeps({ supported: () => false });
  const dictation = createDictation(root, deps);

  await dictation.toggle();

  assert.equal(deps.calls.record, 0);
  assert.equal(dictation.state, 'idle');
  assert.equal(statusOf(root).textContent, 'Recording is not available here');
});

test('a refused microphone shows the denied label and returns to idle', async () => {
  const root = fakeRoot();
  const deps = fakeDeps({
    record: async () => {
      throw new DOMException('denied', 'NotAllowedError');
    },
  });
  const dictation = createDictation(root, deps);

  await dictation.toggle();

  assert.equal(dictation.state, 'idle');
  assert.equal(statusOf(root).textContent, 'Microphone access was denied');
});

test('the recording stops itself at the limit', async () => {
  const root = fakeRoot();
  const deps = fakeDeps();
  const dictation = createDictation(root, deps);

  await dictation.toggle();
  assert.equal(deps.limit.ms, MAX_RECORDING_MS);
  await deps.limit.fn();

  assert.equal(dictation.state, 'review');
  assert.equal(deps.calls.sent.length, 0);
});

test('an empty recording is reported as an error and the microphone is released', async () => {
  const root = fakeRoot('texto');
  const deps = fakeDeps({
    record: async () => ({
      stop: async () => ({ size: 0, type: 'audio/webm' }),
      release: () => {
        deps.calls.releases += 1;
      },
    }),
  });
  const dictation = createDictation(root, deps);

  await dictation.toggle();
  await dictation.toggle();

  assert.equal(dictation.state, 'idle');
  assert.equal(statusOf(root).textContent, 'The recording could not be transcribed');
  assert.equal(deps.calls.releases, 1);
  assert.equal(textareaOf(root).value, 'texto');
});

test('installDictation routes the clicks of the document to the controller of the composer', async () => {
  const listeners = {};
  const doc = { addEventListener: (type, listener) => (listeners[type] = listener) };
  const root = fakeRoot('');
  const deps = fakeDeps();
  const mic = root.parts['[data-dictation-mic]'];
  const clickOn = (element) => ({
    target: {
      closest: (selector) => {
        if (selector === '[data-dictation]') return root;
        if (selector === '[data-dictation-mic]') return element === mic ? mic : null;
        return null;
      },
    },
  });

  installDictation(doc, deps);
  listeners.click(clickOn(mic));
  await new Promise((resolve) => setImmediate(resolve));

  assert.equal(deps.calls.record, 1, 'the mic click starts a recording');
  installDictation(doc, deps);
  assert.equal(deps.calls.record, 1, 'a second install on the same document does nothing');
});

test('the recording stops with a margin below the 60 s the server takes, so recorder latency never exceeds it', () => {
  assert.ok(MAX_RECORDING_MS <= 58_000);
});

test('frameCount never exceeds the 60 s the server accepts, whatever the decoded duration', () => {
  assert.equal(frameCount(1), SAMPLE_RATE);
  assert.equal(frameCount(0), 1);
  assert.equal(frameCount(60.4), 60 * SAMPLE_RATE);
});

test('a composer removed while the permission prompt is open releases the microphone it receives', async () => {
  const root = fakeRoot();
  const deps = fakeDeps();
  let grant;
  deps.record = () => new Promise((resolve) => (grant = resolve));
  const dictation = createDictation(root, deps);

  const starting = dictation.toggle();
  root.isConnected = false;
  grant(fakeSession(deps.calls));
  await starting;

  assert.equal(deps.calls.releases, 1);
  assert.equal(deps.limit.fn, null);
});

test('a composer removed while recording releases the microphone on the next tick', async () => {
  const root = fakeRoot();
  const deps = fakeDeps();
  let tick = null;
  deps.timers.setInterval = (fn) => {
    tick = fn;
    return 'tick';
  };
  const dictation = createDictation(root, deps);
  await dictation.toggle();
  assert.equal(deps.calls.releases, 0);

  root.isConnected = false;
  tick();

  assert.equal(deps.calls.releases, 1);
  assert.equal(deps.limit.fn, null);
  await dictation.toggle();
  assert.equal(deps.calls.record, 2, 'the controller is idle again after being abandoned');
});
