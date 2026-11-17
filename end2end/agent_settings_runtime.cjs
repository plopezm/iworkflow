// Shared by the root smokes: selects the agent runtime in the agent settings bar and proves that it was saved.
// Not a suite (no .e2e.cjs / _smoke.cjs suffix), so the runners do not pick it up.
//
// The bar saves each change with a POST to /agent-settings/runtime (internal/modules/agent_settings/
// infrastructure/adapters/inbound/handlers.go). Waiting for that response is not reliable here: the page does not
// report it to Playwright. So the proof is the persisted value: GET /api/agent-settings, polled until it shows the
// runtime that was selected. A value that is already the target is changed to another runtime first, so a save
// really happens.

const SETTINGS_PATH = '/api/agent-settings';
// The runtime selector of the bar: the only select of the bar with that endpoint (the model, thinking and
// profile selectors post elsewhere, and the new-task form has its own "Agente" select, hidden or not).
const RUNTIME_SELECT = '#agent-settings-bar select[hx-post="/agent-settings/runtime"]';
const SAVE_TIMEOUT_MS = 10000;
const POLL_MS = 200;

// persistedRuntime reads the runtime the server has stored. The request shares the cookies of the page.
async function persistedRuntime(page) {
  const response = await page.request.get(new URL(SETTINGS_PATH, page.url()).toString());
  if (!response.ok()) {
    throw new Error(`leer los ajustes del agente: HTTP ${response.status()}`);
  }
  const settings = await response.json();
  return settings.runtime || '';
}

// saveRuntime selects value in the bar and waits until the stored runtime is value.
async function saveRuntime(page, value) {
  await page.locator(RUNTIME_SELECT + ':visible').selectOption(value);
  const deadline = Date.now() + SAVE_TIMEOUT_MS;
  let stored = '';
  while (Date.now() < deadline) {
    stored = await persistedRuntime(page);
    if (stored === value) return;
    await page.waitForTimeout(POLL_MS);
  }
  throw new Error(`el agente guardado es «${stored}», no «${value}»`);
}

// setAgentRuntime makes runtime the saved agent and returns once the server stores it.
async function setAgentRuntime(page, runtime) {
  const select = page.locator(RUNTIME_SELECT + ':visible');
  if ((await select.inputValue()) === runtime) {
    const other = await select.evaluate((el, target) => {
      const option = [...el.options].find((o) => o.value !== '' && o.value !== target);
      return option ? option.value : null;
    }, runtime);
    if (other === null) throw new Error(`no hay otro agente al que cambiar antes de fijar ${runtime}`);
    await saveRuntime(page, other);
  }
  await saveRuntime(page, runtime);
}

module.exports = { RUNTIME_SELECT, setAgentRuntime };
