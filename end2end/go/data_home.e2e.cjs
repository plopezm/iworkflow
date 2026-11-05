// end2end/go/data_home.e2e.cjs
// The data folder card of Settings -> Resources (module data_home).
//
// The e2e server runs with IWF_HOME set, so the folder is fixed by the environment: the card must show it
// read-only and explain why. The real move (copy, pointer, restart, clean-up) needs a server without IWF_HOME
// and is checked in a browser against a real process by end2end/go/move_home_smoke.sh (which runs
// end2end/move_home_ui_smoke.cjs: refusal notice, restarting card, new folder shown).

async function openResources(ctx) {
  await ctx.goto('/settings/resources');
  await ctx.page.getByText('Carpeta de datos').first().waitFor({ timeout: 15000 });
}

module.exports = {
  name: 'data_home',
  tests: [
    {
      name: 'data_home: la tarjeta Carpeta de datos aparece encima de Límites de recursos',
      fn: async (ctx) => {
        await openResources(ctx);
        await ctx.page.getByText('Límites de recursos').first().waitFor({ timeout: 15000 });
        const order = await ctx.page.evaluate(() => {
          const titles = Array.from(document.querySelectorAll('h2, h3, .card-title, [data-card-title]')).map((el) => el.textContent.trim());
          return { home: titles.findIndex((t) => t.includes('Carpeta de datos')), limits: titles.findIndex((t) => t.includes('Límites de recursos')) };
        });
        ctx.assert(order.home >= 0 && order.limits >= 0, `faltan títulos de tarjeta: ${JSON.stringify(order)}`);
        ctx.assert(order.home < order.limits, `la tarjeta de carpeta debe ir antes: ${JSON.stringify(order)}`);
      },
    },
    {
      name: 'data_home: con IWF_HOME definida muestra la carpeta actual, explica el bloqueo y no ofrece mover',
      fn: async (ctx) => {
        await openResources(ctx);
        await ctx.page.getByText('Carpeta actual').first().waitFor({ timeout: 8000 });
        await ctx.page.getByText('IWF_HOME está definida').first().waitFor({ timeout: 8000 });
        const movers = await ctx.page.getByRole('button', { name: 'Mover carpeta' }).count();
        ctx.assert(movers === 0, `no debe haber botón Mover carpeta con IWF_HOME definida (hay ${movers})`);
        const inputs = await ctx.page.getByLabel('Nueva carpeta (ruta absoluta)').count();
        ctx.assert(inputs === 0, `no debe haber campo de destino con IWF_HOME definida (hay ${inputs})`);
      },
    },
    {
      name: 'data_home: una petición de movimiento con IWF_HOME definida se rechaza sin reiniciar ni guardar nada',
      fn: async (ctx) => {
        await openResources(ctx);
        const response = await ctx.page.request.post(ctx.base + '/data-home/move', { form: { destination: '/tmp/iwf-should-not-exist' } });
        ctx.assert(response.ok(), `el handler responde 200 con la tarjeta (status ${response.status()})`);
        const body = await response.text();
        ctx.assert(body.includes('IWF_HOME'), 'la respuesta explica que IWF_HOME fija la carpeta');
        // The server is still the same one: the page answers right after.
        const again = await ctx.page.request.get(ctx.base + '/data-home/card');
        ctx.assert(again.ok(), 'el servidor sigue respondiendo (no se reinició)');
      },
    },
  ],
};
