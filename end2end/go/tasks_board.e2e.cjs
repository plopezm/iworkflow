// end2end/go/tasks_board.e2e.cjs
// Go-mode copy of the Board part of end2end/tasks.e2e.cjs (the parity target for slice H, U1): the
// board tab, the create dialog, the workspace switcher and the live refresh from the task stream.
// The suite runs in Spanish (the language cookie of the harness), so the texts are the Spanish ones.
// Scope: the list of the board. The detail (U3), the graph and the loose panel (U7) are not covered
// here. Each test registers its own git repo as a workspace, so the tests do not see each other.
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';

function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `board-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `board ${prefix}\n`);
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

async function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

// Registers a workspace from the Workspaces screen and waits until it is listed.
async function registerWs(ctx, name) {
  const dir = uniqueGitRepo(name);
  await ctx.goto('/workspaces');
  await ctx.page.getByLabel('Nombre').first().fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await ctx.page.getByLabel(/Rama base/).first().fill('main');
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.getByText(dir).first().waitFor({ timeout: 12000 });
  return { name, dir };
}

// Opens the board from the navigation bar and selects the workspace in its switcher.
async function openBoard(ctx, name) {
  await ctx.goto('/');
  await ctx.page.getByRole('navigation', { name: 'Navegación' }).getByRole('link', { name: 'Tablero' }).click();
  await ctx.page.locator('#board-controls select[name=workspace]').selectOption({ label: `${name} (main)` });
  await ctx.page.locator('#board-list').waitFor({ timeout: 10000 });
  await sleep(300);
}

// Creates a task from the "Nueva tarea" dialog, leaving the start toggle off (backlog only).
async function createFromDialog(ctx, ws, { title, prompt }) {
  const page = ctx.page;
  await page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  await page.locator('#modal-panel select[name=workspace_id]').selectOption({ label: `${ws} (main)` });
  if (title) await page.locator('#modal-panel input[name=title]').fill(title);
  await page.locator('#modal-panel textarea[name=prompt]').fill(prompt);
  await page.locator('#modal-panel button[type=submit]', { hasText: 'Crear tarea' }).click();
  await page.locator('#modal-panel').waitFor({ state: 'detached', timeout: 10000 }).catch(() => {});
}

// ---- tests ----
module.exports = {
  name: 'tasks_board',
  tests: [
    {
      name: 'board_empty: un workspace sin tareas muestra el estado vacío y el alta de la primera',
      fn: async (ctx) => {
        const ws = await registerWs(ctx, `tb-empty-${tag}`);
        await openBoard(ctx, ws.name);
        await ctx.page.getByText('Aún no hay tareas en este workspace').first().waitFor({ timeout: 10000 });
        await ctx.page.getByRole('button', { name: 'Crear la primera tarea' }).first().waitFor({ timeout: 5000 });
      },
    },
    {
      name: 'create_task: crear en el tablero muestra la tarjeta en "To do" y enlaza a su detalle',
      fn: async (ctx) => {
        const prompt = `board-create-${tag}`;
        const ws = await registerWs(ctx, `tb-create-${tag}`);
        await openBoard(ctx, ws.name);
        await createFromDialog(ctx, ws.name, { prompt });
        const card = ctx.page.locator('#board-list [data-stage="backlog"] a', { hasText: prompt }).first();
        await card.waitFor({ timeout: 10000 });
        ctx.assert(/\/tasks\/[0-9a-f-]{36}$/.test(await card.getAttribute('href')), 'la tarjeta debe enlazar a /tasks/<id>');
      },
    },
    {
      name: 'list_tasks: el selector de workspace muestra solo las tareas de ese workspace',
      fn: async (ctx) => {
        const first = await registerWs(ctx, `tb-filter-a-${tag}`);
        const second = await registerWs(ctx, `tb-filter-b-${tag}`);
        await openBoard(ctx, first.name);
        const firstPrompt = `only-a-${tag}`;
        await createFromDialog(ctx, first.name, { prompt: firstPrompt });
        await ctx.page.locator('#board-list').getByText(firstPrompt).first().waitFor({ timeout: 10000 });
        await ctx.page.locator('#board-controls select[name=workspace]').selectOption({ label: `${second.name} (main)` });
        await ctx.page.locator('#board-list').waitFor({ timeout: 10000 });
        await sleep(500);
        ctx.assert((await ctx.page.locator('#board-list').getByText(firstPrompt).count()) === 0,
          'las tareas de otro workspace no deben mostrarse');
      },
    },
    {
      name: 'live_refresh: una tarea creada fuera del tablero aparece sin recargar (flujo SSE)',
      fn: async (ctx) => {
        const ws = await registerWs(ctx, `tb-live-${tag}`);
        await openBoard(ctx, ws.name);
        const prompt = `live-${tag}`;
        const workspaceId = await ctx.page.locator('#board-controls select[name=workspace]').inputValue();
        // A create that does not come from this page: the stream must carry the change to the board.
        await ctx.page.evaluate(async ({ workspaceId, prompt }) => {
          const body = new URLSearchParams({ workspace_id: workspaceId, prompt });
          await fetch('/board/tasks', { method: 'POST', body, headers: { 'Content-Type': 'application/x-www-form-urlencoded' } });
        }, { workspaceId, prompt });
        await ctx.page.locator('#board-list').getByText(prompt).first().waitFor({ timeout: 10000 });
      },
    },
  ],
};
