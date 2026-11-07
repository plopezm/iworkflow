// end2end/go/task_hooks_strip.e2e.cjs
// Go-mode e2e of the hook status strip of a task (T5b): the hooks of the workspace show in the Actions tab of a started
// task with their status and their folded log, and the per-task toggle skips and includes each one again. The workspace
// menu links to the Hooks tab of the workspace. The hook is created through the Hooks tab of the workspace (T5a), so
// this suite depends on it. A backlog task has no Actions tab, so the task is started: its hook is a `true` command
// that exits 0 in the worktree, so the strip shows it as run. The suite runs in Spanish. Each test registers its own
// git repo as a workspace, so the tests do not see each other.

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';

function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `hooks-strip-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `hooks ${prefix}\n`);
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
  return name;
}

// Opens the Hooks tab of a workspace from its menu in the sidebar, and returns its address.
async function openHooksTab(ctx) {
  await ctx.goto('/');
  const link = ctx.page.locator('#board-sidebar a[href$="/hooks"]').first();
  await link.waitFor({ state: 'attached', timeout: 10000 });
  const href = await link.getAttribute('href');
  await ctx.goto(href);
  await ctx.page.getByRole('button', { name: 'Añadir hook +' }).first().waitFor({ timeout: 10000 });
  return href;
}

// Creates a script hook that runs before the start of a task, through the Hooks tab. The first "Añadir hook +" belongs
// to the group before a task starts, so the hook gets that phase and event.
async function addBeforeStartHook(ctx, command) {
  const page = ctx.page;
  await page.getByRole('button', { name: 'Añadir hook +' }).first().click();
  await page.getByLabel('Comando').fill(command);
  await page.getByRole('button', { name: 'Guardar' }).click();
  await page.getByText(command).first().waitFor({ timeout: 10000 });
}

// Creates a task in the workspace, starts it, and opens its detail on the Actions tab.
async function createStartedTask(ctx, wsName, prompt) {
  await ctx.goto('/');
  await ctx.page.locator('#board-sidebar').getByRole('link', { name: wsName }).first().click();
  await ctx.page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
  await sleep(300);
  const page = ctx.page;
  await page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  await page.getByLabel(/^Workspace/).selectOption({ label: `${wsName} (main)` });
  await page.getByLabel('Prompt').fill(prompt);
  await page.getByLabel('Iniciar ahora').check();
  await page.getByRole('button', { name: 'Crear tarea' }).click();
  // Navigating before the create -> start chain ends cancels the start: wait for the modal to close first.
  await page.getByRole('button', { name: 'Crear tarea' }).waitFor({ state: 'detached', timeout: 15000 });
  const link = page.locator('a[href*="/tasks/"]', { hasText: prompt }).first();
  await link.waitFor({ state: 'attached', timeout: 15000 });
  await sleep(1500);
  await ctx.goto(await link.getAttribute('href'));
  await page.waitForURL(/\/tasks\/[0-9a-f-]{36}/, { timeout: 15000, waitUntil: 'commit' });
  await page.getByRole('tab', { name: 'Acciones' }).first().click();
  await page.locator('[data-task-hooks]').waitFor({ timeout: 10000 });
}

// The strip is rendered with the page, so a run that ends after the page opened shows on the next render: reload until
// the status says so, for a few seconds at most.
async function waitForStatus(ctx, status) {
  const strip = ctx.page.locator('[data-task-hooks]');
  for (let attempt = 0; attempt < 15; attempt++) {
    if (await strip.getByText(status, { exact: true }).count() > 0) return;
    await sleep(1000);
    await ctx.page.reload();
    await ctx.page.getByRole('tab', { name: 'Acciones' }).first().click();
    await ctx.page.locator('[data-task-hooks]').waitFor({ timeout: 10000 });
  }
  ctx.assert(false, `el estado "${status}" no apareció en la franja de hooks`);
}

module.exports = {
  name: 'task_hooks_strip',
  tests: [
    {
      name: 'strip: el hook corrido muestra su estado y registro, y el toggle lo omite e incluye',
      fn: async (ctx) => {
        const wsName = await registerWs(ctx, `hs-toggle-${tag}`);
        await openHooksTab(ctx);
        await addBeforeStartHook(ctx, 'true');
        await createStartedTask(ctx, wsName, `hookstrip-${tag}`);
        const strip = ctx.page.locator('[data-task-hooks]');

        await strip.getByText('Antes de').first().waitFor({ timeout: 10000 });
        await waitForStatus(ctx, 'Correcto');
        ctx.assert(await strip.getByText('Registro', { exact: true }).count() > 0, 'el hook corrido debe tener su registro');

        await strip.getByRole('button', { name: 'Omitir' }).click();
        await strip.getByRole('button', { name: 'Incluir' }).waitFor({ timeout: 10000 });
        ctx.assert(await strip.getByText('Omitido', { exact: true }).count() > 0, 'tras omitirlo el estado debe ser omitido');

        await strip.getByRole('button', { name: 'Incluir' }).click();
        await strip.getByRole('button', { name: 'Omitir' }).waitFor({ timeout: 10000 });
        ctx.assert(await strip.getByText('Correcto', { exact: true }).count() > 0, 'tras incluirlo vuelve a su último estado');
      },
    },
    {
      name: 'menú del workspace: el enlace Hooks abre la pestaña de hooks del workspace',
      fn: async (ctx) => {
        await registerWs(ctx, `hs-menu-${tag}`);
        const href = await openHooksTab(ctx);
        ctx.assert(/\/workspaces\/[^/]+\/hooks$/.test(href), `el enlace Hooks apunta a ${href}`);
        ctx.assert(await ctx.page.getByRole('button', { name: 'Añadir hook +' }).first().count() > 0, 'la pestaña de hooks no abrió');
      },
    },
  ],
};
