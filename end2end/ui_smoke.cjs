// Smoke de UI con Playwright contra un servidor lanzado por end2end/run_smoke.sh.
// Recorre: alta de workspace, selección del agente `pi`, tarea en vivo, diálogo, mensaje,
// finalizar, diff (con la marca del binario `pi`) y merge.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const BASE = process.env.SMOKE_BASE_URL || 'http://127.0.0.1:3000';
let page;
(async () => {
  // SMOKE_CHROMIUM_PATH permite usar un Chromium ya instalado si la versión que espera
  // Playwright no está descargada.
  const browser = await chromium.launch(
    process.env.SMOKE_CHROMIUM_PATH ? { executablePath: process.env.SMOKE_CHROMIUM_PATH } : {},
  );
  const context = await browser.newContext();
  // Sin cookie de idioma la app pide elegirlo; los textos de este smoke están en español.
  await context.addCookies([{ name: 'iw-lang', value: 'es', url: BASE }]);
  page = await context.newPage();
  const problems = [];
  page.on('console', m => { if ((m.type() === 'error' || m.type() === 'warning') && !/reading a resource in `hydrate`/.test(m.text())) problems.push(`${m.type()}: ${m.text()}`); });
  page.on('pageerror', e => problems.push(`pageerror: ${e.message}`));
  page.on('dialog', d => d.accept());
  page.on('response', r => { if (r.status() >= 400) problems.push(`HTTP ${r.status()} ${r.url()}`); });
  const step = (s) => console.log('✔', s);
  // Los smokes comparten BD: se abre el tablero de este workspace, no el primero de la lista.
  const showDemoBoard = () => page.locator('aside').getByRole('link', { name: /ui-demo/ }).first().click();

  await page.goto(BASE + '/workspaces');
  await page.waitForLoadState('networkidle');
  await page.getByLabel('Nombre').fill('ui-demo');
  await page.getByLabel(/^Ruta absoluta/).fill(process.env.SMOKE_REPO);
  await page.getByRole('button', { name: 'Registrar' }).click();
  await page.getByText(process.env.SMOKE_REPO).waitFor();
  step('workspace registrado desde la UI');

  await page.goto(BASE + '/');
  await page.waitForLoadState('networkidle');
  // El dock arranca vacío: el tablero se abre desde la barra lateral.
  await page.getByRole('navigation', { name: 'Navegación' }).getByRole('link', { name: 'Tablero' }).click();
  const runtime = page.getByLabel(/^Agente/);
  // Rust posts to the update_agent_settings server function; the Go bar posts to /agent-settings/<field>.
  const saved = page.waitForResponse(r => r.url().includes('update_agent_settings') || (r.request().method() === 'POST' && /\/agent-settings\/(runtime|model|profile|thinking)$/.test(new URL(r.url()).pathname)));
  await runtime.selectOption('pi');
  await saved;
  await page.reload();
  await page.waitForLoadState('networkidle');
  if ((await runtime.inputValue()) !== 'pi') throw new Error('el agente pi no persistió');
  if (!(await page.getByLabel(/^Perfil/).count())) throw new Error('el perfil debería seguir visible con pi');
  step('agente pi seleccionado y persistido (perfil visible)');

  // Edición en pantalla propia (no en diálogo): crear en Backlog, editar, guardar y volver.
  await page.getByRole('button', { name: 'Nueva tarea' }).click();
  await page.getByLabel(/^Workspace/).selectOption({ label: 'ui-demo (main)' });
  await page.getByLabel('Prompt').fill('Tarea en backlog');
  await page.getByRole('button', { name: 'Crear tarea' }).click();
  await showDemoBoard();
  await page.getByText('Tarea en backlog').locator('visible=true').first().waitFor();
  await page.getByRole('button', { name: 'Editar' }).first().click();
  await page.waitForURL(/\/tasks\/[^/]+\/edit$/);
  if (await page.getByRole('dialog').count()) throw new Error('la edición no debe abrir un diálogo');
  await page.getByRole('heading', { name: 'Editar tarea' }).waitFor();
  if ((await page.getByLabel('Prompt').inputValue()) !== 'Tarea en backlog') throw new Error('el prompt no se precargó');
  await page.getByLabel(/^Título/).fill('Tarea en backlog editada');
  await page.getByLabel('Prompt').fill('Prompt editado');
  await page.getByRole('button', { name: 'Guardar cambios' }).click();
  await page.waitForURL(url => !url.pathname.endsWith('/edit'));
  await page.getByText('Tarea en backlog editada').locator('visible=true').first().waitFor();
  step('edición en pantalla propia: precarga, guardado y vuelta');

  await page.getByRole('button', { name: 'Nueva tarea' }).click();
  await page.getByLabel(/^Workspace/).selectOption({ label: 'ui-demo (main)' });
  await page.getByLabel('Prompt').fill('Tarea desde el navegador');
  await page.getByLabel('Iniciar ahora').check();
  await page.getByRole('button', { name: 'Crear tarea' }).click();
  await showDemoBoard();
  await page.getByRole('link', { name: /Tarea desde el navegador/ }).first().click();
  await page.waitForURL(/\/tasks\//);
  step('tarea creada desde el tablero y navegación al detalle');

  await page.getByText('eco: Tarea desde el navegador').waitFor();
  await page.getByText('en vivo').waitFor();
  step('transcript en vivo por SSE');

  await page.getByRole('heading', { name: '❓ ¿Seguir?' }).waitFor();
  await page.getByRole('button', { name: 'Sí' }).click();
  await page.getByText('respuesta {"confirmed": true').waitFor();
  step('diálogo respondido');

  await page.getByPlaceholder(/Escribe un mensaje/).fill('otra cosa');
  await page.getByRole('button', { name: 'Enviar' }).click();
  await page.getByText('eco: otra cosa').waitFor();
  step('mensaje de seguimiento');

  // Las pestañas internas del panel de la tarea conservan su estado al cambiar de una a otra.
  const panes = () => page.getByRole('tablist').filter({ has: page.getByRole('tab', { name: 'Archivos' }) })
    .locator('xpath=following-sibling::div[1]').locator('> div');
  await page.locator('details.prompt summary').click();
  await page.getByRole('tab', { name: 'Archivos' }).click();
  await panes().nth(1).locator('> *').first().waitFor();
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(500);
  await panes().nth(1).locator('> *').first().evaluate(el => { el.dataset.keep = '1'; });
  await page.getByRole('tab', { name: 'Acciones' }).click();
  if (!(await page.locator('details.prompt').evaluate(el => el.open))) throw new Error('la pestaña Acciones pierde su estado al cambiar');
  await page.getByRole('tab', { name: 'Archivos' }).click();
  if ((await panes().nth(1).locator('> [data-keep="1"]').count()) !== 1) throw new Error('la pestaña Archivos se reconstruye al cambiar');
  await page.getByRole('tab', { name: 'Acciones' }).click();
  step('las pestañas del panel de la tarea conservan su estado');

  await page.getByRole('button', { name: 'Finalizar y revisar' }).click();
  await page.getByText('Pendiente de revisión').first().waitFor();
  step('finalizada: pendiente de revisión');

  await page.getByText('Acciones', { exact: true }).click();
  await page.getByRole('button', { name: 'Ver diff' }).click();
  await page.locator('.dock-tab', { hasText: /^.*Diff/ }).waitFor();
  if (new URL(page.url()).pathname.endsWith('/diff') === false) throw new Error('el diff debe abrirse en su propia pestaña');
  await page.getByText('cambio del agente pi').waitFor();
  step('diff visible (lo escribió el binario pi)');
  await page.screenshot({ path: process.env.SMOKE_SCREENSHOT || 'ui-review.png', fullPage: true });
  await page.locator('.dock-tab[title="Tarea desde el navegador"]').click();

  await page.getByText('Acciones', { exact: true }).click();
  if (process.env.SMOKE_ACTIONS_SCREENSHOT) await page.screenshot({ path: process.env.SMOKE_ACTIONS_SCREENSHOT });
  const pagesBefore = page.context().pages().length;
  await page.getByRole('link', { name: 'Abrir terminal' }).click();
  await page.waitForURL(/\/terminals\/[^/]+$/);
  if (page.context().pages().length !== pagesBefore) throw new Error('abrir terminal no debe abrir otra ventana');
  await page.locator('.dock-tab', { hasText: /Terminal/ }).waitFor();
  await page.locator('.dock-tab[title="Tarea desde el navegador"]').click();
  step('abrir terminal de la tarea abre una pestaña (sin ventana nueva)');

  await page.getByText('Acciones', { exact: true }).click();
  await page.getByRole('button', { name: 'Merge' }).click();
  await page.getByText('Integrada en la rama base.').waitFor();
  step('merge desde la UI');

  await page.goto(BASE + '/');
  await page.waitForLoadState('networkidle');
  // El dock arranca vacío: el tablero se abre desde la barra lateral.
  await page.getByRole('navigation', { name: 'Navegación' }).getByRole('link', { name: 'Tablero' }).click();
  await showDemoBoard();
  await page.getByRole('button', { name: /Mostrar terminadas/ }).click();
  await page.getByText('Hecho', { exact: true }).first().waitFor();
  step('el tablero muestra la tarea integrada');

  if (problems.length) { console.error('PROBLEMAS:\n' + problems.join('\n')); process.exit(1); }
  console.log('sin errores de consola');
  await browser.close();
})().catch(async e => {
  console.error('FALLO:', e.message);
  if (page) {
    console.error('URL:', page.url());
    console.error('botones:', JSON.stringify(await page.getByRole('button').allInnerTexts().catch(() => [])));
    console.error((await page.locator('body').innerText().catch(() => '')).slice(0, 1500));
  }
  process.exit(1);
});
