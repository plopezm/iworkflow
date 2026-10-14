// Smoke del dock (pestañas, divisiones, arrastre y persistencia) con Playwright contra el
// servidor lanzado por end2end/run_smoke.sh. No necesita datos: usa solo pantallas fijas.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const BASE = process.env.SMOKE_BASE_URL || 'http://127.0.0.1:3000';
let page;
const assert = (cond, msg) => { if (!cond) throw new Error(msg); };
(async () => {
  const browser = await chromium.launch(
    process.env.SMOKE_CHROMIUM_PATH ? { executablePath: process.env.SMOKE_CHROMIUM_PATH } : {},
  );
  page = await browser.newPage({ viewport: { width: 1440, height: 860 } });
  const problems = [];
  // Ruido ajeno al dock: favicon, avisos de recursos de páginas heredadas y el catálogo de
  // modelos (necesita el binario del agente, ausente en el servidor del smoke).
  const ignorable = (t) => /favicon|status of 404|status of 500|reading a resource in `hydrate`/.test(t);
  page.on('console', m => { if ((m.type() === 'error' || m.type() === 'warning') && !ignorable(m.text())) problems.push(`${m.type()}: ${m.text()}`); });
  page.on('pageerror', e => problems.push(`pageerror: ${e.message}`));
  page.on('response', r => { if (r.status() >= 400 && !/favicon|list_model_options|list_agent|list_profile/.test(r.url())) problems.push(`HTTP ${r.status()} ${r.url()}`); });
  const step = (s) => console.log('✔', s);
  const tabs = () => page.locator('.dock-tab');
  const groups = () => page.locator('.dock-group');
  const nav = (name) => page.getByRole('navigation', { name: 'Navegación' }).getByRole('link', { name });

  await page.goto(BASE + '/');
  await page.waitForLoadState('networkidle');
  await page.locator('.dock-tab', { hasText: 'Tablero' }).waitFor();
  assert((await tabs().count()) === 1, 'debe abrirse una única pestaña inicial');
  step('pestaña inicial: Tablero');

  await nav('Workspaces').click();
  await page.locator('.dock-tab', { hasText: 'Workspaces' }).waitFor();
  await nav('Workspaces').click();
  await nav('Modelos').click();
  assert((await tabs().count()) === 3, 'abrir de nuevo una ruta abierta no debe duplicar la pestaña');
  assert(new URL(page.url()).pathname === '/models', 'la URL refleja la pestaña activa');
  step('abrir pestañas desde la barra lateral, sin duplicados, URL sincronizada');

  await page.getByRole('button', { name: 'Opciones del grupo' }).click();
  await page.getByRole('button', { name: 'Mover pestaña a la derecha' }).click();
  await page.waitForFunction(() => document.querySelectorAll('.dock-group').length === 2);
  assert((await groups().nth(1).locator('.dock-tab').count()) === 1, 'la pestaña movida está en el grupo nuevo');
  assert((await groups().nth(0).locator('.dock-tab').count()) === 2, 'el grupo original conserva las otras');
  step('mover pestaña a una división a la derecha');

  const before = await groups().nth(0).boundingBox();
  const divider = page.locator('.dock-divider').first();
  const box = await divider.boundingBox();
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await page.mouse.down();
  await page.mouse.move(box.x - 200, box.y + box.height / 2, { steps: 6 });
  await page.mouse.up();
  await page.waitForTimeout(200);
  const after = await groups().nth(0).boundingBox();
  assert(after.width < before.width - 100, `el divisor debe redimensionar (${before.width} -> ${after.width})`);
  step('redimensionar divisor arrastrando');

  await page.getByRole('button', { name: 'Opciones del grupo' }).first().click();
  await page.getByRole('button', { name: 'Dividir hacia abajo' }).click();
  await page.waitForFunction(() => document.querySelectorAll('.dock-group').length === 3);
  await page.locator('.dock-group').nth(1).getByRole('link', { name: 'Terminales' }).click();
  await page.locator('.dock-tab', { hasText: 'Terminales' }).waitFor();
  step('dividir hacia abajo y abrir desde el grupo vacío');

  const source = page.locator('.dock-tab', { hasText: 'Tablero' });
  const target = groups().last().locator('.dock-tabbar');
  await source.dragTo(target);
  await page.waitForTimeout(200);
  assert((await groups().last().locator('.dock-tab', { hasText: 'Tablero' }).count()) === 1,
    'arrastrar una pestaña a otra barra la mueve');
  step('arrastrar una pestaña a otro grupo');

  const layoutBefore = await page.evaluate(() => localStorage.getItem('iwf-dock-layout'));
  await page.reload();
  await page.waitForLoadState('networkidle');
  await page.locator('.dock-tab').first().waitFor();
  assert((await groups().count()) === (JSON.parse(layoutBefore).groups.length), 'el layout se restaura tras recargar');
  assert((await tabs().count()) === JSON.parse(layoutBefore).groups.reduce((n, g) => n + g.tabs.length, 0),
    'las pestañas se restauran tras recargar');
  step('el layout persiste tras recargar');

  // Una división vacía se puede cerrar.
  const countBefore = await groups().count();
  await page.getByRole('button', { name: 'Opciones del grupo' }).first().click();
  await page.getByRole('button', { name: 'Dividir a la derecha' }).click();
  await page.waitForFunction((n) => document.querySelectorAll('.dock-group').length === n + 1, countBefore);
  const empty = page.locator('.dock-group-focused');
  await empty.getByRole('button', { name: 'Cerrar división' }).click();
  await page.waitForFunction((n) => document.querySelectorAll('.dock-group').length === n, countBefore);
  step('una división vacía se puede cerrar');

  const aside = page.locator('aside[aria-label="Barra lateral"]');
  const widthBefore = (await page.locator('main').boundingBox()).width;
  await page.getByRole('button', { name: 'Ocultar barra lateral' }).click();
  await aside.waitFor({ state: 'hidden' });
  const widthAfter = (await page.locator('main').boundingBox()).width;
  assert(widthAfter > widthBefore + 150, `ocultar la barra debe dar espacio (${widthBefore} -> ${widthAfter})`);
  step('ocultar la barra lateral en escritorio libera ancho');

  await page.reload();
  await page.waitForLoadState('networkidle');
  await page.locator('.dock-tab').first().waitFor();
  assert(await aside.isHidden(), 'la barra sigue oculta tras recargar');
  await page.getByRole('button', { name: 'Mostrar barra lateral' }).click();
  await aside.waitFor({ state: 'visible' });
  step('la preferencia persiste y se puede volver a mostrar');

  await page.screenshot({ path: process.env.SMOKE_SCREENSHOT || 'dock-review.png' });
  if (problems.length) { console.error('PROBLEMAS:\n' + problems.join('\n')); process.exit(1); }
  console.log('sin errores de consola');
  await browser.close();
})().catch(async e => {
  console.error('FALLO:', e.message);
  if (page) {
    console.error('URL:', page.url());
    console.error((await page.locator('body').innerText().catch(() => '')).slice(0, 1200));
    await page.screenshot({ path: '/tmp/dock-fail.png' }).catch(() => {});
  }
  process.exit(1);
});
