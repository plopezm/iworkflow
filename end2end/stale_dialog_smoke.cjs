// Smoke: un diálogo de permiso que el agente (Cursor o Claude) ya no espera porque cerró el turno no
// debe quedar pendiente en la UI ni dar «no hay un diálogo pendiente con id perm-N».
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const BASE = process.env.SMOKE_BASE_URL || 'http://127.0.0.1:3000';
// Agente a probar (`cursor` o `claude`): cada uno usa su propio workspace.
const RUNTIME = process.env.SMOKE_RUNTIME || 'cursor';
const WORKSPACE = `stale-${RUNTIME}`;
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
  const runtime = page.getByLabel(/^Agente/);
  const saved = page.waitForResponse(r => r.url().includes('update_agent_settings'));
  await runtime.selectOption(RUNTIME);
  await saved;

  await page.getByRole('button', { name: 'Nueva tarea' }).click();
  await page.getByLabel(/^Workspace/).selectOption({ label: `${WORKSPACE} (main)` });
  await page.getByLabel('Prompt').fill('abandoned');
  await page.getByLabel('Iniciar ahora').check();
  await page.getByRole('button', { name: 'Crear tarea' }).click();
  await page.getByRole('link', { name: /abandoned/ }).first().click();
  await page.waitForURL(/\/tasks\//);

  await page.getByText('turno cerrado').waitFor();
  // El agente cerró el turno: la petición de permiso ya no existe y no puede seguir visible.
  await page.getByText('Cancelar diálogo').waitFor({ state: 'detached', timeout: 10000 });
  if (await page.getByText(/no hay un diálogo pendiente/).count()) {
    throw new Error('quedó un error de diálogo pendiente en la UI');
  }
  step(`${RUNTIME}: el diálogo huérfano se cierra al terminar el turno`);
  await browser.close();
})().catch(e => { console.error('✘', e.message); process.exit(1); });
