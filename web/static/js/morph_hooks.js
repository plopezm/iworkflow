// Installs the hooks that keep client-owned state (an open menu, a collapsed section, an open transcript details)
// alive across idiomorph refreshes. Both ui.js and transcript.js register through here, so the chaining contract
// (a hook returning false vetoes the attribute update; otherwise the hooks installed before it decide) lives once.

// chainMorphHook adds hook to the idiomorph attribute callback without replacing the ones already installed.
export function chainMorphHook(callbacks, hook) {
  const previous = callbacks.beforeAttributeUpdated;
  callbacks.beforeAttributeUpdated = (...args) => (hook(...args) === false ? false : previous?.(...args));
}

// registerMorphHook chains hook into the idiomorph defaults. When idiomorph is not loaded yet (a script placed after
// the module), it waits for the document to finish loading instead of skipping the hook silently.
export function registerMorphHook(hook, target = globalThis) {
  if (target.Idiomorph) {
    chainMorphHook(target.Idiomorph.defaults.callbacks, hook);
    return true;
  }
  target.document?.addEventListener?.('DOMContentLoaded', () => registerMorphHook(hook, target), { once: true });
  return false;
}
