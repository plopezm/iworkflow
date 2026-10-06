// Smoke de las teclas especiales bajo la terminal (Tab, flechas, Ctrl+C) contra un servidor real.
// Registra su propio workspace (SMOKE_REPO, un repo con README.md) y abre su terminal.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const BASE = process.env.SMOKE_BASE_URL || 'http://127.0.0.1:3000';
let page;
(async () => {
  const browser = await chromium.launch(
    process.env.SMOKE_CHROMIUM_PATH ? { executablePath: process.env.SMOKE_CHROMIUM_PATH } : {},
  );
  page = await browser.newPage();
  const problems = [];
  page.on('pageerror', e => problems.push(`pageerror: ${e.message}`));
  const step = (s) => console.log('✔', s);

  await page.goto(BASE + '/workspaces');
  await page.waitForLoadState('networkidle');
  await page.getByLabel('Nombre').fill('term-demo');
  await page.getByLabel(/^Ruta absoluta/).fill(process.env.SMOKE_REPO);
  await page.getByRole('button', { name: 'Registrar' }).click();
  await page.getByText(process.env.SMOKE_REPO).waitFor();
  await page.getByRole('link', { name: 'Abrir terminal' }).first().click();
  await page.waitForURL(/\/terminals\/[^/]+$/);
  await page.locator('.xterm-helper-textarea').waitFor({ state: 'attached' });

  const screen = page.locator('.xterm-rows');
  const key = (name) => page.getByRole('toolbar', { name: 'Teclas especiales' }).getByRole('button', { name, exact: true }).click();
  const run = async (text) => { await page.keyboard.type(text); await page.keyboard.press('Enter'); };
  const count = async (needle) => ((await screen.innerText()).split(needle).length - 1);
  const until = async (fn, what) => {
    for (let i = 0; i < 50; i++) { if (await fn()) return; await page.waitForTimeout(100); }
    throw new Error('no se cumplió: ' + what);
  };

  const toolbar = page.getByRole('toolbar', { name: 'Teclas especiales' });
  const labels = await toolbar.getByRole('button').allTextContents();
  if (labels.join('|') !== 'Tab|↑|←|→|↓|Ctrl+C') throw new Error('botones inesperados: ' + labels);
  step('hilera con los seis botones bajo la terminal');

  await page.locator('.xterm').click();
  await run('echo marca-inicial');
  await until(async () => (await count('marca-inicial')) >= 2, 'eco inicial');

  // Tab: completa `cat READ` -> `cat README.md`.
  await page.keyboard.type('cat READ');
  await key('Tab');
  await until(async () => (await screen.innerText()).includes('cat README.md'), 'Tab completa el nombre');
  await page.keyboard.press('Control+U');
  step('Tab completa en la shell');

  // ↑ recupera el último comando.
  await key('↑');
  await until(async () => (await count('echo marca-inicial')) >= 2, '↑ recupera el historial');
  await page.keyboard.press('Control+U');
  step('↑ recupera el historial');

  // ← y →: `ab` -> cursor antes de `a`, tras `a`, y se inserta X entre ambas.
  await page.keyboard.type('echo ab');
  await key('←'); await key('←'); await key('→');
  await page.keyboard.type('X');
  await page.keyboard.press('Enter');
  await until(async () => (await count('aXb')) >= 2, '← y → mueven el cursor');
  step('← y → mueven el cursor');

  // ↓ tras ↑↑ vuelve al comando más reciente.
  await run('echo uno-fin');
  await run('echo dos-fin');
  await key('↑'); await key('↑'); await key('↓');
  await page.keyboard.press('Enter');
  await until(async () => (await count('dos-fin')) >= 4, '↓ avanza en el historial');
  step('↓ avanza en el historial');

  // Ctrl+C interrumpe un proceso en curso y devuelve el prompt.
  await run('sleep 30');
  await page.waitForTimeout(300);
  await key('Ctrl+C');
  await run('echo tras-ctrl-c');
  await until(async () => (await count('tras-ctrl-c')) >= 2, 'Ctrl+C devuelve el prompt');
  step('Ctrl+C interrumpe el proceso');

  if (problems.length) { console.error('PROBLEMAS:\n' + problems.join('\n')); process.exit(1); }
  await browser.close();
})().catch(async e => {
  console.error('FALLO:', e.message);
  if (page) console.error((await page.locator('.xterm-rows').innerText().catch(() => '')).slice(0, 1500));
  process.exit(1);
});
