// Preloaded with `node --import ./end2end/go/dom_shim.mjs --test <file>` so browser modules can be
// unit-tested under plain Node. Node has no MutationObserver. The shim adds an inert one, and only
// when the global is missing, so a real DOM implementation is never replaced.
//
// The shim does not fire the callbacks: tests that depend on mutations must drive them explicitly.
if (typeof globalThis.MutationObserver === 'undefined') {
  globalThis.MutationObserver = class MutationObserver {
    constructor(callback) {
      this.callback = callback;
    }

    observe() {}

    disconnect() {}

    takeRecords() {
      return [];
    }
  };
}
