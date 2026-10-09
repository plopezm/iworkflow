// Smoke de la reconexión del terminal: al morir el WebSocket (p. ej. el navegador móvil lo
// mata al minimizar) la sesión debe reanudarse sin `[session closed]` ni scrollback duplicado.
// Registra su propio workspace (SMOKE_REPO, un repo con README.md) y abre su terminal.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const BASE = process.env.SMOKE_BASE_URL || 'http://127.0.0.1:3000';
(async () => {
  const browser = await chromium.launch(
    process.env.SMOKE_CHROMIUM_PATH ? { executablePath: process.env.SMOKE_CHROMIUM_PATH } : {},
  );
  const page = await browser.newPage();
  const problems = [];
  page.on('pageerror', e => problems.push(`pageerror: ${e.message}`));
  const step = (s) => console.log('✔', s);

  // Registra cada WebSocket creado para poder matarlo desde el test.
  await page.addInitScript(() => {
    const Native = window.WebSocket;
    window.__sockets = [];
    window.WebSocket = function (...args) {
      const ws = new Native(...args);
      window.__sockets.push(ws);
      return ws;
    };
    Object.assign(window.WebSocket, Native);
    window.WebSocket.prototype = Native.prototype;
  });

  await page.goto(BASE + '/workspaces');
  await page.waitForLoadState('networkidle');
  await page.getByLabel('Nombre').fill('reconnect-demo');
  await page.getByLabel(/^Ruta absoluta/).fill(process.env.SMOKE_REPO);
  await page.getByRole('button', { name: 'Registrar' }).click();
  await page.getByText(process.env.SMOKE_REPO).waitFor();
  await page.getByRole('link', { name: 'Abrir terminal' }).first().click();
  await page.waitForURL(/\/terminals\/[^/]+$/);
  await page.locator('.xterm-helper-textarea').waitFor({ state: 'attached' });

  const screen = page.locator('.xterm-rows');
  const run = async (text) => { await page.keyboard.type(text); await page.keyboard.press('Enter'); };
  const count = async (needle) => ((await screen.innerText()).split(needle).length - 1);
  const until = async (fn, what) => {
    for (let i = 0; i < 80; i++) { if (await fn()) return; await page.waitForTimeout(100); }
    throw new Error('no se cumplió: ' + what);
  };

  await page.locator('.xterm').click();
  await run('echo marca-previa');
  await until(async () => (await count('marca-previa')) >= 2, 'eco inicial');

  // Muere el socket como al minimizar la app: la sesión del servidor sigue viva.
  await page.evaluate(() => window.__sockets.forEach(ws => ws.close()));
  await until(
    async () => (await page.evaluate(() => window.__sockets.length)) >= 2,
    'se abre un segundo WebSocket',
  );
  step('el cliente reconecta tras caer el socket');

  // Scrollback reenviado sin duplicar (comando + salida = 2 apariciones) y sin aviso de cierre.
  await until(async () => (await count('marca-previa')) === 2, 'scrollback sin duplicar');
  if ((await screen.innerText()).includes('[session closed]')) throw new Error('apareció [session closed]');
  step('scrollback restaurado sin duplicados ni [session closed]');

  // La sesión reconectada sigue siendo la misma shell y acepta entrada.
  await run('echo marca-posterior');
  await until(async () => (await count('marca-posterior')) >= 2, 'entrada tras reconectar');
  step('la shell responde tras reconectar');

  await browser.close();
  if (problems.length) { console.error('PROBLEMAS:\n' + problems.join('\n')); process.exit(1); }
})().catch(e => { console.error('FALLO:', e.message); process.exit(1); });
