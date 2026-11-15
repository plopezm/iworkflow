// end2end/go/usage_limits.e2e.cjs
// Go-mode e2e of the usage bar in the sidebar footer (usage_limits module, S1/S5/S6). The runner seeds one static
// state (L4): Claude 13 % / 25 % from the usage mock (end2end/usage_mock.cjs), Codex 75 % / 95 % from a fake rollout,
// so the levels are normal (Claude), warn and critical (Codex). The bar refreshes every 60 s and its server cache
// lasts about as long, so the values do not change during a test. The suite runs in Spanish.
// Screenshots go to SMOKE_SCREENSHOT_DIR (set by run_go.sh to <run dir>/screenshots); their absolute paths are printed.

const fs = require('fs');
const path = require('path');

const SHOT_DIR = process.env.SMOKE_SCREENSHOT_DIR || '/tmp/iwf-e2e-screenshots';
const BAR = '#usage-bar';
const CLAUDE = '[data-usage-provider="claude"]';
const CODEX = '[data-usage-provider="codex"]';

// Opens the shell and waits until the bar is loaded by htmx (the placeholder swaps itself for the bar).
async function openShell(ctx) {
  await ctx.goto('/');
  await ctx.page.locator(`${BAR} [data-usage-provider]`).first().waitFor({ state: 'visible', timeout: 15000 });
}

// Waits for a provider card; on timeout the error carries the bar markup, so a missing card shows its cause.
async function waitForCard(ctx, selector) {
  try {
    await ctx.page.locator(selector).waitFor({ state: 'visible', timeout: 15000 });
  } catch (err) {
    const markup = await ctx.page.locator(BAR).innerHTML({ timeout: 2000 }).catch(() => '(no #usage-bar)');
    throw new Error(`${err.message.split('\n')[0]}\nbar markup: ${markup.slice(0, 2000)}`);
  }
}

function windowRow(ctx, provider, window) {
  return ctx.page.locator(`${provider} [data-usage-window="${window}"]`);
}

// The percentage span of a row: the only span whose whole text is "<n>%".
async function percentOf(row) {
  const text = (await row.locator('span').filter({ hasText: /^\d+%$/ }).first().textContent()) || '';
  return text.trim();
}

// The filled part of a track: the element with class h-full inside the row.
function fillOf(row) {
  return row.locator('div.h-full').first();
}

async function backgroundOf(locator) {
  return locator.evaluate((el) => getComputedStyle(el).backgroundColor);
}

// Writes a screenshot into SHOT_DIR and returns its absolute path (the directory is created on demand).
async function saveShot(locator, name) {
  fs.mkdirSync(SHOT_DIR, { recursive: true });
  const abs = path.resolve(SHOT_DIR, name);
  await locator.screenshot({ path: abs });
  return abs;
}

