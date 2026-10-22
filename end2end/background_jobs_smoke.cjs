// Smoke: mientras el agente espera un job en background la tarea no cuenta como «Requiere
// atención»; al terminar el job sí. El agente falso libera el job solo.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const BASE = process.env.SMOKE_BASE_URL || 'http://127.0.0.1:3000';
const WORKSPACE = 'bg-jobs-demo';
const WAIT_MS = 2500;
(async () => {
  const browser = await chromium.launch(
    process.env.SMOKE_CHROMIUM_PATH ? { executablePath: process.env.SMOKE_CHROMIUM_PATH } : {},
  );
  const context = await browser.newContext();
  await context.addCookies([{ name: 'iw-lang', value: 'es', url: BASE }]);
  const page = await context.newPage();
  const step = (s) => console.log('✔', s);

  await page.goto(BASE + '/workspaces');
  await page.waitForLoadState('networkidle');
  await page.getByLabel('Nombre').fill(WORKSPACE);
  await page.getByLabel(/^Ruta absoluta/).fill(process.env.SMOKE_REPO);
  await page.getByRole('button', { name: 'Registrar' }).click();
  await page.getByText(process.env.SMOKE_REPO).waitFor();

  await page.goto(BASE + '/');
  await page.waitForLoadState('networkidle');
  await page.getByRole('navigation', { name: 'Navegación' }).getByRole('link', { name: 'Tablero' }).click();
  await page.getByRole('button', { name: 'Nueva tarea' }).click();
  await page.getByLabel(/^Workspace/).selectOption({ label: `${WORKSPACE} (main)` });
  await page.getByLabel('Prompt').fill('jobs-wait');
  await page.getByLabel('Iniciar ahora').check();
  await page.getByRole('button', { name: 'Crear tarea' }).click();
  await page.getByRole('link', { name: /jobs-wait/ }).first().click();
  await page.waitForURL(/\/tasks\//);

  // Contador de la sección «Requieren atención» de la barra lateral.
  const attention = async () => {
    const text = await page.getByText('Requieren atención').first().locator('..').textContent();
    return Number((text.match(/(\d+)\s*$/) || [])[1]);
  };
  await page.getByText('eco: jobs-wait').waitFor();
  await page.getByText('Esperando', { exact: true }).first().waitFor();
  // El agente cerró el turno pero el job sigue: la tarea no pide atención.
  await page.waitForTimeout(WAIT_MS);
  if ((await attention()) !== 0) {
    throw new Error('la tarea pidió atención mientras esperaba un job en background');
  }
  step('con un job en background la tarea no pide atención');

  // Al terminar el job ya no queda nada en marcha: ahora sí necesita atención.
  await page.waitForFunction(() => /Requieren atención\s*[1-9]/.test(document.querySelector('aside').innerText.replace(/\n/g, ' ')), null, { timeout: 15000 });
  step('al terminar el job la tarea pide atención');
  await browser.close();
})().catch(e => { console.error('✘', e.message); process.exit(1); });
