// Smoke: cambiar de pestaña del dock no reinicia las pantallas (texto sin guardar en «Editar
// tarea» y en el formulario de Workspaces). Se ejecuta contra el servidor de end2end/run_smoke.sh.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const BASE = process.env.SMOKE_BASE_URL || 'http://127.0.0.1:3000';
let page;
const assert = (cond, msg) => { if (!cond) throw new Error(msg); };
(async () => {
  const browser = await chromium.launch(
    process.env.SMOKE_CHROMIUM_PATH ? { executablePath: process.env.SMOKE_CHROMIUM_PATH } : {},
  );
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  // Sin cookie de idioma la app pide elegirlo; los textos de este smoke están en español.
  await context.addCookies([{ name: 'iw-lang', value: 'es', url: BASE }]);
  page = await context.newPage();
  const problems = [];
  page.on('console', m => {
    if ((m.type() === 'error' || m.type() === 'warning') &&
        !/reading a resource in `hydrate`/.test(m.text()) &&
        !/htmx:(afterRequest|sendAbort)/.test(m.text())) {
      problems.push(`${m.type()}: ${m.text()}`);
    }
  });
  page.on('pageerror', e => problems.push(`pageerror: ${e.message}`));
  page.on('response', r => { if (r.status() >= 400) problems.push(`HTTP ${r.status()} ${r.url()}`); });
  const step = (s) => console.log('✔', s);
  const tab = (name) => page.locator('.dock-tab', { hasText: name }).first();
  const nav = (name) => page.getByRole('navigation', { name: 'Navegación' }).getByRole('link', { name });
  // Un solo panel visible a la vez: los demás siguen montados pero ocultos.
  const visible = (loc) => loc.locator('visible=true').first();

  await page.goto(BASE + '/workspaces');
  await page.waitForLoadState('networkidle');
  await page.getByLabel('Nombre').fill('tab-state-demo');
  await page.getByLabel(/^Ruta absoluta/).fill(process.env.SMOKE_REPO);
  await page.getByRole('button', { name: 'Registrar' }).click();
  await page.getByText(process.env.SMOKE_REPO).waitFor();
  step('workspace registrado desde la UI');

  // Texto sin enviar en el formulario de Workspaces sobrevive a un cambio de pestaña.
  await visible(page.getByLabel('Nombre')).fill('borrador sin guardar');
  await nav('Tablero').click();
  await tab('Tablero').waitFor();
  await tab('Workspaces').click();
  assert((await visible(page.getByLabel('Nombre')).inputValue()) === 'borrador sin guardar',
    'el formulario de Workspaces se reinició al cambiar de pestaña');
  step('el formulario de Workspaces conserva el texto al cambiar de pestaña');

  // Tarea en backlog para editarla.
  await tab('Tablero').click();
  await page.locator('aside').getByRole('link', { name: /tab-state-demo/ }).first().click();
  await page.getByRole('button', { name: 'Nueva tarea' }).click();
  await page.getByLabel(/^Workspace/).selectOption({ label: 'tab-state-demo (main)' });
  await page.getByLabel('Prompt').fill('Prompt original');
  await page.getByRole('button', { name: 'Crear tarea' }).click();
  await page.getByText('Prompt original').locator('visible=true').first().waitFor();
  await page.getByRole('button', { name: 'Editar' }).first().click();
  await page.waitForURL(/\/tasks\/[^/]+\/edit$/);
  await page.getByRole('heading', { name: 'Editar tarea' }).waitFor();
  await page.waitForFunction(() => [...document.querySelectorAll('textarea')].some(t => t.offsetParent && t.value === 'Prompt original'));

  await page.getByLabel(/^Título/).fill('Título a medio escribir');
  await page.getByLabel('Prompt').fill('Prompt a medio escribir');
  await page.getByLabel('Opciones avanzadas').check();

  // Ida y vuelta entre pestañas sin guardar, varias veces.
  for (let i = 0; i < 3; i++) {
    await tab('Tablero').click();
    await page.getByRole('button', { name: 'Nueva tarea' }).waitFor();
    await tab('Editar tarea').click();
    await page.getByRole('heading', { name: 'Editar tarea' }).waitFor();
  }
  assert((await page.getByLabel(/^Título/).inputValue()) === 'Título a medio escribir', 'el título se reinició');
  assert((await page.getByLabel('Prompt').inputValue()) === 'Prompt a medio escribir', 'el prompt se reinició');
  assert(await page.getByLabel('Opciones avanzadas').isChecked(), 'las opciones avanzadas se reiniciaron');
  step('Editar tarea conserva título, prompt y opciones al cambiar de pestaña sin guardar');

  // Mensaje al agente (composer) de una tarea en vivo, sin enviar: sobrevive a un cambio de pestaña.
  await tab('Tablero').click();
  const runtime = visible(page.getByLabel(/^Agente/));
  // Rust posts to the update_agent_settings server function; the Go bar posts to /agent-settings/<field>.
  const saved = page.waitForResponse(r => r.url().includes('update_agent_settings') || (r.request().method() === 'POST' && /\/agent-settings\/(runtime|model|profile|thinking)$/.test(new URL(r.url()).pathname)));
  await runtime.selectOption('pi');
  await saved;
  await page.getByRole('button', { name: 'Nueva tarea' }).click();
  await visible(page.getByLabel(/^Workspace/)).selectOption({ label: 'tab-state-demo (main)' });
  await visible(page.getByLabel('Prompt')).fill('Tarea del smoke de pestañas');
  await visible(page.getByLabel('Iniciar ahora')).check();
  await page.getByRole('button', { name: 'Crear tarea' }).click();
  await page.locator('aside').getByRole('link', { name: /tab-state-demo/ }).first().click();
  await page.getByRole('link', { name: /Tarea del smoke de pestañas/ }).first().click();
  await page.waitForURL(/\/tasks\/[^/]+$/);
  await page.getByText('eco: Tarea del smoke de pestañas').waitFor();
  const composer = () => visible(page.getByPlaceholder(/Escribe un mensaje/));
  await composer().fill('mensaje sin enviar');
  for (let i = 0; i < 3; i++) {
    await tab('Workspaces').click();
    await visible(page.getByLabel('Nombre')).waitFor();
    await page.locator('.dock-tab[aria-selected="false"]', { hasText: 'Tarea del smoke de pestañas' }).first().click();
  }
  assert((await composer().inputValue()) === 'mensaje sin enviar',
    'el mensaje del composer se reinició al cambiar de pestaña');
  step('el campo de mensaje de la tarea conserva el texto al cambiar de pestaña');

  if (problems.length) { console.error('PROBLEMAS:\n' + problems.join('\n')); process.exit(1); }
  console.log('sin errores de consola');
  await browser.close();
})().catch(async e => {
  console.error('FALLO:', e.message);
  if (page) {
    console.error('URL:', page.url());
    console.error((await page.locator('body').innerText().catch(() => '')).slice(0, 1200));
    await page.screenshot({ path: '/tmp/tab-state-fail.png' }).catch(() => {});
  }
  process.exit(1);
});
