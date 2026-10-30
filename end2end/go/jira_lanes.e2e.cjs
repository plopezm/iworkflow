// end2end/go/jira_lanes.e2e.cjs
// Modular E2E tests for Jira J2: connection, sync, issue listing and custom lane management.

const JIRA_URL = process.env.JIRA_MOCK_URL || 'http://127.0.0.1:9999';
const JIRA_TOKEN = 'e2e-token';
const tag = Date.now().toString();

async function connectAndSync(ctx) {
  try { await fetch(`${JIRA_URL}/_reset`); }
  catch (e) { /* mock may not be ready; ignore */ }
  await ctx.goto('/jira');
  await ctx.page.getByText('Conexión con Jira').first().waitFor({ timeout: 10000 });
  const already = await ctx.page.getByText('Editar conexión').first().count();
  if (already === 0) {
    await ctx.page.getByLabel(/^URL del servidor/).first().fill(JIRA_URL);
    await ctx.page.getByLabel(/^Token de usuario/).first().fill(JIRA_TOKEN);
    await ctx.page.getByRole('button', { name: /Conectar|Comprobando/ }).first().click();
    await ctx.page.getByText('Editar conexión').first().waitFor({ timeout: 15000 });
  }
  await ctx.page.getByRole('button', { name: /Sincronizar/ }).first().click();
  await ctx.page.getByText('Mis tareas').first().waitFor({ timeout: 15000 });
  await ctx.page.getByText('ABC-1').first().waitFor({ timeout: 15000 });
  await ctx.page.waitForTimeout(800);
}

async function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

module.exports = {
  name: 'jira_lanes',
  tests: [
    {
      name: 'save_connection + get_connection: conectar con el mock y ver el resumen',
      fn: async (ctx) => {
        await connectAndSync(ctx);
        const page = ctx.page;
        await page.getByText('Editar conexión').first().waitFor({ timeout: 15000 });
        await page.getByText('Desconectar').first().waitFor({ timeout: 15000 });
        ctx.assert(await page.getByText('Servidor:').first().count() > 0, 'falta el resumen "Servidor:"');
        ctx.assert(await page.getByText('Última sincronización:').first().count() > 0, 'falta "Última sincronización:"');
      },
    },
    {
      name: 'sync_issues: sincronizar importa los issues en "Mis tareas"',
      fn: async (ctx) => {
        await connectAndSync(ctx);
        ctx.assert(await ctx.page.getByText('Mis tareas').first().count() > 0, 'falta la lane "Mis tareas"');
      },
    },
    {
      name: 'list_issues: los tres issues aparecen en la lane por defecto',
      fn: async (ctx) => {
        await connectAndSync(ctx);
        const page = ctx.page;
        const lane = page.locator('section:has-text("Mis tareas")').first();
        await lane.locator('a[href="/jira/ABC-1"]').first().waitFor({ timeout: 15000 });
        await lane.locator('a[href="/jira/ABC-2"]').first().waitFor({ timeout: 15000 });
        await lane.locator('a[href="/jira/ABC-3"]').first().waitFor({ timeout: 15000 });
        await lane.getByText('Falla el login').first().waitFor({ timeout: 15000 });
        await lane.getByText('Documentar la API').first().waitFor({ timeout: 15000 });
      },
    },
    {
      name: 'create_lane: crear un carril nuevo',
      fn: async (ctx) => {
        await connectAndSync(ctx);
        const page = ctx.page;
        await page.getByRole('button', { name: /Nuevo carril/ }).first().click();
        const laneName = `carril-e2e-${tag}`;
        await page.getByLabel(/Nombre/).first().fill(laneName);
        await page.getByLabel(/Consulta JQL|JQL/).first().fill('project = ABC ORDER BY created DESC');
        await page.getByRole('button', { name: /Crear carril/ }).first().click();
        await page.getByText(laneName).first().waitFor({ timeout: 15000 });
      },
    },
    {
      name: 'update_lane: editar el nombre de un carril',
      fn: async (ctx) => {
        await connectAndSync(ctx);
        const page = ctx.page;
        const laneName = `edit-${tag}`;
        await page.getByRole('button', { name: /Nuevo carril/ }).first().click();
        await page.getByLabel(/Nombre/).first().fill(laneName);
        await page.getByLabel(/Consulta JQL|JQL/).first().fill('project = ABC');
        await page.getByRole('button', { name: /Crear carril/ }).first().click();
        await page.getByText(laneName).first().waitFor({ timeout: 15000 });
        const lane = page.locator('section.rounded-xl', { hasText: laneName }).first();
        await lane.getByTitle('Editar carril').first().click();
        await page.getByText('Editar carril').first().waitFor({ timeout: 15000 });
        await page.getByLabel(/Nombre/).first().fill(`edited-${tag}`);
        await page.getByRole('button', { name: /Guardar cambios/ }).first().click();
        await page.getByText(`edited-${tag}`).first().waitFor({ timeout: 15000 });
      },
    },
    {
      name: 'delete_lane: borrar un carril creado',
      fn: async (ctx) => {
        await connectAndSync(ctx);
        const page = ctx.page;
        const laneName = `del-${tag}`;
        await page.getByRole('button', { name: /Nuevo carril/ }).first().click();
        await page.getByLabel(/Nombre/).first().fill(laneName);
        await page.getByLabel(/Consulta JQL|JQL/).first().fill('project = ABC');
        await page.getByRole('button', { name: /Crear carril/ }).first().click();
        await page.getByText(laneName).first().waitFor({ timeout: 15000 });
        const lane = page.locator('section.rounded-xl', { hasText: laneName }).first();
        const toggle = lane.locator('header button[aria-expanded]').first();
        if ((await toggle.getAttribute('aria-expanded')) === 'false') {
          await toggle.click();
          await sleep(400);
        }
        await lane.getByTitle('Eliminar carril').first().click();
        const confirmBtn = lane.locator('button.btn-primary:has-text("Eliminar carril")').first();
        await confirmBtn.waitFor({ timeout: 8000 });
        await confirmBtn.click();
        let gone = false;
        const start = Date.now();
        while (Date.now() - start < 15000) {
          gone = await page.getByText(laneName).first().count() === 0;
          if (gone) break;
          await sleep(500);
        }
        ctx.assert(gone, `el carril "${laneName}" no se borró`);
      },
    },
  ],
};
