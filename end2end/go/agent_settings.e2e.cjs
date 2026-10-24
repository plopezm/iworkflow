// end2end/go/agent_settings.e2e.cjs
// Go port of the agent-settings part of end2end/settings.e2e.cjs (T4, phase 2).
//
// Scope: the agent bar on /settings, which is the agent_settings module's UI. The preferences cases
// of the original script (language, network access, listen port, access token, restart) belong to the
// preferences module (T2) and are NOT ported here.
//
// Selectors are identical to the Rust version: each selector is a <label class="select-pill"> with a
// visually hidden name ("Agente", "Modelo", "Razonamiento", "Perfil") and a native <select> inside.
// The bar is loaded by htmx after page load, so the tests wait for the footer text first.
//
// Differences from the Rust script: none in the assertions. The Go bar also replaces itself after a
// change (hx-swap outerHTML), so every locator below is resolved again after each change.

// ---- helpers ----

// Counts the htmx swaps of the bar and their settles. htmx binds the controls of a swapped-in bar only
// when it settles it (a short delay after the swap), so a change made in that gap is not sent.
const BAR_SETTLE_SCRIPT = `
  window.__iwfBarSwaps = 0;
  window.__iwfBarSettles = 0;
  const inBar = (e) => {
    const el = (e.detail && e.detail.elt) || e.target;
    return !!(el && el.closest && el.closest('#agent-settings-bar'));
  };
  document.addEventListener('htmx:afterSwap', (e) => { if (inBar(e)) window.__iwfBarSwaps++; }, true);
  document.addEventListener('htmx:afterSettle', (e) => { if (inBar(e)) window.__iwfBarSettles++; }, true);
`;

// Waits until htmx has settled every swap of the bar, so its controls accept changes.
async function waitForBarSettled(page, { timeout = 10000 } = {}) {
  await page.waitForFunction(
    () => window.__iwfBarSwaps > 0 && window.__iwfBarSwaps === window.__iwfBarSettles,
    null,
    { timeout }
  );
}

// Waits for the title attribute of a pill to contain `needle`; returns the last value read.
async function waitForTitle(pill, needle, { timeout = 10000 } = {}) {
  const start = Date.now();
  let last = '';
  while (Date.now() - start < timeout) {
    try {
      last = (await pill.first().getAttribute('title')) || '';
    } catch {
      last = '';
    }
    if (last.includes(needle)) return last;
    await new Promise((r) => setTimeout(r, 200));
  }
  return last;
}

// The <label class="select-pill"> carrying the title (the current value). The inner select has none.
function pillLabel(ctx, label) {
  return ctx.page.locator('.select-pill').filter({ hasText: label }).first();
}

// The native <select> inside the pill whose accessible name is `label`.
function pillSelect(ctx, label) {
  return pillLabel(ctx, label).locator('select').first();
}

// The title (current visible value) of the pill `label`.
async function pillTitle(ctx, label) {
  return (await pillLabel(ctx, label).getAttribute('title')) || '';
}

// Navigates to /settings and waits for the bar. "Se aplica a las tareas nuevas" only appears once the
// htmx request that loads the bar has swapped it in.
async function goToSettingsBar(ctx) {
  await ctx.page.addInitScript(BAR_SETTLE_SCRIPT);
  await ctx.goto('/settings');
  await ctx.page.getByText('Se aplica a las tareas nuevas').first().waitFor({ timeout: 15000 });
  await waitForBarSettled(ctx.page);
}

