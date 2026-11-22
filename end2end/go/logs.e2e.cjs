// end2end/go/logs.e2e.cjs
// The log card of Settings -> Resources (module logs): the most recent lines of the server log, newest first,
// refreshed by itself with a morph of its content. The e2e server writes <IWF_HOME>/logs/iworkflow.log.
// Screenshots: SMOKE_SCREENSHOT_DIR (set by run_go.sh; default /tmp/iwf-e2e-screenshots).
const fs = require('fs');
const path = require('path');

// Mirrors of the product values: application.MaxLines (lines shown), the max-h-96 class of the log block
// (384 px, plus slack) and the card refresh interval (every 5 s, see props.go). Update them together.
const MAX_LINES = 500;
const MAX_BLOCK_PX = 400;
const MIN_LINES_TO_OVERFLOW = 30;
const REFRESH_WAIT_MS = 12000;
const SHOTS = process.env.SMOKE_SCREENSHOT_DIR || '/tmp/iwf-e2e-screenshots';

async function openResources(ctx) {
  await ctx.goto('/settings/resources');
  await ctx.page.locator('#logs-card').waitFor({ timeout: 15000 });
}

module.exports = {
  name: 'logs',
  tests: [
    {
      name: 'logs: la tarjeta Logs aparece en Configuración > Recursos, después de Límites de recursos',
      fn: async (ctx) => {
        await openResources(ctx);
        await ctx.page.getByText('Límites de recursos').first().waitFor({ timeout: 15000 });
        const order = await ctx.page.evaluate(() => {
          const titles = Array.from(document.querySelectorAll('.card-title')).map((el) => el.textContent.trim());
          return { limits: titles.findIndex((t) => t.includes('Límites de recursos')), logs: titles.findIndex((t) => t === 'Logs') };
        });
        ctx.assert(order.limits >= 0 && order.logs >= 0, `faltan títulos de tarjeta: ${JSON.stringify(order)}`);
        ctx.assert(order.logs > order.limits, `la tarjeta de logs debe ir después: ${JSON.stringify(order)}`);
      },
    },
    {
      name: 'logs: muestra líneas del log del servidor, la más reciente arriba',
      fn: async (ctx) => {
        await openResources(ctx);
        const lines = ctx.page.locator('#logs-card #logs-lines > span');
        await lines.first().waitFor({ timeout: 15000 });
        const count = await lines.count();
        ctx.assert(count > 0, 'el log del servidor tiene al menos una línea');
        ctx.assert(count <= MAX_LINES, `no se muestran más de ${MAX_LINES} líneas (hay ${count})`);
        const text = (await lines.first().textContent()).trim();
        ctx.assert(text.length > 0, 'la primera línea no está vacía');
        const box = await ctx.page.locator('#logs-lines').evaluate((el) => ({ overflowY: getComputedStyle(el).overflowY, client: el.clientHeight, scroll: el.scrollHeight }));
        ctx.assert(box.overflowY === 'auto' || box.overflowY === 'scroll', `el log tiene scroll vertical (${JSON.stringify(box)})`);
        ctx.assert(box.client <= MAX_BLOCK_PX, `el log tiene altura acotada (${JSON.stringify(box)})`);
        if (count > MIN_LINES_TO_OVERFLOW) ctx.assert(box.scroll > box.client, `con ${count} líneas el contenido desborda y se puede desplazar (${JSON.stringify(box)})`);
        fs.mkdirSync(SHOTS, { recursive: true });
        const shot = path.join(SHOTS, 'logs-card.png');
        await ctx.page.locator('#logs-card').scrollIntoViewIfNeeded();
        await ctx.page.screenshot({ path: shot, fullPage: true });
        console.log('screenshot: ' + shot);
      },
    },
    {
      name: 'logs: la tarjeta se refresca sola y conserva su contenedor en la página',
      fn: async (ctx) => {
        await openResources(ctx);
        const before = await ctx.page.evaluate(() => document.querySelector('#logs-card').dataset.probe || '');
        ctx.assert(before === '', 'sin marca previa');
        await ctx.page.evaluate(() => { document.querySelector('#logs-card').dataset.probe = 'kept'; });
        await ctx.page.waitForResponse((r) => r.url().endsWith('/logs/card') && r.ok(), { timeout: REFRESH_WAIT_MS });
        const kept = await ctx.page.evaluate(() => document.querySelector('#logs-card').dataset.probe);
        ctx.assert(kept === 'kept', 'el contenedor de la tarjeta permanece en la página (solo se morfea su contenido)');
      },
    },
    {
      name: 'logs: el fragmento /logs/card es el contenido de la tarjeta, sin el contenedor',
      fn: async (ctx) => {
        const res = await ctx.page.request.get(ctx.base + '/logs/card');
        ctx.assert(res.ok(), `status ${res.status()}`);
        const body = await res.text();
        ctx.assert(body.includes('Logs'), 'incluye el título');
        ctx.assert(!body.includes('id="logs-card"'), 'no repite el contenedor que se refresca');
      },
    },
  ],
};
