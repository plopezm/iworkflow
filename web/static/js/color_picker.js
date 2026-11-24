// Custom color of the workspace color picker: the "+" swatch holds a native color input over a radio. Picking a color
// stores it as the radio's value and selects it, so the form submits "#rrggbb" under the same field as the palette.
// Delegated on the document, so pickers loaded later by htmx need no wiring.

const HEX = /^#[0-9a-f]{6}$/;

// applyCustomColor writes the picked color into the swatch that owns the input; returns false for a non-hex value.
export function applyCustomColor(input) {
  const swatch = input.closest('[data-ws-custom]');
  const radio = swatch?.querySelector('[data-ws-custom-radio]');
  const color = String(input.value || '').toLowerCase();
  if (!swatch || !radio || !HEX.test(color)) return false;
  radio.value = color;
  radio.checked = true;
  swatch.style.setProperty('--ws', color);
  radio.dispatchEvent(new Event('change', { bubbles: true }));
  return true;
}

export function installColorPicker(doc = globalThis.document) {
  doc?.addEventListener?.('input', (event) => {
    const target = event.target;
    if (target?.matches?.('[data-ws-custom-input]')) applyCustomColor(target);
  });
}

installColorPicker();