// ---- tests ----
module.exports = {
  name: 'agent_settings',
  tests: [
    {
      name: 'get_agent_settings: /settings muestra la barra de ajustes (runtime / modelo / razonamiento / perfil)',
      fn: async (ctx) => {
        await goToSettingsBar(ctx);
        const agente = pillSelect(ctx, 'Agente');
        const modelo = pillSelect(ctx, 'Modelo');
        const razonamiento = pillSelect(ctx, 'Razonamiento');
        const perfil = pillSelect(ctx, 'Perfil');
        await agente.first().waitFor({ timeout: 15000 });
        await modelo.first().waitFor({ timeout: 15000 });
        await razonamiento.first().waitFor({ timeout: 15000 });
        await perfil.first().waitFor({ timeout: 15000 });
        const aTitle = await pillTitle(ctx, 'Agente');
        const mTitle = await pillTitle(ctx, 'Modelo');
        const rTitle = await pillTitle(ctx, 'Razonamiento');
        const pTitle = await pillTitle(ctx, 'Perfil');
        ctx.assert(aTitle.length > 0, `el runtime no se muestra (title="${aTitle}")`);
        ctx.assert(mTitle.length > 0, `el modelo no se muestra (title="${mTitle}")`);
        ctx.assert(rTitle.length > 0, `el razonamiento no se muestra (title="${rTitle}")`);
        ctx.assert(pTitle.length > 0, `el perfil no se muestra (title="${pTitle}")`);
        // La barra indica que los cambios afectan a tareas nuevas.
        await ctx.page.getByText('Se aplica a las tareas nuevas').first().waitFor({ timeout: 8000 });
      },
    },
    {
      name: 'update_agent_settings: cambiar el nivel de razonamiento y guardar se refleja en la barra',
      fn: async (ctx) => {
        await goToSettingsBar(ctx);
        const razonamiento = pillSelect(ctx, 'Razonamiento');
        await razonamiento.first().waitFor({ timeout: 15000 });
        const antes = await pillTitle(ctx, 'Razonamiento');
        ctx.assert(antes.length > 0, `razonamiento inicial vacío (title="${antes}")`);
        // Elige un nivel DISTINTO del actual para garantizar una guardación real.
        const opts = await razonamiento.evaluate((sel) =>
          Array.from(sel.options).map((o) => ({ v: o.value, t: o.text }))
        );
        const original = await razonamiento.inputValue();
        ctx.assert(opts.length >= 2, `el select de razonamiento necesita >=2 opciones (n=${opts.length})`);
        const target = opts.find((o) => o.v !== original);
        ctx.assert(target, `no hay un nivel distinto del actual (${original})`);
        await razonamiento.selectOption({ value: target.v });
        const despues = await waitForTitle(pillLabel(ctx, 'Razonamiento'), target.t, { timeout: 15000 });
        ctx.assert(
          despues.includes(target.t),
          `el nivel de razonamiento no cambió a "${target.t}" (title="${despues}")`
        );
        ctx.assert(despues !== antes, `la barra no reflejó el cambio (antes="${antes}")`);
        // Restaura el valor original para no dejar el estado modificado.
        await waitForBarSettled(ctx.page);
        await pillSelect(ctx, 'Razonamiento').selectOption({ value: original });
        const restaurado = await waitForTitle(pillLabel(ctx, 'Razonamiento'), antes, { timeout: 15000 });
        ctx.assert(
          restaurado.includes(antes),
          `no se restauró el razonamiento a "${antes}" (title="${restaurado}")`
        );
      },
    },
    {
      name: 'list_model_options: el select Modelo tiene al menos una opción',
      fn: async (ctx) => {
        await goToSettingsBar(ctx);
        const modelo = pillSelect(ctx, 'Modelo');
        await modelo.first().waitFor({ timeout: 15000 });
        const n = await modelo.evaluate((sel) => sel.options.length);
        ctx.assert(n >= 1, `el select Modelo no tiene opciones (${n})`);
        // En el entorno smoke el catálogo está vacío y sólo aparece "Por defecto".
      },
    },
    {
      name: 'list_profile_options: el select Perfil tiene al menos una opción',
      fn: async (ctx) => {
        await goToSettingsBar(ctx);
        const perfil = pillSelect(ctx, 'Perfil');
        await perfil.first().waitFor({ timeout: 15000 });
        const n = await perfil.evaluate((sel) => sel.options.length);
        ctx.assert(n >= 1, `el select Perfil no tiene opciones (${n})`);
      },
    },
    {
      name: 'list_thinking_levels: el select Razonamiento tiene al menos una opción',
      fn: async (ctx) => {
        await goToSettingsBar(ctx);
        const razonamiento = pillSelect(ctx, 'Razonamiento');
        await razonamiento.first().waitFor({ timeout: 15000 });
        const n = await razonamiento.evaluate((sel) => sel.options.length);
        ctx.assert(n >= 1, `el select Razonamiento no tiene opciones (${n})`);
      },
    },
    {
      name: 'list_runtime_options: el select Agente tiene al menos una opción',
      fn: async (ctx) => {
        await goToSettingsBar(ctx);
        const agente = pillSelect(ctx, 'Agente');
        await agente.first().waitFor({ timeout: 15000 });
        const n = await agente.evaluate((sel) => sel.options.length);
        ctx.assert(n >= 1, `el select Agente no tiene opciones (${n})`);
      },
    },
  ],
};
