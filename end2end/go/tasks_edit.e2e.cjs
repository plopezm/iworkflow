// end2end/go/tasks_edit.e2e.cjs
// Go-mode copy of the create and edit part of end2end/tasks.e2e.cjs (the parity target for slice H, U2): the
// create dialog with its branches and images, the edit tab of a backlog task, and the card actions of a backlog
// task (start, edit, delete). The suite runs in Spanish (the language cookie of the harness).
// Scope: the create and edit forms and the backlog card. The detail (U3), the transcript (U4) and the graph (U7)
// are not covered here. Each test registers its own git repo as a workspace, so the tests do not see each other.
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const SHOTS = process.env.E2E_SHOTS_DIR || '/tmp/iwf-edit-lazy-shots';
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';
// A one-pixel PNG for the image inputs.
const PNG = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==', 'base64');

function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `edit-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `edit ${prefix}\n`);
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

async function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

// Registers a workspace from the Workspaces screen and waits until it is listed.
async function registerWs(ctx, name) {
  const dir = uniqueGitRepo(name);
  await ctx.goto('/workspaces/new'); // registration has its own tab
  await ctx.page.getByLabel('Nombre').first().fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await ctx.page.getByLabel(/Rama base/).first().fill('main');
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.getByText(dir).first().waitFor({ timeout: 12000 });
  return { name, dir };
}

// Opens the board from the sidebar by clicking on the workspace item.
async function openBoard(ctx, name) {
  await ctx.goto('/');
  await ctx.page.locator('#board-sidebar').getByRole('link', { name }).first().click();
  await ctx.page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
  await sleep(300);
}

// Opens the create dialog of the board for the workspace and fills it.
async function fillDialog(ctx, ws, { title, prompt }) {
  const page = ctx.page;
  await page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  await page.locator('form:has(select[name=workspace_id]):visible select[name=workspace_id]').selectOption({ label: `${ws} (main)` });
  if (title) await page.locator('form:has(select[name=workspace_id]):visible input[name=title]').fill(title);
  await page.locator('form:has(select[name=workspace_id]):visible textarea[name=prompt]').fill(prompt);
}

// Submits the create dialog and waits until it closes.
async function submitDialog(ctx) {
  await ctx.page.locator('form:has(select[name=workspace_id]):visible button[type=submit]', { hasText: 'Crear tarea' }).click();
  await ctx.page.locator('form:has(select[name=workspace_id]):visible').waitFor({ state: 'detached', timeout: 10000 }).catch(() => {});
}

// Opens the edit tab of the backlog card that shows the prompt.
async function openEditFromCard(ctx, prompt) {
  const card = ctx.page.locator('[id^="board-list-"]:visible [data-stage="backlog"]', { hasText: prompt }).first();
  await card.waitFor({ timeout: 10000 });
  await card.getByRole('button', { name: 'Editar' }).click();
  await ctx.page.getByRole('heading', { name: 'Editar tarea' }).waitFor({ timeout: 15000 });
}

// Delays the background loads of the edit page (branches and agent section) so their loading state can be seen.
async function delayLazyLoads(page, ms) {
  await page.route(/\/board\/tasks\/(branches|agent-section|create-workspaces)\?/, async (route) => {
    await sleep(ms);
    await route.continue();
  });
}

async function shot(ctx, name) {
  fs.mkdirSync(SHOTS, { recursive: true });
  await ctx.screenshot(path.join(SHOTS, `${name}.png`));
}

// ---- tests ----
module.exports = {
  name: 'tasks_edit',
  tests: [
    {
      name: 'create_task: la imagen elegida se previsualiza y se guarda con la tarea',
      fn: async (ctx) => {
        const name = `te-image-${tag}`;
        const ws = await registerWs(ctx, name);
        await openBoard(ctx, ws.name);
        const prompt = `adjunto-${tag}`;
        await fillDialog(ctx, ws.name, { prompt });
        await ctx.page.locator('form:has(select[name=workspace_id]):visible input[type=file]').first().setInputFiles({ name: `captura-${tag}.png`, mimeType: 'image/png', buffer: PNG });
        await ctx.page.locator(`form:has(select[name=workspace_id]):visible img[alt="captura-${tag}.png"]`).first().waitFor({ timeout: 10000 });
        await submitDialog(ctx);
        await ctx.page.locator('[id^="board-list-"]:visible [data-stage="backlog"]', { hasText: prompt }).first().waitFor({ timeout: 10000 });
        await openEditFromCard(ctx, prompt);
        await ctx.page.locator(`img[alt="captura-${tag}.png"]`).first().waitFor({ state: 'attached', timeout: 15000 });
      },
    },
    {
      name: 'update_task: el editor precarga la tarea, guarda los cambios y vuelve al tablero',
      fn: async (ctx) => {
        const ws = await registerWs(ctx, `te-update-${tag}`);
        await openBoard(ctx, ws.name);
        const prompt = `orig-${tag}`;
        await fillDialog(ctx, ws.name, { prompt });
        await submitDialog(ctx);
        await openEditFromCard(ctx, prompt);
        ctx.assert((await ctx.page.getByLabel('Prompt').inputValue()) === prompt, 'el prompt no se precargó');
        await ctx.page.getByLabel(/^Título/).fill(`titulo-editado-${tag}`);
        await ctx.page.getByLabel('Prompt').fill(`prompt-editado-${tag}`);
        await ctx.page.getByRole('button', { name: 'Guardar cambios' }).click();
        await ctx.page.getByRole('heading', { name: 'Editar tarea' }).waitFor({ state: 'detached', timeout: 15000 });
        // The board the save returns to is a new tab; the one the user left stays in the DOM, hidden.
        await ctx.page.locator('[id^="board-list-"]:visible').getByText(`titulo-editado-${tag}`).first().waitFor({ timeout: 15000 });
      },
    },
    {
      name: 'list_workspace_branches: el modal de nueva tarea ofrece las ramas del workspace',
      fn: async (ctx) => {
        const ws = await registerWs(ctx, `te-branches-${tag}`);
        execSync(`git -C '${ws.dir}' branch rama-extra-${tag}`, { stdio: 'ignore' });
        await openBoard(ctx, ws.name);
        await ctx.page.getByRole('button', { name: 'Nueva tarea' }).first().click();
        await ctx.page.locator('form:has(select[name=workspace_id]):visible select[name=workspace_id]').selectOption({ label: `${ws.name} (main)` });
        const select = ctx.page.locator('form:has(select[name=workspace_id]):visible select[name=base_branch]');
        await select.waitFor({ timeout: 15000 });
        const started = Date.now();
        let listed = false;
        while (Date.now() - started < 15000 && !listed) {
          listed = await select.evaluate((el, b) => [...el.options].some((o) => o.textContent.includes(b)), `rama-extra-${tag}`);
          if (!listed) await sleep(400);
        }
        ctx.assert(listed, 'la rama creada no aparece entre las ramas base');
      },
    },
    {
      name: 'edit_lazy: la página abre al instante, muestra "Cargando" y luego carga ramas y agente',
      fn: async (ctx) => {
        const ws = await registerWs(ctx, `te-lazy-${tag}`);
        execSync(`git -C '${ws.dir}' branch rama-lazy-${tag}`, { stdio: 'ignore' });
        await openBoard(ctx, ws.name);
        const prompt = `lazy-${tag}`;
        await fillDialog(ctx, ws.name, { prompt });
        await submitDialog(ctx);
        await delayLazyLoads(ctx.page, 2500);
        await openEditFromCard(ctx, prompt);
        const page = ctx.page;
        await page.getByText('Cargando ramas').waitFor({ timeout: 5000 });
        await page.locator('form:visible input[name=show_advanced]').check();
        await page.getByText('Cargando opciones del agente').waitFor({ timeout: 5000 });
        await shot(ctx, '1-edit-loading-branches-and-agent');
        await page.getByText('Cargando ramas').waitFor({ state: 'detached', timeout: 15000 });
        await page.getByText('Cargando opciones del agente').waitFor({ state: 'detached', timeout: 15000 });
        const options = await page.locator('form:visible select[name=base_branch] option').allTextContents();
        ctx.assert(options.some((o) => o.includes(`rama-lazy-${tag}`)), `la rama no se cargó: ${options.join(', ')}`);
        ctx.assert((await page.locator('form:visible select[name=agent_runtime]').count()) === 1, 'el selector de agente no se cargó');
        await shot(ctx, '2-edit-loaded-branches-and-agent');
      },
    },
    {
      name: 'create_lazy: la pantalla abre al instante, muestra "Cargando" y luego carga workspaces, ramas y agente',
      fn: async (ctx) => {
        const ws = await registerWs(ctx, `te-clazy-${tag}`);
        execSync(`git -C '${ws.dir}' branch rama-clazy-${tag}`, { stdio: 'ignore' });
        await ctx.goto('/');
        await delayLazyLoads(ctx.page, 2500);
        await ctx.page.locator('#board-sidebar').getByRole('link', { name: ws.name }).first().click();
        await ctx.page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
        await ctx.page.getByRole('button', { name: 'Nueva tarea' }).first().click();
        const page = ctx.page;
        await page.getByRole('heading', { name: 'Nueva tarea' }).waitFor({ timeout: 5000 });
        // The page and its prompt are usable while the workspaces load.
        await page.getByText('Cargando workspaces').waitFor({ timeout: 5000 });
        await page.locator('form:visible textarea[name=prompt]').waitFor({ timeout: 5000 });
        await page.locator('form:visible input[name=show_advanced]').check();
        await page.getByText('Cargando opciones del agente').waitFor({ timeout: 5000 });
        await shot(ctx, '3-create-loading-workspaces-and-agent');
        await page.getByText('Cargando workspaces').waitFor({ state: 'detached', timeout: 15000 });
        await page.getByText('Cargando ramas').waitFor({ timeout: 5000 });
        await shot(ctx, '4-create-loading-branches');
        await page.getByText('Cargando ramas').waitFor({ state: 'detached', timeout: 15000 });
        await page.getByText('Cargando opciones del agente').waitFor({ state: 'detached', timeout: 15000 });
        const options = await page.locator('form:visible select[name=base_branch] option').allTextContents();
        ctx.assert(options.some((o) => o.includes(`rama-clazy-${tag}`)), `la rama no se cargó: ${options.join(', ')}`);
        ctx.assert((await page.locator('form:visible select[name=agent_runtime]').count()) === 1, 'el selector de agente no se cargó');
        await shot(ctx, '5-create-loaded');
      },
    },
    {
      name: 'create_lazy: crear mientras cargan los workspaces conserva el workspace de la dirección',
      fn: async (ctx) => {
        const ws = await registerWs(ctx, `te-clazysub-${tag}`);
        await ctx.goto('/');
        await delayLazyLoads(ctx.page, 4000);
        await ctx.page.locator('#board-sidebar').getByRole('link', { name: ws.name }).first().click();
        await ctx.page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
        await ctx.page.getByRole('button', { name: 'Nueva tarea' }).first().click();
        await ctx.page.getByText('Cargando workspaces').waitFor({ timeout: 5000 });
        const prompt = `clazysub-${tag}`;
        await ctx.page.locator('form:visible textarea[name=prompt]').fill(prompt);
        const create = ctx.page.locator('form:visible button[type=submit]', { hasText: 'Crear tarea' });
        ctx.assert(await create.isEnabled(), 'el botón de crear no debe quedar deshabilitado mientras carga');
        await create.click();
        await ctx.page.locator('[id^="board-list-"]:visible [data-stage="backlog"]', { hasText: prompt }).first().waitFor({ timeout: 15000 });
      },
    },
    {
      name: 'edit_lazy: guardar mientras carga conserva los datos de la tarea',
      fn: async (ctx) => {
        const ws = await registerWs(ctx, `te-lazysave-${tag}`);
        await openBoard(ctx, ws.name);
        const prompt = `lazysave-${tag}`;
        await fillDialog(ctx, ws.name, { prompt });
        await submitDialog(ctx);
        await delayLazyLoads(ctx.page, 4000);
        await openEditFromCard(ctx, prompt);
        await ctx.page.getByText('Cargando ramas').waitFor({ timeout: 5000 });
        await ctx.page.getByLabel(/^Título/).fill(`guardado-${tag}`);
        await ctx.page.getByRole('button', { name: 'Guardar cambios' }).click();
        await ctx.page.getByRole('heading', { name: 'Editar tarea' }).waitFor({ state: 'detached', timeout: 15000 });
        await ctx.page.locator('[id^="board-list-"]:visible').getByText(`guardado-${tag}`).first().waitFor({ timeout: 15000 });
      },
    },
    {
      name: 'delete_task: borrar desde la tarjeta de Backlog pide confirmación y la elimina',
      fn: async (ctx) => {
        const ws = await registerWs(ctx, `te-delete-${tag}`);
        await openBoard(ctx, ws.name);
        const prompt = `borrar-${tag}`;
        await fillDialog(ctx, ws.name, { prompt });
        await submitDialog(ctx);
        const card = ctx.page.locator('[id^="board-list-"]:visible [data-stage="backlog"]', { hasText: prompt }).first();
        await card.waitFor({ timeout: 10000 });
        // The harness accepts the confirmation of the card (see end2end/harness.cjs).
        await card.getByRole('button', { name: 'Borrar' }).click();
        await card.waitFor({ state: 'detached', timeout: 15000 });
      },
    },
    {
      name: 'start_task: "Iniciar" en la tarjeta de Backlog arranca la tarea o muestra por qué no',
      fn: async (ctx) => {
        const ws = await registerWs(ctx, `te-start-${tag}`);
        await openBoard(ctx, ws.name);
        const prompt = `iniciar-${tag}`;
        await fillDialog(ctx, ws.name, { prompt });
        await submitDialog(ctx);
        // The card is the element with data-task-id: the loose panel container also carries data-stage="backlog".
        const card = ctx.page.locator('[id^="board-list-"]:visible [data-task-id][data-stage="backlog"]', { hasText: prompt }).first();
        await card.waitFor({ timeout: 10000 });
        await card.getByRole('button', { name: 'Iniciar' }).click();
        // Either the task left backlog, or the failure is shown in the notice slot: never a silent click.
        const started = Date.now();
        let outcome = '';
        while (Date.now() - started < 20000 && !outcome) {
          if ((await ctx.page.locator('[id^="board-list-"]:visible [data-task-id][data-stage="backlog"]', { hasText: prompt }).count()) === 0) outcome = 'moved';
          else if ((await ctx.page.locator('[id^="board-notice-"]:visible .text-error').count()) > 0) outcome = 'failed';
          else await sleep(400);
        }
        ctx.assert(outcome !== '', 'Iniciar no produjo ni un cambio de estado ni un aviso');
      },
    },
  ],
};
