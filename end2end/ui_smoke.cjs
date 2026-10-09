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
  page = await browser.newPage();
  const problems = [];
  page.on('console', m => { if (m.type() === 'error' || m.type() === 'warning') problems.push(`${m.type()}: ${m.text()}`); });
  page.on('pageerror', e => problems.push(`pageerror: ${e.message}`));
  page.on('dialog', d => d.accept());
  page.on('response', r => { if (r.status() >= 400) problems.push(`HTTP ${r.status()} ${r.url()}`); });
  const step = (s) => console.log('✔', s);

  await page.goto(BASE + '/workspaces');
  await page.waitForLoadState('networkidle');
  await page.getByLabel('Nombre').fill('ui-demo');
  await page.getByLabel(/^Ruta absoluta/).fill(process.env.SMOKE_REPO);
  await page.getByRole('button', { name: 'Registrar' }).click();
  await page.getByText(process.env.SMOKE_REPO).waitFor();
  step('workspace registrado desde la UI');

  await page.goto(BASE + '/');
  await page.waitForLoadState('networkidle');
  const runtime = page.getByLabel(/^Agente/);
  const saved = page.waitForResponse(r => r.url().includes('update_agent_settings'));
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
  await page.getByText('Tarea en backlog').first().waitFor();
  await page.getByRole('button', { name: 'Editar' }).first().click();
  await page.waitForURL(/\/tasks\/[^/]+\/edit$/);
  if (await page.getByRole('dialog').count()) throw new Error('la edición no debe abrir un diálogo');
  await page.getByRole('heading', { name: 'Editar tarea' }).waitFor();
  if ((await page.getByLabel('Prompt').inputValue()) !== 'Tarea en backlog') throw new Error('el prompt no se precargó');
  await page.getByLabel(/^Título/).fill('Tarea en backlog editada');
  await page.getByLabel('Prompt').fill('Prompt editado');
  await page.getByRole('button', { name: 'Guardar cambios' }).click();
  await page.waitForURL(url => !url.pathname.endsWith('/edit'));
  await page.getByText('Tarea en backlog editada').first().waitFor();
  step('edición en pantalla propia: precarga, guardado y vuelta');

  await page.getByRole('button', { name: 'Nueva tarea' }).click();
  await page.getByLabel(/^Workspace/).selectOption({ label: 'ui-demo (main)' });
  await page.getByLabel('Prompt').fill('Tarea desde el navegador');
  await page.getByLabel('Iniciar ahora').check();
  await page.getByRole('button', { name: 'Crear tarea' }).click();
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

  await page.getByRole('button', { name: 'Finalizar y revisar' }).click();
  await page.getByText('Pendiente de revisión').first().waitFor();
  step('finalizada: pendiente de revisión');

  await page.getByRole('button', { name: 'Ver diff' }).click();
  await page.getByText('+cambio del agente pi').waitFor();
  step('diff visible (lo escribió el binario pi)');
  await page.screenshot({ path: process.env.SMOKE_SCREENSHOT || 'ui-review.png', fullPage: true });

  await page.getByText('Acciones', { exact: true }).click();
  await page.getByRole('button', { name: 'Merge' }).click();
  await page.getByText('Integrada en la rama base.').waitFor();
  step('merge desde la UI');

  await page.goto(BASE + '/');
  await page.waitForLoadState('networkidle');
  await page.getByLabel('Mostrar terminadas').check();
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