module.exports = {
  name: 'usage_limits',
  tests: [
    {
      name: 'footer: la barra de uso aparece en el pie del sidebar, encima de la barra de ajustes del agente',
      fn: async (ctx) => {
        await openShell(ctx);
        const bar = ctx.page.locator(BAR);
        await bar.waitFor({ state: 'visible', timeout: 15000 });
        const agent = ctx.page.locator('#agent-settings-bar');
        await agent.waitFor({ state: 'visible', timeout: 15000 });
        const barBox = await bar.boundingBox();
        const agentBox = await agent.boundingBox();
        ctx.assert(barBox && agentBox, 'no se pudo medir la barra de uso o la de ajustes del agente');
        ctx.assert(
          barBox.y + barBox.height <= agentBox.y,
          `la barra de uso (y=${barBox.y}, alto=${barBox.height}) no está encima de la de ajustes (y=${agentBox.y})`,
        );
        // Both bars share the footer container: the usage bar is inside it, not in the page body.
        const footerHoldsBoth = await ctx.page.evaluate(() => {
          const bar = document.getElementById('usage-bar');
          const agent = document.getElementById('agent-settings-bar');
          return !!(bar && agent && bar.parentElement === agent.parentElement);
        });
        ctx.assert(footerHoldsBoth, 'la barra de uso y la de ajustes no comparten el contenedor del pie');
      },
    },
    {
      name: 'claude: 5h y semana muestran 13 % y 25 % en nivel normal, con reinicio y relleno a su ancho',
      fn: async (ctx) => {
        await openShell(ctx);
        const card = ctx.page.locator(CLAUDE);
        await card.waitFor({ state: 'visible', timeout: 15000 });
        ctx.assert((await card.locator('span').filter({ hasText: /^Claude$/ }).count()) > 0, 'la tarjeta no se llama Claude');

        const fiveH = windowRow(ctx, CLAUDE, '5h');
        const week = windowRow(ctx, CLAUDE, 'week');
        ctx.assert((await percentOf(fiveH)) === '13%', `5h: se esperaba 13%, se lee "${await percentOf(fiveH)}"`);
        ctx.assert((await percentOf(week)) === '25%', `semana: se esperaba 25%, se lee "${await percentOf(week)}"`);
        ctx.assert((await fiveH.getAttribute('data-level')) === 'normal', `5h: nivel ${await fiveH.getAttribute('data-level')}`);
        ctx.assert((await week.getAttribute('data-level')) === 'normal', `semana: nivel ${await week.getAttribute('data-level')}`);
        ctx.assert((await week.locator('span').first().textContent()).trim() === 'semana', 'la fila semanal no está rotulada "semana"');

        // The reset of the 5 h window is two hours and about ten minutes away; the week window is three days away.
        const fiveHReset = (await fiveH.locator('span').filter({ hasText: '↻' }).first().textContent()).trim();
        const weekReset = (await week.locator('span').filter({ hasText: '↻' }).first().textContent()).trim();
        ctx.assert(/^↻\s*2h\d{2}$/.test(fiveHReset), `reinicio 5h inesperado: "${fiveHReset}"`);
        // The week reset is three days away at seeding; the server rounds to the nearest day.
        ctx.assert(/^↻\s*3d$/.test(weekReset), `reinicio semanal inesperado: "${weekReset}"`);

        ctx.assert((await fillOf(fiveH).evaluate((el) => el.style.width)) === '13%', 'el relleno 5h no mide 13%');
        ctx.assert((await fillOf(week).evaluate((el) => el.style.width)) === '25%', 'el relleno semanal no mide 25%');
        ctx.assert((await fillOf(fiveH).getAttribute('class')).includes('bg-ok'), 'el relleno normal no usa bg-ok');
      },
    },
    {
      name: 'codex: 5h 75 % en warn y semana 95 % en critical, con colores de relleno distintos del normal',
      fn: async (ctx) => {
        await openShell(ctx);
        await waitForCard(ctx, CODEX);

        const fiveH = windowRow(ctx, CODEX, '5h');
        const week = windowRow(ctx, CODEX, 'week');
        ctx.assert((await percentOf(fiveH)) === '75%', `Codex 5h: se esperaba 75%, se lee "${await percentOf(fiveH)}"`);
        ctx.assert((await percentOf(week)) === '95%', `Codex semana: se esperaba 95%, se lee "${await percentOf(week)}"`);
        ctx.assert((await fiveH.getAttribute('data-level')) === 'warn', `Codex 5h: nivel ${await fiveH.getAttribute('data-level')}`);
        ctx.assert((await week.getAttribute('data-level')) === 'critical', `Codex semana: nivel ${await week.getAttribute('data-level')}`);
        ctx.assert((await fillOf(fiveH).evaluate((el) => el.style.width)) === '75%', 'el relleno Codex 5h no mide 75%');
        ctx.assert((await fillOf(week).evaluate((el) => el.style.width)) === '95%', 'el relleno Codex semana no mide 95%');

        const classes5h = await fillOf(fiveH).getAttribute('class');
        const classesWeek = await fillOf(week).getAttribute('class');
        ctx.assert(classes5h.includes('bg-warn'), `relleno Codex 5h sin bg-warn: "${classes5h}"`);
        ctx.assert(classesWeek.includes('bg-error'), `relleno Codex semana sin bg-error: "${classesWeek}"`);

        // The colours differ from each other and from the normal fill of Claude.
        const normal = await backgroundOf(fillOf(windowRow(ctx, CLAUDE, '5h')));
        const warn = await backgroundOf(fillOf(fiveH));
        const critical = await backgroundOf(fillOf(week));
        ctx.assert(warn !== normal, `el relleno warn tiene el mismo color que el normal (${warn})`);
        ctx.assert(critical !== normal, `el relleno critical tiene el mismo color que el normal (${critical})`);
        ctx.assert(critical !== warn, `warn y critical tienen el mismo color (${warn})`);
      },
    },
    {
      name: 'polling: la barra se refresca cada 60 s y la página carga sin errores de consola ni HTTP',
      fn: async (ctx) => {
        await openShell(ctx);
        const bar = ctx.page.locator(BAR);
        const trigger = (await bar.getAttribute('hx-trigger')) || '';
        ctx.assert(trigger.includes('every 60s'), `hx-trigger de la barra: "${trigger}"`);
        ctx.assert((await bar.getAttribute('hx-get')) === '/usage-limits/bar', 'la barra no se refresca desde /usage-limits/bar');
        // Let the page settle, then check that nothing failed while loading the shell and the bar.
        await ctx.page.waitForTimeout(500);
        const usageProblems = ctx.getProblems().filter((p) => /usage|console:error|pageerror/.test(p));
        ctx.assert(usageProblems.length === 0, `problemas al cargar la página: ${usageProblems.join(' | ')}`);
      },
    },
    {
      name: 'capturas: la barra y el sidebar completo se guardan como PNG y se imprimen sus rutas',
      fn: async (ctx) => {
        await ctx.page.setViewportSize({ width: 1440, height: 860 });
        await openShell(ctx);
        await ctx.page.locator(`${CLAUDE}`).waitFor({ state: 'visible', timeout: 15000 });
        await waitForCard(ctx, CODEX);

        const barShot = await saveShot(ctx.page.locator(BAR), 'usage-bar.png');
        const fullShot = path.resolve(SHOT_DIR, 'usage-sidebar-full.png');
        await ctx.page.screenshot({ path: fullShot, fullPage: true });

        ctx.assert(fs.statSync(barShot).size > 0, `captura vacía: ${barShot}`);
        ctx.assert(fs.statSync(fullShot).size > 0, `captura vacía: ${fullShot}`);
        console.log(`screenshot: ${barShot}`);
        console.log(`screenshot: ${fullShot}`);
      },
    },
  ],
};
