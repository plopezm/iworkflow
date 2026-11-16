// end2end/go/engram_settings.e2e.cjs
// Go-mode e2e of the Engram server card in Configuración (odd/tasks/engram-settings.md).
// run_go.sh starts the server with IWF_ENGRAM_URL pointing at a port nothing listens on, so the address starts as
// "environment" and iworkflow never starts Engram for it. The tests only save REMOTE addresses (TEST-NET-1,
// 192.0.2.0/24), which iworkflow is never allowed to start, so no engram process can be spawned by this suite.
const fs = require('fs');
const net = require('net');
const http = require('http');
const path = require('path');

const SHOT_DIR = process.env.SMOKE_SCREENSHOT_DIR || '/tmp/iwf-e2e-screenshots';
const REMOTE = 'http://192.0.2.1:7437';

async function shot(ctx, name) {
  fs.mkdirSync(SHOT_DIR, { recursive: true });
  const file = path.join(SHOT_DIR, name);
  await ctx.page.screenshot({ path: file });
  console.log(`screenshot: ${file}`);
}

// Opens Configuración on the Engram section and returns the card locator.
async function openCard(ctx) {
  await ctx.goto('/settings');
  await ctx.page.getByRole('tab', { name: 'Engram' }).click();
  const card = ctx.page.locator('[data-engram-card]:visible');
  await card.waitFor({ timeout: 10000 });
  return card;
}

// A free loopback port, so the saved address is local and the server is allowed to start Engram for it.
function freePort() {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => {
      const { port } = server.address();
      server.close(() => resolve(port));
    });
  });
}

function launches() {
  const file = process.env.IWF_E2E_ENGRAM_LAUNCH_LOG;
  if (!file) throw new Error('IWF_E2E_ENGRAM_LAUNCH_LOG no esta definida: ejecuta el suite con end2end/go/run_go.sh');
  return fs.existsSync(file) ? fs.readFileSync(file, 'utf8').split('\n').filter(Boolean) : [];
}

function stopFakeDaemon(port) {
  return new Promise((resolve) => {
    http.get({ host: '127.0.0.1', port, path: '/_stop' }, (res) => { res.resume(); res.on('end', resolve); }).on('error', resolve);
  });
}

module.exports = {
  name: 'engram_settings',
  tests: [
    {
      name: 'tarjeta: muestra la dirección de IWF_ENGRAM_URL y avisa de que no se inicia para ella',
      fn: async (ctx) => {
        const card = await openCard(ctx);
        const value = await card.locator('#engram-url').inputValue();
        ctx.assert(/^http:\/\/127\.0\.0\.1:\d+$/.test(value), `la direccion no es la del entorno: ${value}`);
        ctx.assert((await card.innerText()).includes('IWF_ENGRAM_URL'), 'falta el aviso de IWF_ENGRAM_URL');
        ctx.assert((await card.locator('[data-engram-unreachable]').count()) === 1, 'deberia decir que Engram no responde');
        await shot(ctx, 'engram-settings-1-card.png');
      },
    },
    {
      name: 'guardar: una dirección inválida se rechaza con un mensaje y no se guarda',
      fn: async (ctx) => {
        const card = await openCard(ctx);
        const before = await card.locator('#engram-url').inputValue();
        await card.locator('#engram-url').fill('ftp://nope');
        await card.getByRole('button', { name: 'Guardar y aplicar' }).click();
        await ctx.page.getByText(/debe ser una URL http o https/).first().waitFor({ timeout: 10000 });
        const reopened = await openCard(ctx);
        ctx.assert((await reopened.locator('#engram-url').inputValue()) === before, 'se guardó una dirección inválida');
        await shot(ctx, 'engram-settings-2-invalid.png');
      },
    },
    {
      name: 'guardar: una dirección remota se aplica, no se inicia nada y "Restaurar" vuelve a la del entorno',
      fn: async (ctx) => {
        const card = await openCard(ctx);
        const env = await card.locator('#engram-url').inputValue();
        await card.locator('#engram-url').fill(REMOTE);
        await card.getByRole('button', { name: 'Guardar y aplicar' }).click();
        await ctx.page.getByText(/no se iniciará desde aquí/).first().waitFor({ timeout: 15000 });
        ctx.assert((await ctx.page.locator('[data-engram-card]:visible #engram-url').inputValue()) === REMOTE, 'no se aplicó la dirección');
        await shot(ctx, 'engram-settings-3-saved-remote.png');

        const reopened = await openCard(ctx);
        ctx.assert((await reopened.locator('#engram-url').inputValue()) === REMOTE, 'la dirección no persiste');
        await reopened.getByRole('button', { name: 'Restaurar por defecto' }).click();
        await ctx.page.locator('[data-engram-card]:visible #engram-url').waitFor({ timeout: 10000 });
        await ctx.page.waitForFunction((expected) => document.querySelector('[data-engram-card] #engram-url')?.value === expected, env, { timeout: 10000 });
        ctx.assert((await ctx.page.getByRole('button', { name: 'Restaurar por defecto' }).count()) === 0, 'sigue el boton de restaurar');
      },
    },
    {
      name: 'auto-arranque: una dirección local apagada se inicia una sola vez y si ya responde no se vuelve a iniciar',
      fn: async (ctx) => {
        const port = await freePort();
        const address = `http://127.0.0.1:${port}`;
        const before = launches().length;
        try {
          const card = await openCard(ctx);
          await card.locator('#engram-url').fill(address);
          await card.getByRole('button', { name: 'Guardar y aplicar' }).click();
          await ctx.page.getByText(/estaba apagado y se ha iniciado/).first().waitFor({ timeout: 25000 });
          await ctx.page.locator('[data-engram-card]:visible [data-engram-reachable]').waitFor({ timeout: 10000 });
          const afterStart = launches();
          ctx.assert(afterStart.length === before + 1, `se esperaba 1 arranque y hubo ${afterStart.length - before}`);
          ctx.assert(afterStart[afterStart.length - 1] === `run -d -p ${port}`, `argumentos inesperados: ${afterStart[afterStart.length - 1]}`);
          await shot(ctx, 'engram-settings-4-started.png');

          await ctx.page.getByRole('button', { name: 'Guardar y aplicar' }).click();
          await ctx.page.getByText(/ya estaba en marcha/).first().waitFor({ timeout: 15000 });
          ctx.assert(launches().length === before + 1, 'se volvio a iniciar un Engram que ya respondia');
          await shot(ctx, 'engram-settings-5-already-running.png');
        } finally {
          await stopFakeDaemon(port);
          const reopened = await openCard(ctx);
          const reset = reopened.getByRole('button', { name: 'Restaurar por defecto' });
          if (await reset.count()) await reset.click();
          await ctx.page.waitForFunction(() => !document.querySelector('[data-engram-card] button[hx-post$="/reset"]'), null, { timeout: 15000 });
        }
      },
    },
  ],
};
