// end2end/go/jira_epics.e2e.cjs
// E2E of the Jira board grouped by epic: grouping, selecting an epic, changing the status of a child
// issue, filters, the Lanes switch, keyboard use and the narrow layout. Screenshots of the key states
// are written to JIRA_EPICS_SHOTS (default /tmp/jira-epics-shots) for a visual review.
//
// The Jira mock holds three default issues without epic. POST /_seed_epics adds two epics:
//   EP-10 "Checkout rediseñado" (In Progress): CK-1 Done, CK-2 In Progress, CK-3 To Do
//   EP-20 "Onboarding guiado"   (To Do):       ON-1 To Do

const fs = require('node:fs');
const path = require('node:path');

const JIRA_URL = process.env.JIRA_MOCK_URL || 'http://127.0.0.1:9999';
const JIRA_TOKEN = 'e2e-token';
const SHOTS = process.env.JIRA_EPICS_SHOTS || '/tmp/jira-epics-shots';
fs.mkdirSync(SHOTS, { recursive: true });

const settle = async (ctx) => { await ctx.page.waitForTimeout(700); await ctx.page.waitForLoadState('networkidle'); };
const shot = (ctx, name) => ctx.screenshot(path.join(SHOTS, `${name}.png`));

async function openBoard(ctx) {
  await fetch(`${JIRA_URL}/_reset`);
  await fetch(`${JIRA_URL}/_seed_epics`);
  await ctx.goto('/jira');
  await ctx.page.getByText('Conexión con Jira').first().waitFor({ timeout: 10000 });
  if ((await ctx.page.getByText('Editar conexión').first().count()) === 0) {
    await ctx.page.getByLabel(/^URL del servidor/).first().fill(JIRA_URL);
    await ctx.page.getByLabel(/^Token de usuario/).first().fill(JIRA_TOKEN);
    await ctx.page.getByRole('button', { name: /Conectar|Comprobando/ }).first().click();
    await ctx.page.getByText('Editar conexión').first().waitFor({ timeout: 15000 });
  }
  await ctx.page.getByRole('button', { name: /Sincronizar/ }).first().click();
  await card(ctx, 'Checkout rediseñado').waitFor({ timeout: 15000 });
  // Let the sync swap settle before interacting, as a person would.
  await ctx.page.waitForLoadState('networkidle');
  await ctx.page.waitForTimeout(600);
}

const card = (ctx, title) =>
  ctx.page.locator('ul[aria-label="Épicas"] > li > button', { hasText: title }).first();
const detail = (ctx, title) => ctx.page.locator(`section[aria-label="${title}"]`).first();
const statusOf = (ctx, key) => ctx.page.locator(`#status-${key}`);

async function mockStatus(key) {
  const res = await fetch(`${JIRA_URL}/rest/api/2/issue/${key}`);
  return (await res.json()).fields.status.name;
}

