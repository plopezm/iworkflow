// Smoke de UI con Playwright contra un servidor lanzado por end2end/run_smoke.sh.
// Recorre: alta de workspace, tarea en vivo, diálogo, mensaje, finalizar, diff y merge.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const BASE = process.env.SMOKE_BASE_URL || 'http://127.0.0.1:3000';
(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage();
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

  await page.getByRole('link', { name: 'Tareas' }).click();
  await page.getByLabel(/^Workspace/).selectOption({ label: 'ui-demo (main)' });
  await page.getByLabel('Prompt').fill('Tarea desde el navegador');
  await page.getByRole('button', { name: 'Lanzar tarea' }).click();
  await page.waitForURL(/\/tasks\//);
  step('tarea lanzada y navegación al detalle');

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
  await page.getByText('+cambio del agente').waitFor();
  step('diff visible');
  await page.screenshot({ path: process.env.SMOKE_SCREENSHOT || 'ui-review.png', fullPage: true });

  await page.getByRole('button', { name: 'Merge' }).click();
  await page.getByText('Integrada en la rama base.').waitFor();
  step('merge desde la UI');

  await page.goto(BASE + '/');
  await page.getByRole('cell', { name: 'Integrada' }).waitFor();
  step('dashboard muestra la tarea integrada');

  if (problems.length) { console.error('PROBLEMAS:\n' + problems.join('\n')); process.exit(1); }
  console.log('sin errores de consola');
  await browser.close();
})().catch(e => { console.error('FALLO:', e.message); process.exit(1); });
