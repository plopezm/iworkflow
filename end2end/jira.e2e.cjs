// end2end/jira.e2e.cjs
// Pruebas end-to-end (Playwright CJS, harness compartido) del módulo /jira.
// El Jira REST-v2 mock existe y lo arranca end2end/run_tests.sh en $JIRA_MOCK_URL
// (end2end/jira_mock.cjs). Issues semilla: ABC-1 (To Do/Bug), ABC-2 (In Progress/Task),
// ABC-3 (Done/Task). La lane por defecto id='mine' (nombre 'Mis tareas') importa todos
// los issues porque el mock ignora el JQL y devuelve los tres.
//
// Contracto (como workspaces / agents_files):
//   - ctx.page.getByLabel / getByRole / getByText / locator(...) — nunca locator('text=...').
//   - ctx.goto(ruta) del harness (resuelve SMOKE_BASE_URL y hace domcontentloaded + 400ms).
//   - Selects: hay varios <select> por página (el selector de agente/modelo en el header).
//     Los select del módulo usan <select class="input"> con label asociado, así que se
//     apuntan por label: getByLabel('Transición'), getByLabel('Workspace'), getByLabel('Prioridad').
//   - Polling con waitFor; los selects con Suspend cargan las opciones asíncronamente.
//   - La BD compartida persiste el estado entre tests: cada test hace connectAndSync()
//     al inicio (idempotente) y los mutables usan names únicos.

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';
const JIRA_URL = process.env.JIRA_MOCK_URL || 'http://127.0.0.1:9999';
const JIRA_TOKEN = 'e2e-token';