module.exports = {
  name: 'jira_epics',
  tests: [
    {
      name: 'agrupa por epic, con progreso, y abre la primera con sus subtareas',
      noProblems: true,
      fn: async (ctx) => {
        await openBoard(ctx);
        const page = ctx.page;
        const ep10 = card(ctx, 'Checkout rediseñado');
        ctx.assert((await ep10.innerText()).includes('1/3'), 'EP-10 debe mostrar 1/3 hechas');
        ctx.assert((await card(ctx, 'Onboarding guiado').innerText()).includes('0/1'), 'EP-20 debe mostrar 0/1');
        ctx.assert((await card(ctx, 'Sin epic').count()) === 1, 'falta el grupo "Sin epic"');
        const open = await page.locator('ul[aria-label="Épicas"] > li > button[aria-current="true"]').count();
        ctx.assert(open === 1, `debe haber exactamente una epic abierta (hay ${open})`);
        await ep10.click();
        await detail(ctx, 'Checkout rediseñado').waitFor({ timeout: 10000 });
        const bar = ep10.getByRole('progressbar');
        ctx.assert((await bar.getAttribute('aria-valuenow')) === '1', 'la barra debe indicar 1 hecha');
        const panel = detail(ctx, 'Checkout rediseñado');
        for (const text of ['Pasarela de pago', 'Validar la tarjeta', 'Recibo por email']) {
          ctx.assert((await panel.getByText(text).count()) > 0, `falta la subtarea "${text}"`);
        }
        ctx.assert((await panel.getByText('Pantalla de bienvenida').count()) === 0, 'no debe mezclar subtareas de otra epic');
        await page.waitForTimeout(300);
        await shot(ctx, '01-desktop-epics');
      },
    },
    {
      name: 'al seleccionar otra epic se ven sus subtareas y la selección se marca',
      fn: async (ctx) => {
        await openBoard(ctx);
        await card(ctx, 'Onboarding guiado').click();
        await detail(ctx, 'Onboarding guiado').getByText('Pantalla de bienvenida').waitFor({ timeout: 10000 });
        ctx.assert((await card(ctx, 'Onboarding guiado').getAttribute('aria-current')) === 'true', 'EP-20 debe quedar marcada');
        ctx.assert((await card(ctx, 'Checkout rediseñado').getAttribute('aria-current')) === 'false', 'EP-10 ya no debe estar marcada');
        ctx.assert((await ctx.page.getByText('Pasarela de pago').count()) === 0, 'las subtareas de EP-10 deben desaparecer');
        await card(ctx, 'Sin epic').click();
        await ctx.page.locator('a[href="/jira/ABC-1"]').first().waitFor({ timeout: 10000 });
        await shot(ctx, '02-sin-epic');
      },
    },
    {
      name: 'cambia el estado de una subtarea desde el detalle y llega a Jira',
      noProblems: true,
      fn: async (ctx) => {
        await openBoard(ctx);
        const page = ctx.page;
        const trigger = statusOf(ctx, 'CK-3').getByRole('button').first();
        ctx.assert((await trigger.innerText()).includes('To Do'), 'CK-3 empieza en To Do');
        await trigger.click();
        const start = statusOf(ctx, 'CK-3').getByRole('button', { name: /Iniciar/ });
        await start.waitFor({ timeout: 10000 });
        await page.waitForTimeout(200);
        await shot(ctx, '03-menu-de-estados');
        await start.click();
        await statusOf(ctx, 'CK-3').getByText('In Progress').first().waitFor({ timeout: 10000 });
        ctx.assert((await mockStatus('CK-3')) === 'In Progress', 'Jira debe recibir la transición');
        ctx.assert((await card(ctx, 'Checkout rediseñado').getAttribute('aria-current')) === 'true', 'la misma epic sigue abierta');

        await statusOf(ctx, 'CK-3').getByRole('button').first().click();
        await statusOf(ctx, 'CK-3').getByRole('button', { name: /Cerrar/ }).click();
        await page.waitForFunction(() => [...document.querySelectorAll('ul[aria-label="Épicas"] > li > button')].some((b) => b.innerText.includes('2/3')), null, { timeout: 10000 });
        ctx.assert((await mockStatus('CK-3')) === 'Done', 'Jira debe quedar en Done');
        await shot(ctx, '04-tras-cambiar-estado');
      },
    },
    {
      name: 'el menú de estados se puede cancelar sin cambiar nada',
      fn: async (ctx) => {
        await openBoard(ctx);
        await statusOf(ctx, 'CK-3').getByRole('button').first().click();
        await statusOf(ctx, 'CK-3').getByRole('button', { name: /Iniciar/ }).waitFor({ timeout: 10000 });
        await ctx.page.getByRole('button', { name: /Cancelar/ }).first().click();
        await statusOf(ctx, 'CK-3').getByText('To Do').first().waitFor({ timeout: 10000 });
        ctx.assert((await mockStatus('CK-3')) === 'To Do', 'cancelar no debe cambiar el estado en Jira');
      },
    },
    {
      name: 'el buscador filtra las epics y borrar el filtro las devuelve',
      fn: async (ctx) => {
        await openBoard(ctx);
        const page = ctx.page;
        await page.getByRole('searchbox', { name: /Buscar/ }).pressSequentially('bienvenida');
        await card(ctx, 'Onboarding guiado').waitFor({ timeout: 10000 });
        await page.waitForFunction(() => document.querySelectorAll('ul[aria-label="Épicas"] > li').length === 1, null, { timeout: 10000 });
        await settle(ctx);
        ctx.assert((await card(ctx, 'Checkout rediseñado').count()) === 0, 'EP-10 no coincide con el filtro');
        await page.getByRole('searchbox', { name: /Buscar/ }).press('Control+A');
        await page.getByRole('searchbox', { name: /Buscar/ }).press('Backspace');
        await settle(ctx);
        await card(ctx, 'Checkout rediseñado').waitFor({ timeout: 10000 });
        await page.getByRole('searchbox', { name: /Buscar/ }).pressSequentially('zzz-no-existe');
        await settle(ctx);
        await page.getByText('Ninguna épica coincide con los filtros').waitFor({ timeout: 10000 });
        await shot(ctx, '05-sin-resultados');
      },
    },
    {
      name: 'el interruptor Carriles muestra los carriles y Épicas vuelve a la agrupación',
      fn: async (ctx) => {
        await openBoard(ctx);
        const page = ctx.page;
        await page.getByRole('button', { name: /^Carriles$/ }).click();
        await page.getByText('Mis tareas').first().waitFor({ timeout: 10000 });
        ctx.assert((await page.getByRole('button', { name: /^Carriles$/ }).getAttribute('aria-pressed')) === 'true', 'Carriles debe quedar activo');
        await page.getByRole('button', { name: /^Épicas$/ }).click();
        await card(ctx, 'Checkout rediseñado').waitFor({ timeout: 10000 });
      },
    },
    {
      name: 'se puede usar con teclado: foco en una epic y Enter la abre',
      fn: async (ctx) => {
        await openBoard(ctx);
        const ep20 = card(ctx, 'Onboarding guiado');
        await ep20.focus();
        await ctx.page.keyboard.press('Enter');
        await detail(ctx, 'Onboarding guiado').waitFor({ timeout: 10000 });
        ctx.assert((await card(ctx, 'Onboarding guiado').getAttribute('aria-current')) === 'true', 'Enter debe abrir la epic');
      },
    },
    {
      name: 'al cambiar de pestaña y volver se conservan el filtro, el texto escrito y la epic abierta',
      noProblems: true,
      fn: async (ctx) => {
        await openBoard(ctx);
        const page = ctx.page;
        const jiraTab = page.locator('.dock-tab', { hasText: 'Jira' }).first();
        const visible = (loc) => loc.locator('visible=true').first();

        await visible(page.locator('select[name="status"]')).selectOption('To Do');
        await visible(page.getByRole('searchbox', { name: /Buscar/ })).pressSequentially('a');
        await page.waitForFunction(() => {
          const select = [...document.querySelectorAll('select[name="status"]')].find((el) => el.offsetParent !== null);
          return select && select.value === 'To Do' && document.querySelectorAll('ul[aria-label="Épicas"] > li').length === 3;
        }, null, { timeout: 10000 });
        await settle(ctx);
        await card(ctx, 'Onboarding guiado').click();
        await detail(ctx, 'Onboarding guiado').getByText('Pantalla de bienvenida').waitFor({ timeout: 10000 });

        // Leave for the issue tab (a dock tab of the same module) and for a tab of another module.
        await detail(ctx, 'Onboarding guiado').locator('a[href="/jira/ON-1"]').first().click();
        await page.locator('.dock-tab', { hasText: 'ON-1' }).first().waitFor({ timeout: 10000 });
        const other = page.getByRole('navigation', { name: 'Navegación' }).getByRole('link').filter({ hasNotText: /Jira/ }).first();
        await other.click();
        await page.waitForTimeout(600);

        await jiraTab.click();
        await page.waitForTimeout(400);
        const state = await page.evaluate(() => {
          const shown = (selector) => [...document.querySelectorAll(selector)].find((el) => el.offsetParent !== null);
          return {
            status: shown('select[name="status"]')?.value,
            search: shown('input[name="search"]')?.value,
            cards: document.querySelectorAll('ul[aria-label="Épicas"] > li').length,
            pressed: shown('button[aria-pressed="true"]')?.textContent.trim(),
          };
        });
        ctx.assert(state.status === 'To Do', `el filtro de estado se perdió (${state.status})`);
        ctx.assert(state.search === 'a', `el texto del buscador se perdió (${state.search})`);
        ctx.assert(state.cards === 3, `la lista de epics cambió (${state.cards})`);
        ctx.assert(state.pressed === 'Épicas', `el modo cambió (${state.pressed})`);
        ctx.assert((await card(ctx, 'Onboarding guiado').getAttribute('aria-current')) === 'true', 'la epic abierta se perdió');
        ctx.assert((await detail(ctx, 'Onboarding guiado').getByText('Pantalla de bienvenida').count()) > 0, 'el detalle de la epic se perdió');
      },
    },
    {
      name: 'al cambiar de pestaña y volver se conserva el modo Carriles',
      fn: async (ctx) => {
        await openBoard(ctx);
        const page = ctx.page;
        await page.getByRole('button', { name: /^Carriles$/ }).click();
        await page.getByText('Mis tareas').first().waitFor({ timeout: 10000 });
        const other = page.getByRole('navigation', { name: 'Navegación' }).getByRole('link').filter({ hasNotText: /Jira/ }).first();
        await other.click();
        await page.waitForTimeout(600);
        await page.locator('.dock-tab', { hasText: 'Jira' }).first().click();
        await page.getByText('Mis tareas').first().waitFor({ timeout: 10000 });
        ctx.assert((await page.getByRole('button', { name: /^Carriles$/ }).getAttribute('aria-pressed')) === 'true', 'el modo Carriles se perdió');
      },
    },
    {
      name: 'en móvil no hay scroll horizontal y los controles son tocables',
      fn: async (ctx) => {
        await ctx.page.setViewportSize({ width: 390, height: 844 });
        await openBoard(ctx);
        const page = ctx.page;
        await page.waitForTimeout(400);
        const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
        ctx.assert(overflow <= 1, `hay scroll horizontal en móvil (${overflow}px)`);
        const box = await statusOf(ctx, 'CK-3').getByRole('button').first().boundingBox();
        ctx.assert(box && box.height >= 32, `el control de estado es demasiado pequeño (${box && box.height}px)`);
        const cardBox = await card(ctx, 'Checkout rediseñado').boundingBox();
        ctx.assert(cardBox && cardBox.height >= 44, 'la tarjeta de epic debe medir al menos 44px de alto');
        await shot(ctx, '06-movil');
        await card(ctx, 'Onboarding guiado').click();
        await detail(ctx, 'Onboarding guiado').waitFor({ timeout: 10000 });
        await page.waitForTimeout(700);
        await shot(ctx, '07-movil-detalle');
      },
    },
  ],
};