function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `jira-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `content ${prefix} ${tag}`);
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

async function registerWorkspace(ctx, name) {
  const repo = uniqueGitRepo(`ws-${name}`);
  await ctx.goto('/workspaces');
  await ctx.page.getByLabel('Nombre').first().waitFor({ timeout: 10000 });
  await ctx.page.getByLabel('Nombre').first().fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(repo);
  await ctx.page.getByLabel(/Rama base/).first().fill('main');
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.getByText(name).first().waitFor({ timeout: 12000 });
  return name;
}

async function connectAndSync(ctx) {
  // Re-siembra el mock al inicio de cada test: el transition_issue muta el estado de Jira
  // de ABC-1 (y otros tests reutilizan el mismo mock en el mismo servidor). El reset deja
  // la semilla original. (run_tests.sh ya arranca un mock fresco; esto hace la suite robusta
  // si el mock es compartido o se reutiliza.)
  try { await fetch(`${JIRA_URL}/_reset`); }
  catch (e) { /* el mock puede no estar; no bloquear la conexión */ }
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
  await ctx.page.waitForTimeout(1200);
}

// Polling local: espera X ms (los select con Suspend cargan las opciones asíncronamente).
async function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

// Espera a que un select (por su label) tenga la opción dada (Suspend carga asíncrono).
// select.evaluate solo serializa la callback: la variable externa llega como argumento.
async function selectWhenReady(page, label, optionText) {
  const select = page.getByLabel(label);
  await select.waitFor({ timeout: 15000 });
  const start = Date.now();
  while (Date.now() - start < 15000) {
    const has = await select.evaluate((el, opt) =>
      Array.from(el.options).some((o) => o.textContent === opt)
    , optionText);
    if (has) return select;
    await sleep(200);
  }
  throw new Error(`no hay opción "${optionText}" en el select "${label}"`);
}

module.exports = {
  name: 'jira',
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
      name: 'get_issue: abrir ABC-1 muestra el detalle (key, resumen, estado, asignado)',
      fn: async (ctx) => {
        await connectAndSync(ctx);
        const page = ctx.page;
        await ctx.goto('/jira/ABC-1');
        await page.getByText('ABC-1: Falla el login').first().waitFor({ timeout: 15000 });
        await page.locator('.badge').getByText('To Do').first().waitFor({ timeout: 15000 });
        await page.getByText('Nueva').first().waitFor({ timeout: 15000 });
        await page.getByText('Responsable:').first().waitFor({ timeout: 15000 });
        await page.getByText('Ana').first().waitFor({ timeout: 15000 });
        await page.getByText('Descripción').first().waitFor({ timeout: 15000 });
        await page.getByText('El login falla en el navegador.').first().waitFor({ timeout: 15000 });
      },
    },
    {
      name: 'list_issue_comments: el detalle muestra la sección de comentarios',
      fn: async (ctx) => {
        await connectAndSync(ctx);
        const page = ctx.page;
        await ctx.goto('/jira/ABC-1');
        await page.getByText('Comentarios').first().waitFor({ timeout: 15000 });
        await page.getByText('Sin comentarios.').first().waitFor({ timeout: 15000 });
      },
    },
    {
      name: 'add_issue_comment: añadir un comentario lo muestra en la lista',
      fn: async (ctx) => {
        await connectAndSync(ctx);
        const page = ctx.page;
        const comment = `comentario e2e ${tag}`;
        await ctx.goto('/jira/ABC-2');
        await page.getByText('ABC-2: Mejorar el onboarding').first().waitFor({ timeout: 15000 });
        await page.locator('textarea').first().fill(comment);
        await page.getByRole('button', { name: /Comentar en Jira/ }).first().click();
        await page.getByText(comment).first().waitFor({ timeout: 15000 });
      },
    },
    {
      name: 'list_issue_transitions: el detalle muestra la selección de transiciones',
      fn: async (ctx) => {
        await connectAndSync(ctx);
        const page = ctx.page;
        await ctx.goto('/jira/ABC-1');
        await page.getByText('Cambiar estado en Jira').first().waitFor({ timeout: 15000 });
        const transSelect = page.getByLabel('Transición');
        await transSelect.waitFor({ timeout: 15000 });
        let opts = [];
        const start = Date.now();
        while (Date.now() - start < 15000) {
          opts = await transSelect.evaluate((el) => Array.from(el.options).map((o) => o.textContent));
          if (opts.length > 1) break;
          await sleep(200);
        }
        ctx.assert(
          opts.some((t) => t.includes('Iniciar → In Progress')) && opts.some((t) => t.includes('Cerrar → Done')),
          `no hay opciones de transición: ${JSON.stringify(opts)}`
        );
      },
    },
    {
      name: 'transition_issue: aplicar una transición cambia el estado a In Progress',
      fn: async (ctx) => {
        await connectAndSync(ctx);
        const page = ctx.page;
        await ctx.goto('/jira/ABC-1');
        await page.getByText('Cambiar estado en Jira').first().waitFor({ timeout: 15000 });
        const transSelect = page.getByLabel('Transición');
        await selectWhenReady(page, 'Transición', 'Iniciar → In Progress');
        await transSelect.selectOption({ label: 'Iniciar → In Progress' });
        await page.getByRole('button', { name: /Aplicar|Aplicando/ }).first().click();
        await sleep(1500);
        await page.locator('.badge').getByText('In Progress').first().waitFor({ timeout: 15000 });
      },
    },
    {
      name: 'set_issue_ignored: ignorar y restaurar un issue',
      fn: async (ctx) => {
        await connectAndSync(ctx);
        const page = ctx.page;
        // Usamos ABC-2 (nunca se convierte): ABC-3 lo hace el test de convert_issue
        // y persiste el estado 'converted' en la BD entre runs.
        await ctx.goto('/jira/ABC-2');
        await page.getByText('ABC-2: Mejorar el onboarding').first().waitFor({ timeout: 15000 });
        await page.getByRole('button', { name: /Ignorar/ }).first().click();
        await page.getByRole('button', { name: /Restaurar/ }).first().waitFor({ timeout: 15000 });
        await page.getByText('Ignorada').first().waitFor({ timeout: 15000 });
        await page.getByRole('button', { name: /Restaurar/ }).first().click();
        await page.getByRole('button', { name: /Ignorar/ }).first().waitFor({ timeout: 15000 });
        await page.getByText('Nueva').first().waitFor({ timeout: 15000 });
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
        // Un carril nuevo puede nacer colapsado: el ConfirmPanel vive en el cuerpo colapsable
        // (overflow-hidden, 0fr), así que se expande antes de pedir la confirmación.
        const toggle = lane.locator('header button[aria-expanded]').first();
        if ((await toggle.getAttribute('aria-expanded')) === 'false') {
          await toggle.click();
          await sleep(400);
        }
        await lane.getByTitle('Eliminar carril').first().click();
        // ConfirmPanel inline (no es dialog nativo): confirmar con el btn-primary "Eliminar carril".
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
    {
      name: 'prepare_convion: abrir la página de conversión de ABC-1',
      fn: async (ctx) => {
        await connectAndSync(ctx);
        await registerWorkspace(ctx, `ws-conv-${tag}`);
        await ctx.goto('/jira/ABC-1/convert');
        await ctx.page.getByText(/Convertir ABC-1 en tarea de iworkflow/).first().waitFor({ timeout: 15000 });
        ctx.assert(await ctx.page.getByText('Workspace').first().count() > 0, 'falta el select de workspace');
      },
    },
    {
      name: 'list_convion_branches: la conversión muestra el workspace y las prioridades',
      fn: async (ctx) => {
        await connectAndSync(ctx);
        const wsName = await registerWorkspace(ctx, `ws-br-${tag}`);
        await ctx.goto('/jira/ABC-1/convert');
        await ctx.page.getByText(/Convertir ABC-1 en tarea de iworkflow/).first().waitFor({ timeout: 15000 });
        const wsSelect = ctx.page.getByLabel('Workspace');
        await selectWhenReady(ctx.page, 'Workspace', `${wsName} (main)`);
        await wsSelect.selectOption({ label: `${wsName} (main)` });
        await sleep(800);
        await ctx.page.getByText('Prioridad').first().waitFor({ timeout: 15000 });
        await ctx.page.getByText('Prompt').first().waitFor({ timeout: 15000 });
      },
    },
    {
      name: 'convert_issue: convertir ABC-3 crea una tarea y la marca Convertida',
      fn: async (ctx) => {
        await connectAndSync(ctx);
        const wsName = await registerWorkspace(ctx, `ws-cv-${tag}`);
        await ctx.goto('/jira/ABC-3/convert');
        await ctx.page.getByText(/Convertir ABC-3 en tarea de iworkflow/).first().waitFor({ timeout: 15000 });
        const wsSelect = ctx.page.getByLabel('Workspace');
        await selectWhenReady(ctx.page, 'Workspace', `${wsName} (main)`);
        await wsSelect.selectOption({ label: `${wsName} (main)` });
        await ctx.page.getByLabel('Prioridad').first().selectOption({ label: 'Normal' }, { noScroll: true });
        await ctx.page.getByRole('button', { name: /Crear tarea/ }).first().click();
        await sleep(3000);
        await ctx.goto('/jira/ABC-3');
        await ctx.page.getByText('ABC-3: Documentar la API').first().waitFor({ timeout: 15000 });
        await ctx.page.getByText('Ver tarea de iworkflow').first().waitFor({ timeout: 15000 });
      },
    },
  ],
};
